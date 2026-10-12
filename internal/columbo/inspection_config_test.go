package columbo

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
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
				if err := os.WriteFile(path, []byte(tc.data), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if tc.directory {
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			}
			ordinary, ordinaryError := LoadConfig(path, tc.explicit)
			captured, hash, capturedError := loadInspectionConfig(path, tc.explicit)
			if !reflect.DeepEqual(ordinary, captured) || reflect.TypeOf(ordinaryError) != reflect.TypeOf(capturedError) {
				t.Fatalf("snapshot differs from ordinary config semantics: %v, %v", ordinaryError, capturedError)
			}
			if ordinaryError != nil {
				if ordinaryError.Error() != capturedError.Error() {
					t.Fatalf("snapshot changed config error: %q != %q", ordinaryError, capturedError)
				}
				return
			}
			want := ""
			if tc.exists {
				want = inspectionHash([]byte(tc.data))
			}
			if hash != want {
				t.Fatalf("snapshot hash %q differs from decoded bytes %q", hash, want)
			}
		})
	}
}

func TestInspectionConfigurationSnapshotSurvivesLaterFilesystemChanges(t *testing.T) {
	for _, mode := range []string{"mutate", "create", "remove"} {
		t.Run(mode, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), ".columbo.yml")
			initial := []byte("thresholds:\n  function-lines: 11\n")
			wantHash, wantLimit := "", int64(10)
			if mode != "create" {
				if err := os.WriteFile(path, initial, 0600); err != nil {
					t.Fatal(err)
				}
				wantHash, wantLimit = inspectionHash(initial), 11
			}
			config, hash, err := loadInspectionConfig(path, false)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "remove" {
				err = os.Remove(path)
			} else {
				err = os.WriteFile(path, []byte("thresholds:\n  function-lines: 12\n"), 0600)
			}
			if err != nil {
				t.Fatal(err)
			}
			if config.Counts["function-lines"] != wantLimit || hash != wantHash {
				t.Fatal("later filesystem state altered the captured configuration")
			}
		})
	}
}

func TestInspectionConfigurationErrorsPrecedeLoading(t *testing.T) {
	root := inspectionTestModule(t)
	if err := os.WriteFile(filepath.Join(root, "invalid.go"), []byte("package fixture\nfunc Broken(){missing()}"), 0600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ".columbo.yml")
	if err := os.WriteFile(path, []byte("["), 0600); err != nil {
		t.Fatal(err)
	}
	_, expected := LoadConfig(path, false)
	_, err := validateInspectionInput(root, "")
	if err == nil || err.Error() != expected.Error() {
		t.Fatalf("configuration error must precede source loading: %v", err)
	}
	if _, err := validateInspectionInput(root, "missing.yml"); !os.IsNotExist(err) {
		t.Fatalf("explicit missing config must precede loading: %v", err)
	}
	if _, err := validateInspectionInput(root, "../outside.yml"); err == nil || err.Error() != "inspection configuration must be inside source root" {
		t.Fatalf("source-bound path check must precede config read: %v", err)
	}
	link := filepath.Join(t.TempDir(), "linked")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	if _, err := validateInspectionInput(link, "../outside.yml"); err == nil || !strings.HasPrefix(err.Error(), "symlink source root refused:") {
		t.Fatalf("root check must precede config path check: %v", err)
	}
}

func TestInspectionConfigurationCapturedInReport(t *testing.T) {
	root := inspectionTestModule(t)
	data := []byte("thresholds:\n  function-lines: 11\n")
	if err := os.WriteFile(filepath.Join(root, "settings.yml"), data, 0600); err != nil {
		t.Fatal(err)
	}
	input, err := validateInspectionInput(root, "settings.yml")
	if err != nil {
		t.Fatal(err)
	}
	report, err := input.analyze(nil)
	if err != nil {
		t.Fatal(err)
	}
	facts := report.inspection
	if facts.ConfigFile != "settings.yml" || facts.ConfigSHA != inspectionHash(data) || facts.Config.Counts["function-lines"] != 11 {
		t.Fatal("inspection must contain the same source-bound configuration snapshot")
	}
}
