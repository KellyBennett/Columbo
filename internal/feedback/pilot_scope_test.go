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
		"go.mod":                   "module example.invalid/scope\n\ngo 1.25.1\n",
		"leaf/leaf.go":             "package leaf\n\nfunc Value() int { return 1 }\n",
		"importer/importer.go":     "package importer\n\nimport \"example.invalid/scope/leaf\"\n\nfunc Value() int { return leaf.Value() }\n",
		"unrelated/unrelated.go":   "package unrelated\n\nfunc Value() int { return 2 }\n",
		"README.md":                "# scope\n",
		"leaf/testdata/golden.txt": "1\n",
		"web/web.go":               "package web\n\nimport _ \"embed\"\n\n//go:embed static/index.html\nvar Index string\n",
		"web/static/static.go":     "package static\n",
		"web/static/index.html":    "<p>hi</p>\n",
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

func TestAffectedScopeAttributesNonGoFilesToTheirPackages(t *testing.T) {
	dir := scopeModule(t)
	options := PilotOptions{Module: ".", CheckTimeout: time.Minute}
	for _, test := range []struct {
		name     string
		changed  []string
		packages []string
	}{
		{"documentation is ignored", []string{"README.md", "docs/guide.md", ".github/workflows/ci.yml", "leaf/leaf.go"}, []string{"example.invalid/scope/importer", "example.invalid/scope/leaf"}},
		{"test data selects its package", []string{"leaf/testdata/golden.txt"}, []string{"example.invalid/scope/importer", "example.invalid/scope/leaf"}},
		{"embedded file selects its embedder", []string{"web/static/index.html"}, []string{"example.invalid/scope/web", "example.invalid/scope/web/static"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, TestScope{Packages: test.packages, Reason: "affected packages"}, affectedScope(context.Background(), options, dir, test.changed))
		})
	}
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
		{"documentation only", PilotOptions{}, []string{"README.md"}, "no package encloses the changed files"},
		{"Go source outside the module", PilotOptions{Module: "leaf"}, []string{"unrelated/unrelated.go"}, "unrelated/unrelated.go is Go source outside the module"},
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
