package columbo

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigOwnerDecodeContract(t *testing.T) {
	for _, tc := range []struct {
		name     string
		data     string
		message  string
		count    int64
		severity string
	}{
		{"empty", "", "", 10, "fail"},
		{"valid", "thresholds:\n  function-lines: 11\n", "", 11, "fail"},
		{"partial-count", "thresholds:\n  function-lines: 11\n  parameters: 0\nunknown: true\n", "invalid positive integer threshold parameters", 11, "fail"},
		{"partial-severity", "severity:\n  long-function: warn\nunknown: true\nversion: 2\n", "unknown configuration key unknown", 10, "warn"},
		{"version-first", "version: 2\nunknown: true\n", "version must be integer 1", 10, "fail"},
		{"unknown-first", "unknown: true\nversion: 2\n", "unknown configuration key unknown", 10, "fail"},
		{"duplicate-before-setter", "version: 2\nversion: 1\n", "duplicate key version", 10, "fail"},
		{"null-before-setter", "version: 2\nunknown: null\n", "nulls, aliases, anchors and merge keys are not allowed", 10, "fail"},
		{"documents-before-setter", "version: 2\n---\nversion: 1\n", "configuration must contain exactly one YAML document", 10, "fail"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yml")
			data := []byte(tc.data)
			require.NoError(t, os.WriteFile(path, data, 0600))
			want := Defaults()
			want.Counts["function-lines"] = tc.count
			want.Severity["long-function"] = tc.severity
			sum := sha256.Sum256(data)
			for _, explicit := range []bool{false, true} {
				ordinary, ordinaryError := LoadConfig(path, explicit)
				captured, hash, capturedError := loadInspectionConfig(path, explicit)
				assert.Equal(t, want, ordinary, "ordinary partial/default configuration")
				assert.Equal(t, want, captured, "captured partial/default configuration")
				assert.Equal(t, hex.EncodeToString(sum[:]), hash, "hash of captured bytes")
				for _, err := range []error{ordinaryError, capturedError} {
					if tc.message == "" {
						assert.NoError(t, err)
					} else {
						assert.EqualError(t, err, tc.message)
					}
				}
				assert.IsType(t, ordinaryError, capturedError, "configuration error type")
			}
			got, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.Equal(t, data, got, "configuration input bytes must not change")
		})
	}
}

func TestConfigOwnerReadContract(t *testing.T) {
	for _, tc := range []struct {
		name      string
		directory bool
		explicit  bool
		wantError error
	}{
		{name: "implicit-missing"},
		{name: "explicit-missing", explicit: true, wantError: syscall.ENOENT},
		{name: "implicit-directory", directory: true, wantError: syscall.EISDIR},
		{name: "explicit-directory", directory: true, explicit: true, wantError: syscall.EISDIR},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yml")
			if tc.directory {
				require.NoError(t, os.Mkdir(path, 0700))
			}
			ordinary, ordinaryError := LoadConfig(path, tc.explicit)
			captured, hash, capturedError := loadInspectionConfig(path, tc.explicit)
			assert.Empty(t, hash, "unreadable configuration must not have a captured hash")
			assert.Equal(t, Defaults(), ordinary)
			assert.Equal(t, Defaults(), captured)
			for _, err := range []error{ordinaryError, capturedError} {
				if tc.wantError == nil {
					assert.NoError(t, err)
					continue
				}
				var pe *os.PathError
				require.ErrorAs(t, err, &pe, "original path error")
				assert.Equal(t, path, pe.Path)
				assert.ErrorIs(t, err, tc.wantError, "original errno")
			}
			if ordinaryError != nil {
				assert.EqualError(t, capturedError, ordinaryError.Error(), "read error text")
			}
			require.NotNil(t, ordinary.Counts)
			ordinary.Counts["function-lines"] = 999
			assert.Equal(t, int64(10), captured.Counts["function-lines"], "captured defaults must have independent maps")
			assert.Equal(t, int64(10), Defaults().Counts["function-lines"], "fresh defaults must have independent maps")
		})
	}
}
