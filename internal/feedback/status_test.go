package feedback

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"github.com/KellyBennett/Columbo/internal/columbo"
	"github.com/stretchr/testify/require"
)

func TestClassifyMiss(t *testing.T) {
	known := columbo.SmellIDs()
	tests := []struct {
		body   string
		rule   string
		isMiss bool
	}{
		{"Columbo missed: long-function in Parse", "long-function", true},
		{"  columbo MISSED: this is a Feature-Envy case", "feature-envy", true},
		{"Columbo missed: data-clump here, and long-function too", "data-clump", true},
		{"Columbo missed: long-functions everywhere", "", true},
		{"Columbo missed: (duplicate-code)", "duplicate-code", true},
		{"Columbo missed: something subtle", "", true},
		{"Nice catch on long-function", "", false},
		{"> Columbo missed: long-function", "", false},
	}
	for _, test := range tests {
		rule, isMiss := classifyMiss(test.body, known)
		require.Equal(t, test.isMiss, isMiss, test.body)
		require.Equal(t, test.rule, rule, test.body)
	}
}

func TestStatusCountsReactionsAndMissesPerRule(t *testing.T) {
	fake, server := newFakeGitHub(t)
	fake.set("/repos/o/r/issues/1/comments", `[
		{"id": 21, "body": " columbo MISSED: feature-envy in G", "created_at": "2026-01-02T02:00:00Z", "updated_at": "2026-01-02T02:00:00Z"},
		{"id": 22, "body": "Columbo missed: a subtle one", "created_at": "2026-01-02T02:00:00Z", "updated_at": "2026-01-02T02:00:00Z"},
		{"id": 23, "body": "Columbo missed: long-parameter-list in H", "created_at": "2026-01-02T02:00:00Z", "updated_at": "2026-01-02T02:00:00Z"}]`)
	store := filepath.Join(t.TempDir(), "feedback.sqlite")
	collectAt(t, server, store, "2026-02-01T00:00:00Z")

	var out bytes.Buffer
	require.NoError(t, Status(context.Background(), store, &out))
	require.Equal(t, ""+
		"rule                 👍  👎  missed\n"+
		"feature-envy          0   0       1\n"+
		"long-parameter-list   1   1       1\n"+
		"no rule               0   0       1\n", out.String())
}

func TestStatusWithoutAStoreExitsTwo(t *testing.T) {
	var stderr bytes.Buffer
	missing := filepath.Join(t.TempDir(), "missing.sqlite")
	code := Main([]string{"status", "--store", missing}, &bytes.Buffer{}, &stderr, DefaultEnvironment())
	require.Equal(t, 2, code)
	require.Contains(t, stderr.String(), "no feedback store at "+missing)
	require.NoFileExists(t, missing)
}
