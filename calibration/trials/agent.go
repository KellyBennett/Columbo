package trials

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Agent is the Strategy a trial hands its workspace and prompt to; tests swap in a deterministic one.
type Agent interface {
	// Describe names the agent command and model as recorded with every trial.
	Describe() (command, model string)
	Fix(ctx context.Context, dir, prompt string) AgentRun
}

type AgentRun struct {
	Exit     int
	TimedOut bool
	Wall     time.Duration
	Turns    int
	CostUSD  float64
	ModelID  string
	Reply    string
	// Stop is the agent's own reason for ending, such as success or error_max_turns.
	Stop string
	// InfraError means the agent never got to work, so the trial is retried rather than scored.
	InfraError string
	Transcript []byte
}

// DefaultCommand runs Claude Code headless without the user's settings, CLAUDE.md, plugins, or MCP servers.
// The allowed tools limit which commands the agent may start, not what they do: go test runs test code the agent can write.
const DefaultCommand = `claude -p --safe-mode --restricted --no-session-persistence --model {model} --max-turns {turns} ` +
	`--output-format stream-json --verbose --permission-mode dontAsk --tools Read,Edit,Write,Glob,Grep,Bash ` +
	`--allowedTools "Read,Edit,Write,Glob,Grep,Bash(go build:*),Bash(go vet:*),Bash(go test:*),Bash(columbo:*)"`

type CommandAgent struct {
	// Command is shell words; {model} and {turns} are substituted. The prompt arrives on stdin.
	Command  string
	Model    string
	MaxTurns int
	Timeout  time.Duration
	// Env sets variables on top of the few inherited ones.
	Env []string
}

// inheritedEnv names the variables an agent gets from the harness's own environment: what the go command needs to find
// its toolchain and caches, and USER and HOME, which the claude CLI needs to find its login. Tokens, sockets, and every
// other variable stay behind, because the agent's tests run with whatever the agent inherits.
var inheritedEnv = []string{"PATH", "HOME", "USER", "TMPDIR", "GOPATH", "GOCACHE", "GOMODCACHE", "GOFLAGS", "GOTOOLCHAIN"}

func agentEnv(extra []string) []string {
	var env []string
	for _, name := range inheritedEnv {
		if v, ok := os.LookupEnv(name); ok {
			env = append(env, name+"="+v)
		}
	}
	return append(env, extra...)
}

func (a CommandAgent) Describe() (string, string) { return a.Command, a.Model }

func (a CommandAgent) Fix(ctx context.Context, dir, prompt string) AgentRun {
	words, err := splitWords(a.Command)
	if err != nil || len(words) == 0 {
		return AgentRun{InfraError: fmt.Sprintf("agent command %q: %v", a.Command, err)}
	}
	for i, w := range words {
		words[i] = strings.NewReplacer("{model}", a.Model, "{turns}", strconv.Itoa(a.MaxTurns)).Replace(w)
	}
	ctx, cancel := context.WithTimeout(ctx, a.Timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, words[0], words[1:]...)
	cmd.Dir = dir
	cmd.Env = agentEnv(a.Env)
	cmd.Stdin = strings.NewReader(prompt)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = 10 * time.Second
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	start := time.Now()
	err = cmd.Run()
	run := AgentRun{Wall: time.Since(start), Transcript: stdout.Bytes(), TimedOut: errors.Is(ctx.Err(), context.DeadlineExceeded)}
	var exit *exec.ExitError
	switch {
	case errors.As(err, &exit):
		run.Exit = exit.ExitCode()
	case err != nil && !run.TimedOut:
		run.InfraError = err.Error()
		return run
	}
	readResult(&run, stdout.Bytes())
	switch {
	case run.Stop == "" && run.TimedOut:
		run.Stop = "timeout"
	case run.Stop == "" && run.InfraError == "":
		run.InfraError = "no result from agent: " + firstLine(stderr.String())
	}
	return run
}

// claudeResult is the final stream-json record Claude Code prints.
type claudeResult struct {
	Type      string                     `json:"type"`
	Subtype   string                     `json:"subtype"`
	IsError   bool                       `json:"is_error"`
	NumTurns  int                        `json:"num_turns"`
	CostUSD   float64                    `json:"total_cost_usd"`
	Result    string                     `json:"result"`
	APIError  any                        `json:"api_error_status"`
	ModelCost map[string]json.RawMessage `json:"modelUsage"`
}

func readResult(run *AgentRun, transcript []byte) {
	sc := bufio.NewScanner(bytes.NewReader(transcript))
	sc.Buffer(make([]byte, 1<<20), 64<<20)
	for sc.Scan() {
		var r claudeResult
		if json.Unmarshal(sc.Bytes(), &r) != nil || r.Type != "result" {
			continue
		}
		run.Stop, run.Turns, run.CostUSD, run.Reply = r.Subtype, r.NumTurns, r.CostUSD, r.Result
		var models []string
		for m := range r.ModelCost {
			models = append(models, m)
		}
		sort.Strings(models)
		run.ModelID = strings.Join(models, ",")
		switch {
		case r.APIError != nil:
			run.InfraError = fmt.Sprintf("api error %v: %s", r.APIError, firstLine(r.Result))
		case r.IsError && r.Subtype != "error_max_turns":
			run.InfraError = "agent ended on an error: " + firstLine(r.Result)
		}
	}
}

// splitWords splits like a POSIX shell for plain words, single quotes, and double quotes; nothing else is special.
func splitWords(s string) ([]string, error) {
	var words []string
	var cur strings.Builder
	inWord := false
	var quote rune
	for _, r := range s {
		switch {
		case quote != 0 && r == quote:
			quote = 0
		case quote != 0:
			cur.WriteRune(r)
		case r == '\'' || r == '"':
			quote, inWord = r, true
		case r == ' ' || r == '\t' || r == '\n':
			if inWord {
				words = append(words, cur.String())
				cur.Reset()
				inWord = false
			}
		default:
			cur.WriteRune(r)
			inWord = true
		}
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated %c quote", quote)
	}
	if inWord {
		words = append(words, cur.String())
	}
	return words, nil
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
