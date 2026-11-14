package trials

import (
	"fmt"
	"strings"
	"testing"
)

func tree(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, text := range files {
		writeFile(t, dir+"/"+rel, text)
	}
	return dir
}

func numbered(n int, edit map[int]string) string {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		line, ok := edit[i]
		if !ok {
			line = fmt.Sprintf("line %d", i)
		}
		b.WriteString(line + "\n")
	}
	return b.String()
}

func TestAPatchReplaysTheTreeItWasMadeFrom(t *testing.T) {
	a := tree(t, map[string]string{
		"keep.go": "package p\n", "edit.go": numbered(30, nil), "gone.go": "package p\n\nfunc Gone() {}\n", "blank.txt": "x\n\n",
	})
	b := tree(t, map[string]string{
		"keep.go": "package p\n", "edit.go": numbered(31, map[int]string{2: "changed", 15: "moved", 31: "appended"}),
		"sub/added.go": "package sub\n", "blank.txt": "x\n\n\ny\n", "empty.txt": "",
	})
	_, patch, err := diffTrees(a, b)
	if err != nil {
		t.Fatal(err)
	}
	if err := applyPatch(a, patch); err != nil {
		t.Fatalf("apply: %v\n%s", err, patch)
	}
	got, _ := regularFiles(a)
	want, _ := regularFiles(b)
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("replayed tree\n%q\nwant\n%q", got, want)
	}
}

func TestAPatchRefusesATreeItWasNotMadeFrom(t *testing.T) {
	a := tree(t, map[string]string{"edit.go": numbered(10, nil)})
	b := tree(t, map[string]string{"edit.go": numbered(10, map[int]string{5: "changed"})})
	_, patch, err := diffTrees(a, b)
	if err != nil {
		t.Fatal(err)
	}
	other := tree(t, map[string]string{"edit.go": numbered(10, map[int]string{4: "drifted"})})
	if err := applyPatch(other, patch); err == nil {
		t.Fatal("a patch applied over a context line it does not match")
	}
}
