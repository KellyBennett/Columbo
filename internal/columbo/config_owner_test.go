package columbo

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"
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
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			want := Defaults()
			want.Counts["function-lines"] = tc.count
			want.Severity["long-function"] = tc.severity
			sum := sha256.Sum256(data)
			for _, explicit := range []bool{false, true} {
				ordinary, ordinaryError := LoadConfig(path, explicit)
				captured, hash, capturedError := loadInspectionConfig(path, explicit)
				if !reflect.DeepEqual(ordinary, want) || !reflect.DeepEqual(captured, want) {
					t.Fatalf("partial/default config mismatch: %#v %#v", ordinary, captured)
				}
				if hash != hex.EncodeToString(sum[:]) {
					t.Fatalf("captured-byte hash mismatch: %q", hash)
				}
				for _, err := range []error{ordinaryError, capturedError} {
					if tc.message == "" {
						if err != nil {
							t.Fatal(err)
						}
						continue
					}
					if err == nil || err.Error() != tc.message {
						t.Fatalf("error %v, want %q", err, tc.message)
					}
				}
				if reflect.TypeOf(ordinaryError) != reflect.TypeOf(capturedError) {
					t.Fatal("error type changed")
				}
			}
			got, err := os.ReadFile(path)
			if err != nil || !reflect.DeepEqual(got, data) {
				t.Fatal("input bytes changed")
			}
		})
	}
}

func TestConfigOwnerReadContract(t *testing.T) {
	for _, directory := range []bool{false, true} {
		path := filepath.Join(t.TempDir(), "config.yml")
		if directory {
			if err := os.Mkdir(path, 0700); err != nil {
				t.Fatal(err)
			}
		}
		for _, explicit := range []bool{false, true} {
			ordinary, ordinaryError := LoadConfig(path, explicit)
			captured, hash, capturedError := loadInspectionConfig(path, explicit)
			if hash != "" || !reflect.DeepEqual(ordinary, Defaults()) || !reflect.DeepEqual(captured, Defaults()) {
				t.Fatal("read failure changed defaults or absent hash")
			}
			for _, err := range []error{ordinaryError, capturedError} {
				if !directory && !explicit {
					if err != nil {
						t.Fatal(err)
					}
					continue
				}
				var pe *os.PathError
				if !errors.As(err, &pe) || pe.Path != path {
					t.Fatalf("original path error missing: %v", err)
				}
				want := error(syscall.ENOENT)
				if directory {
					want = syscall.EISDIR
				}
				if !errors.Is(err, want) {
					t.Fatalf("original errno missing: %v", err)
				}
			}
			if ordinaryError != nil && ordinaryError.Error() != capturedError.Error() {
				t.Fatal("read error text changed")
			}
			ordinary.Counts["function-lines"] = 999
			if captured.Counts["function-lines"] != 10 || Defaults().Counts["function-lines"] != 10 {
				t.Fatal("defaults share mutable maps")
			}
		}
	}
}
