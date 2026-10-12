package columbo

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInspectionRejectsSourceDriftBeforePublication(t *testing.T) {
	for _, change := range []string{"selected", "unselected", "non-Go", "added", "removed", "configuration"} {
		t.Run(change, func(t *testing.T) {
			root := inspectionTestModule(t)
			require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("before"), 0600))
			input, err := validateInspectionInput(root, "")
			require.NoError(t, err)
			require.NoError(t, input.capture())
			a, err := load(root, []string{"./..."}, input.config)
			require.NoError(t, err)
			switch change {
			case "selected":
				require.NoError(t, os.WriteFile(filepath.Join(root, "model.go"), []byte(inspectionFixture+"\n"), 0600))
			case "unselected":
				require.NoError(t, os.WriteFile(filepath.Join(root, "platform_windows.go"), []byte("package fixture\n"), 0600))
			case "non-Go":
				require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("after"), 0600))
			case "added":
				require.NoError(t, os.WriteFile(filepath.Join(root, "new.txt"), []byte("new"), 0600))
			case "removed":
				require.NoError(t, os.Remove(filepath.Join(root, "README.md")))
			case "configuration":
				require.NoError(t, os.WriteFile(filepath.Join(root, ".columbo.yml"), []byte("exclude: []\n"), 0600))
			}
			report, err := input.finish(a, []string{"./..."})
			require.ErrorContains(t, err, "source changed during inspection")
			require.Nil(t, report.inspection)
		})
	}
}

func TestInspectionRejectsConfigurationDriftBeforeLoading(t *testing.T) {
	for _, mode := range []string{"changed", "created", "removed"} {
		t.Run(mode, func(t *testing.T) {
			root := inspectionTestModule(t)
			path := filepath.Join(root, ".columbo.yml")
			if mode != "created" {
				require.NoError(t, os.WriteFile(path, []byte("exclude: []\n"), 0600))
			}
			input, err := validateInspectionInput(root, "")
			require.NoError(t, err)
			if mode == "removed" {
				require.NoError(t, os.Remove(path))
			} else {
				require.NoError(t, os.WriteFile(path, []byte("thresholds:\n  function-lines: 11\n"), 0600))
			}
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
	code := Run([]string{"--inspect-source", "--no-history"}, Invocation{Dir: root, Stdout: &stdout, Stderr: &stderr})
	require.Equal(t, 2, code)
	require.Empty(t, stdout.String())
	require.Contains(t, stderr.String(), "outside source root")
	files, err := filepath.Glob(filepath.Join(root, "*.sqlite"))
	require.NoError(t, err)
	require.Empty(t, files)
}

func TestInspectionRequiresModuleRoot(t *testing.T) {
	root := inspectionTestModule(t)
	subdirectory := filepath.Join(root, "sub")
	require.NoError(t, os.Mkdir(subdirectory, 0700))
	require.NoError(t, os.WriteFile(filepath.Join(subdirectory, "sub.go"), []byte("package sub\n"), 0600))
	input, err := validateInspectionInput(subdirectory, "")
	require.NoError(t, err)
	_, err = input.analyze(nil)
	require.ErrorContains(t, err, "root must be module root")
}
