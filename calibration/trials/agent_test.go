package trials

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestDefaultCommandKeepsEachAllowedToolsListOneArgument(t *testing.T) {
	words, err := splitWords(DefaultCommand)
	if err != nil {
		t.Fatal(err)
	}
	i := slices.Index(words, "--allowedTools")
	if i < 0 || i+1 >= len(words) {
		t.Fatalf("no --allowedTools value in %q", words)
	}
	shell := map[string]bool{"Bash(go build:*)": true, "Bash(go vet:*)": true, "Bash(go test:*)": true, "Bash(columbo:*)": true}
	for _, tool := range strings.Split(words[i+1], ",") {
		if strings.HasPrefix(tool, "Bash") && !shell[tool] {
			t.Errorf("allowed tool %q grants shell beyond go build, vet, test, and columbo", tool)
		}
	}
	for _, unsafe := range []string{"--dangerously-skip-permissions", "bypassPermissions", "--allow-dangerously-skip-permissions"} {
		if slices.Contains(words, unsafe) {
			t.Errorf("default agent command passes %s", unsafe)
		}
	}
}

func TestSplitWordsRejectsAnUnterminatedQuote(t *testing.T) {
	if _, err := splitWords(`claude -p "oops`); err == nil {
		t.Fatal("unterminated quote accepted")
	}
}

func script(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "agent.sh")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCommandAgentReadsClaudeResultFromStreamJSON(t *testing.T) {
	bin := script(t, `cat > "$PWD/prompt.txt"
echo '{"type":"system","subtype":"init"}'
echo '{"type":"result","subtype":"success","is_error":false,"num_turns":7,"total_cost_usd":0.42,"result":"done '"$1"'","api_error_status":null,"modelUsage":{"claude-sonnet-5-5":{}}}'
`)
	dir := t.TempDir()
	run := CommandAgent{Command: bin + " {model}", Model: "sonnet", MaxTurns: 3, Timeout: time.Minute}.Fix(context.Background(), dir, "fix it")
	if run.InfraError != "" || run.Stop != "success" || run.Turns != 7 || run.CostUSD != 0.42 || run.ModelID != "claude-sonnet-5-5" || run.Reply != "done sonnet" {
		t.Fatalf("run %+v", run)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "prompt.txt")); string(got) != "fix it" {
		t.Fatalf("agent read prompt %q from stdin in its workspace", got)
	}
}

func TestCommandAgentStopsAtTheTimeout(t *testing.T) {
	bin := script(t, "sleep 30\n")
	run := CommandAgent{Command: bin, Timeout: 200 * time.Millisecond}.Fix(context.Background(), t.TempDir(), "")
	if !run.TimedOut || run.Stop != "timeout" || run.InfraError != "" {
		t.Fatalf("run %+v, want a scored timeout", run)
	}
	if run.Wall > 15*time.Second {
		t.Fatalf("timeout took %s to stop the agent", run.Wall)
	}
}

func TestAnAgentThatNeverReportsIsAnInfrastructureError(t *testing.T) {
	bin := script(t, "echo 'rate limited' >&2\nexit 1\n")
	run := CommandAgent{Command: bin, Timeout: time.Minute}.Fix(context.Background(), t.TempDir(), "")
	if !strings.Contains(run.InfraError, "rate limited") {
		t.Fatalf("infra error %q, want the agent's stderr", run.InfraError)
	}
}

func TestAnErrorResultIsAnInfrastructureErrorButTheTurnCapIsNot(t *testing.T) {
	for subtype, infra := range map[string]bool{"success": true, "error_during_execution": true, "error_max_turns": false} {
		bin := script(t, `echo '{"type":"result","subtype":"`+subtype+`","is_error":true,"num_turns":1,"result":"Not logged in","api_error_status":null}'
`)
		run := CommandAgent{Command: bin, Timeout: time.Minute}.Fix(context.Background(), t.TempDir(), "")
		if (run.InfraError != "") != infra {
			t.Errorf("is_error with subtype %s: infra error %q, want infra %v", subtype, run.InfraError, infra)
		}
	}
}

func TestTheAgentInheritsOnlyTheAllowedEnvironment(t *testing.T) {
	t.Setenv("CALIBRATION_PROBE_TOKEN", "secret")
	t.Setenv("USER", "someone")
	bin := script(t, "env > \"$PWD/env.txt\"\necho '{\"type\":\"result\",\"subtype\":\"success\"}'\n")
	dir := t.TempDir()
	CommandAgent{Command: bin, Timeout: time.Minute, Env: []string{"GOWORK=off"}}.Fix(context.Background(), dir, "")
	got, err := os.ReadFile(filepath.Join(dir, "env.txt"))
	if err != nil {
		t.Fatal(err)
	}
	env := "\n" + string(got)
	for _, want := range []string{"\nUSER=someone\n", "\nGOWORK=off\n", "\nPATH="} {
		if !strings.Contains(env, want) {
			t.Errorf("agent environment lacks %q", strings.TrimSpace(want))
		}
	}
	if strings.Contains(env, "CALIBRATION_PROBE_TOKEN") {
		t.Error("agent inherited a variable outside the allowlist")
	}
}
