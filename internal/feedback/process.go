package feedback

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

func git(ctx context.Context, dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", append([]string{"-c", "core.hooksPath=/dev/null"}, args...)...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s failed: %s", args[0], strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

type CommandResult struct {
	Args          []string `json:"args"`
	Directory     string   `json:"directory"`
	Exit          int      `json:"exit"`
	TimedOut      bool     `json:"timed_out"`
	Seconds       float64  `json:"seconds"`
	Stdout        string   `json:"stdout"`
	Stderr        string   `json:"stderr"`
	Error         string   `json:"error,omitempty"`
	EvidenceValid bool     `json:"evidence_valid,omitempty"`
}

func trialEnvironment(extra ...string) []string {
	var env []string
	names := []string{"PATH", "HOME", "USER", "TMPDIR", "GOPATH", "GOCACHE", "GOMODCACHE", "GOFLAGS", "GOTOOLCHAIN", "GOPROXY", "GOPRIVATE", "GONOSUMDB", "GONOPROXY", "CGO_ENABLED", "GOOS", "GOARCH", "ANTHROPIC_BASE_URL", "ANTHROPIC_AUTH_TOKEN", "ANTHROPIC_API_KEY", "CLAUDE_CODE_OAUTH_TOKEN", "AWS_PROFILE", "AWS_REGION", "AWS_DEFAULT_REGION", "CLAUDE_CODE_USE_BEDROCK", "CLAUDE_CODE_USE_VERTEX", "CLOUD_ML_REGION", "ANTHROPIC_VERTEX_PROJECT_ID"}
	for _, name := range append(names, extra...) {
		if value, ok := os.LookupEnv(name); ok {
			env = append(env, name+"="+value)
		}
	}
	return append(env, "GIT_TERMINAL_PROMPT=0")
}

func runTrialCommand(ctx context.Context, dir string, args []string, prompt, log string, timeout time.Duration, passEnv ...string) CommandResult {
	result := CommandResult{Args: args, Directory: dir, Exit: -1, Stdout: log + ".stdout", Stderr: log + ".stderr"}
	if len(args) == 0 {
		result.Error = "empty command"
		return result
	}
	if err := os.MkdirAll(filepath.Dir(log), 0o700); err != nil {
		result.Error = err.Error()
		return result
	}
	stdout, err := os.OpenFile(result.Stdout, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	defer stdout.Close()
	stderr, err := os.OpenFile(result.Stderr, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	defer stderr.Close()
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir, cmd.Env, cmd.Stdin = dir, trialEnvironment(passEnv...), strings.NewReader(prompt)
	cmd.Stdout, cmd.Stderr = stdout, stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = 5 * time.Second
	start := time.Now()
	err = cmd.Run()
	result.Seconds, result.TimedOut = time.Since(start).Seconds(), errors.Is(ctx.Err(), context.DeadlineExceeded)
	if cmd.ProcessState != nil {
		result.Exit = cmd.ProcessState.ExitCode()
	}
	if err != nil {
		result.Error = err.Error()
	}
	return result
}

func checkoutRevision(ctx context.Context, archive, sha, destination string) error {
	if !commitPattern.MatchString(sha) {
		return fmt.Errorf("invalid revision %q", sha)
	}
	if _, err := git(ctx, "", "init", "--quiet", destination); err != nil {
		return err
	}
	if _, err := git(ctx, destination, "fetch", "--quiet", "--no-tags", archive, sha); err != nil {
		return err
	}
	_, err := git(ctx, destination, "checkout", "--quiet", "--detach", "FETCH_HEAD")
	return err
}
