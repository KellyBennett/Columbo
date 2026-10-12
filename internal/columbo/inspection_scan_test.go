package columbo

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInspectionRejectsSourceDriftBeforePublication(t *testing.T) {
	writeFile := func(name, contents string) func(string) error {
		return func(root string) error {
			return os.WriteFile(filepath.Join(root, name), []byte(contents), 0600)
		}
	}
	for _, tc := range []struct {
		name   string
		change func(string) error
	}{
		{name: "selected", change: writeFile("model.go", inspectionFixture+"\n")},
		{name: "unselected", change: writeFile("platform_windows.go", "package fixture\n")},
		{name: "non-Go", change: writeFile("README.md", "after")},
		{name: "added", change: writeFile("new.txt", "new")},
		{name: "removed", change: func(root string) error { return os.Remove(filepath.Join(root, "README.md")) }},
		{name: "configuration", change: writeFile(".columbo.yml", "exclude: []\n")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := inspectionTestModule(t)
			require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("before"), 0600))
			input, err := validateInspectionInput(root, "")
			require.NoError(t, err)
			require.NoError(t, input.capture())
			a, err := load(root, []string{"./..."}, input.config)
			require.NoError(t, err)
			require.NoError(t, tc.change(root))
			report, err := input.finish(a, []string{"./..."})
			assert.ErrorContains(t, err, "source changed during inspection")
			assert.Nil(t, report.inspection)
		})
	}
}

func TestInspectionRejectsConfigurationDriftBeforeLoading(t *testing.T) {
	writeChanged := func(path string) error {
		return os.WriteFile(path, []byte("thresholds:\n  function-lines: 11\n"), 0600)
	}
	for _, tc := range []struct {
		name   string
		exists bool
		change func(string) error
	}{
		{name: "changed", exists: true, change: writeChanged},
		{name: "created", change: writeChanged},
		{name: "removed", exists: true, change: os.Remove},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := inspectionTestModule(t)
			path := filepath.Join(root, ".columbo.yml")
			if tc.exists {
				require.NoError(t, os.WriteFile(path, []byte("exclude: []\n"), 0600))
			}
			input, err := validateInspectionInput(root, "")
			require.NoError(t, err)
			require.NoError(t, tc.change(path))
			require.ErrorContains(t, input.capture(), "configuration changed during inspection")
		})
	}
}

func TestInspectionOutputBoundaries(t *testing.T) {
	root := inspectionTestModule(t)
	input, err := validateInspectionInput(root, "")
	require.NoError(t, err)
	require.ErrorContains(t, input.validateOutput(filepath.Join(root, "report.sqlite")), "outside source root")
	out := filepath.Join(t.TempDir(), "report.sqlite")
	require.NoError(t, input.validateOutput(out))
	require.NoError(t, os.WriteFile(out, []byte("preserved"), 0600))
	require.ErrorContains(t, input.validateOutput(out), "already exists")
	data, err := os.ReadFile(out)
	require.NoError(t, err)
	require.Equal(t, "preserved", string(data))
	link := filepath.Join(t.TempDir(), "source-link")
	require.NoError(t, os.Symlink(root, link))
	require.ErrorContains(t, input.validateOutput(filepath.Join(link, "report.sqlite")), "outside source root")
}

func TestInspectionCLIRejectsUnsafeOutputWithoutSummary(t *testing.T) {
	root := inspectionTestModule(t)
	var stdout, stderr bytes.Buffer
	code := Run([]string{"--no-history", "--output", filepath.Join(root, "report.sqlite")}, Invocation{Dir: root, Stdout: &stdout, Stderr: &stderr})
	require.Equal(t, 2, code)
	require.Empty(t, stdout.String())
	require.Contains(t, stderr.String(), "outside source root")
	files, err := filepath.Glob(filepath.Join(root, "*.sqlite"))
	require.NoError(t, err)
	require.Empty(t, files)
}

func TestInspectionSupportsModuleSubdirectories(t *testing.T) {
	root := inspectionTestModule(t)
	subdirectory := filepath.Join(root, "sub")
	require.NoError(t, os.Mkdir(subdirectory, 0700))
	require.NoError(t, os.WriteFile(filepath.Join(subdirectory, "sub.go"), []byte("package sub\n"), 0600))
	input, err := validateInspectionInput(subdirectory, "")
	require.NoError(t, err)
	report, err := input.analyze(nil)
	require.NoError(t, err)
	require.NotNil(t, report.inspection)
	assert.Equal(t, root, report.inspection.Root)
	byPath := map[string]inspectionFile{}
	for _, file := range report.inspection.Files {
		byPath[file.Path] = file
	}
	require.Contains(t, byPath, "model.go", "inventory covers the enclosing module")
	assert.False(t, byPath["model.go"].Selected, "runtime selection stays in the invoked subdirectory")
	require.Contains(t, byPath, "sub/sub.go")
	assert.True(t, byPath["sub/sub.go"].Selected)
	assert.True(t, byPath["sub/sub.go"].Target)
}
