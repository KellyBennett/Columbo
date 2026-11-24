package feedback

import (
	"cmp"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func scopeModule(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"go.mod":                 "module example.invalid/scope\n\ngo 1.25.1\n",
		"leaf/leaf.go":           "package leaf\n\nfunc Value() int { return 1 }\n",
		"importer/importer.go":   "package importer\n\nimport \"example.invalid/scope/leaf\"\n\nfunc Value() int { return leaf.Value() }\n",
		"unrelated/unrelated.go": "package unrelated\n\nfunc Value() int { return 2 }\n",
	}
	for name, content := range files {
		require.NoError(t, os.MkdirAll(filepath.Join(dir, filepath.Dir(name)), 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600))
	}
	return dir
}

func TestAffectedScopeCoversChangedPackagesAndTheirImporters(t *testing.T) {
	dir := scopeModule(t)
	options := PilotOptions{Module: ".", CheckTimeout: time.Minute}
	scope := affectedScope(context.Background(), options, dir, []string{"leaf/leaf.go"})
	require.Equal(t, TestScope{Packages: []string{"example.invalid/scope/importer", "example.invalid/scope/leaf"}, Reason: "affected packages"}, scope)
	require.Equal(t, []string{"go build ./...", "go vet ./...", "go test 'example.invalid/scope/importer' 'example.invalid/scope/leaf'"}, options.checkCommands(scope))
}

func TestAffectedScopeFallsBackToTheWholeModule(t *testing.T) {
	dir := scopeModule(t)
	for _, test := range []struct {
		name    string
		options PilotOptions
		changed []string
		reason  string
	}{
		{"module requirements", PilotOptions{}, []string{"leaf/leaf.go", "go.mod"}, "go.mod changes module requirements"},
		{"non-Go input", PilotOptions{}, []string{"leaf/data.txt"}, "leaf/data.txt is not a Go source file"},
		{"outside the module", PilotOptions{Module: "leaf"}, []string{"unrelated/unrelated.go"}, "unrelated/unrelated.go is outside the module"},
		{"full tests", PilotOptions{FullTests: true}, []string{"leaf/leaf.go"}, "--full-tests"},
	} {
		t.Run(test.name, func(t *testing.T) {
			options := test.options
			options.Module, options.CheckTimeout = cmp.Or(options.Module, "."), time.Minute
			scope := affectedScope(context.Background(), options, dir, test.changed)
			require.Equal(t, TestScope{Reason: test.reason}, scope)
			require.Contains(t, options.checkCommands(scope), "go test ./...")
		})
	}
}
