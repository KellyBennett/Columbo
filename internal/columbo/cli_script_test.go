package columbo

import (
	"github.com/rogpeppe/go-internal/testscript"
	"strconv"
	"testing"
)

func TestCLI(t *testing.T) {
	testscript.Run(t, testscript.Params{Dir: "testdata/cli", Cmds: map[string]func(*testscript.TestScript, bool, []string){"columbo": scriptColumbo}})
}

// The first argument is the exact expected exit code, keeping distinctions
// between findings (1) and failed investigations or invalid flags (2).
func scriptColumbo(script *testscript.TestScript, neg bool, args []string) {
	if neg || len(args) == 0 {
		script.Fatalf("usage: columbo EXPECTED_EXIT [flags]")
	}
	expected, err := strconv.Atoi(args[0])
	script.Check(err)
	actual := Run(args[1:], Invocation{Dir: script.MkAbs("."), Version: "test", Stdout: script.Stdout(), Stderr: script.Stderr()})
	if actual != expected {
		script.Fatalf("exit code: got %d, want %d", actual, expected)
	}
}
