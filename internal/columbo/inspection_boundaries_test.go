package columbo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInspectionInventoryAcceptancePrecedesClassification(t *testing.T) {
	root := inspectionTestModule(t)
	a, err := load(root, []string{"./..."}, Defaults())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "model.go"), []byte("stale"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "a-not-go")
	if err := os.Symlink("absent", link); err != nil {
		t.Fatal(err)
	}
	files, err := a.inspectionFiles()
	if err == nil || err.Error() != "symlink source path refused: "+link || len(files) != 0 {
		t.Fatalf("earlier non-Go symlink must win: %v %#v", err, files)
	}
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	files, err = a.inspectionFiles()
	if err == nil || err.Error() != "source changed during inspection: model.go" || len(files) != 1 || files[0].Path != "generated.go" {
		t.Fatalf("stale selected source must retain earlier entries: %v %#v", err, files)
	}
}

func TestInspectionInventoryPrunesGitAndRetainsNonSelectedBytes(t *testing.T) {
	root := inspectionTestModule(t)
	a, err := load(root, []string{"./..."}, Defaults())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, ".git"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("absent", filepath.Join(root, ".git", "bad.go")); err != nil {
		t.Fatal(err)
	}
	data := []byte("changed non-selected source")
	if err := os.WriteFile(filepath.Join(root, "platform_windows.go"), data, 0600); err != nil {
		t.Fatal(err)
	}
	files, err := a.inspectionFiles()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if f.Path == "platform_windows.go" && (f.SHA != inspectionHash(data) || f.Selected || f.Target || !strings.HasPrefix(f.Scope, "not-selected-runtime")) {
			t.Fatalf("non-selected physical bytes changed scope: %#v", f)
		}
		if f.Path == "model_test.go" && (f.Selected || f.Scope != "test source: not selected") {
			t.Fatalf("test scope: %#v", f)
		}
	}
	if len(files) != 4 {
		t.Fatalf("inventory: %#v", files)
	}
}

func TestInspectionLexicalBindingsRemainPhysicalAndOneHop(t *testing.T) {
	root := inspectionTestModule(t)
	facts := inspectTestFacts(t, root)
	for _, d := range facts.Declarations {
		for _, c := range d.Calls {
			for _, span := range []*inspectionSpan{c.Binding, c.Receiver} {
				if span == nil {
					continue
				}
				data, err := os.ReadFile(filepath.Join(root, span.Path))
				if err != nil {
					t.Fatal(err)
				}
				if span.FileSHA != inspectionHash(data) || span.SHA != inspectionHash(data[span.StartOffset:span.EndOffset]) {
					t.Fatalf("physical binding changed: %#v", span)
				}
			}
			if c.Target != nil && len(c.Target.Calls) != 0 {
				t.Fatal("expanded beyond one hop")
			}
		}
	}
}
