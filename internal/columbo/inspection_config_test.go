package columbo

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInspectionConfigurationSnapshotMatchesOrdinaryLoad(t *testing.T) {
	for _, tc := range []struct {
		name      string
		data      string
		exists    bool
		explicit  bool
		directory bool
	}{
		{name: "valid", data: "thresholds:\n  function-lines: 11\n", exists: true},
		{name: "explicit-valid", data: "exclude: []\n", exists: true, explicit: true},
		{name: "empty", exists: true},
		{name: "implicit-missing"},
		{name: "explicit-missing", explicit: true},
		{name: "invalid-yaml", data: "[", exists: true},
		{name: "unknown-key", data: "unknown: true\n", exists: true},
		{name: "invalid-threshold", data: "thresholds:\n  function-lines: 0\n", exists: true},
		{name: "read-error", directory: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yml")
			if tc.exists {
				require.NoError(t, os.WriteFile(path, []byte(tc.data), 0600))
			}
			if tc.directory {
				require.NoError(t, os.Mkdir(path, 0700))
			}
			ordinary, ordinaryError := LoadConfig(path, tc.explicit)
			captured, hash, capturedError := loadInspectionConfig(path, tc.explicit)
			assert.Equal(t, ordinary, captured, "ordinary configuration semantics")
			assert.IsType(t, ordinaryError, capturedError, "ordinary configuration error type")
			if ordinaryError != nil {
				assert.EqualError(t, capturedError, ordinaryError.Error(), "ordinary configuration error text")
			} else {
				assert.NoError(t, capturedError)
			}
			want := ""
			if tc.exists {
				want = inspectionHash([]byte(tc.data))
			}
			assert.Equal(t, want, hash, "hash of the bytes used to decode configuration")
		})
	}
}

func TestInspectionConfigurationSnapshotSurvivesLaterFilesystemChanges(t *testing.T) {
	initial := []byte("thresholds:\n  function-lines: 11\n")
	writeLater := func(path string) error {
		return os.WriteFile(path, []byte("thresholds:\n  function-lines: 12\n"), 0600)
	}
	for _, tc := range []struct {
		name      string
		exists    bool
		change    func(string) error
		wantHash  string
		wantLimit int64
	}{
		{name: "mutate", exists: true, change: writeLater, wantHash: inspectionHash(initial), wantLimit: 11},
		{name: "create", change: writeLater, wantLimit: 10},
		{name: "remove", exists: true, change: os.Remove, wantHash: inspectionHash(initial), wantLimit: 11},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), ".columbo.yml")
			if tc.exists {
				require.NoError(t, os.WriteFile(path, initial, 0600))
			}
			config, hash, err := loadInspectionConfig(path, false)
			require.NoError(t, err)
			require.NoError(t, tc.change(path))
			assert.Equal(t, tc.wantLimit, config.Counts["function-lines"], "later filesystem state must not alter captured configuration")
			assert.Equal(t, tc.wantHash, hash, "later filesystem state must not alter captured hash")
		})
	}
}

func TestInspectionConfigurationErrorsPrecedeLoading(t *testing.T) {
	root := inspectionTestModule(t)
	require.NoError(t, os.WriteFile(filepath.Join(root, "invalid.go"), []byte("package fixture\nfunc Broken(){missing()}"), 0600))
	path := filepath.Join(root, ".columbo.yml")
	require.NoError(t, os.WriteFile(path, []byte("["), 0600))
	_, expected := LoadConfig(path, false)
	require.Error(t, expected)
	_, err := validateInspectionInput(root, "")
	assert.EqualError(t, err, expected.Error(), "configuration error must precede source loading")
	_, err = validateInspectionInput(root, "missing.yml")
	assert.ErrorIs(t, err, os.ErrNotExist, "explicit missing config must precede loading")
	_, err = validateInspectionInput(root, "../outside.yml")
	assert.EqualError(t, err, "inspection configuration must be inside source root", "source-bound path check must precede config read")
	link := filepath.Join(t.TempDir(), "linked")
	require.NoError(t, os.Symlink(root, link))
	_, err = validateInspectionInput(link, "../outside.yml")
	assert.EqualError(t, err, "symlink source root refused: "+link, "root check must precede config path check")
}

func TestInspectionConfigurationCapturedInReport(t *testing.T) {
	root := inspectionTestModule(t)
	data := []byte("thresholds:\n  function-lines: 11\n")
	require.NoError(t, os.WriteFile(filepath.Join(root, "settings.yml"), data, 0600))
	input, err := validateInspectionInput(root, "settings.yml")
	require.NoError(t, err)
	report, err := input.analyze(nil)
	require.NoError(t, err)
	facts := report.inspection
	require.NotNil(t, facts)
	assert.Equal(t, "settings.yml", facts.ConfigFile)
	assert.Equal(t, inspectionHash(data), facts.ConfigSHA)
	assert.Equal(t, int64(11), facts.Config.Counts["function-lines"])
}
