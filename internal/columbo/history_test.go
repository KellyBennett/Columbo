package columbo

import (
	"github.com/stretchr/testify/require"
	"os"
	"os/exec"
	"strings"
)

// gitFixture owns a disposable repository and deterministic commit metadata.
type gitFixture struct {
	dir string
	env []string
}

func fixtureGitEnv() []string {
	return append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@example.test", "GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@example.test", "GIT_AUTHOR_DATE=2000-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2000-01-01T00:00:00Z")
}
func (t *testHarness) committedFixture(source string) gitFixture {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git unavailable")
	}
	fixture := gitFixture{t.fixture(source), fixtureGitEnv()}
	fixture.run(t, "init")
	fixture.run(t, "add", ".")
	fixture.run(t, "commit", "-m", "fixture")
	return fixture
}
func (f gitFixture) run(t *testHarness, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = f.dir
	command.Env = f.env
	output, err := command.CombinedOutput()
	t.requiref(err == nil, "git %v: %v\n%s", args, err, output)
	return strings.TrimSpace(string(output))
}
func historyReceipts(c Case) []History {
	histories := []History{}
	for _, receipt := range c.Receipts {
		if history, ok := receipt.(History); ok {
			histories = append(histories, history)
		}
	}
	return histories
}
func (t *testHarness) requireHistory(c Case, commit string) {
	t.Helper()
	histories := historyReceipts(c)
	require.NotEmpty(t.T, histories, "missing provenance")
	for _, history := range histories {
		require.Equal(t.T, commit, history.Commit)
		require.Equal(t.T, int64(946684800), history.CommittedAt)
		require.Equal(t.T, []string{"source.go"}, history.Files)
	}
}
