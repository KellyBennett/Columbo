package trials

import (
	"os"
	"path/filepath"
	"testing"
)

func module(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, src := range files {
		path := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

const original = "package p\n\nfunc Total(a, b int) int {\n\treturn sum(a, b)\n}\n\nfunc sum(a, b int) int { return a + b }\n"
const mutant = "package p\n\nfunc Total(a, b int) int {\n\ts := a + b\n\treturn s\n}\n\nfunc sum(a, b int) int { return a + b }\n"

func TestRestoringTheOriginalScoresZero(t *testing.T) {
	d, err := MeasureDistance(module(t, map[string]string{"p.go": original}), module(t, map[string]string{"p.go": mutant}), module(t, map[string]string{"p.go": original}))
	if err != nil {
		t.Fatal(err)
	}
	if d.Fixed != 0 || d.Mutant == 0 || d.Normalized() != 0 {
		t.Fatalf("distance %+v normalized %.2f, want fixed 0 and a nonzero mutant distance", d, d.Normalized())
	}
}

func TestLeavingTheMutantScoresOne(t *testing.T) {
	d, err := MeasureDistance(module(t, map[string]string{"p.go": original}), module(t, map[string]string{"p.go": mutant}), module(t, map[string]string{"p.go": mutant}))
	if err != nil {
		t.Fatal(err)
	}
	if d.Normalized() != 1 {
		t.Fatalf("normalized %.2f, want 1 for a fix that changed nothing", d.Normalized())
	}
}

func TestLineBreaksInsideAnExpressionCostNothing(t *testing.T) {
	wrapped := "package p\n\nfunc Total(a, b int) int {\n\treturn sum(a,\n\t\tb)\n}\n\nfunc sum(a, b int) int { return a + b }\n"
	d, err := MeasureDistance(module(t, map[string]string{"p.go": original}), module(t, map[string]string{"p.go": mutant}), module(t, map[string]string{"p.go": wrapped}))
	if err != nil {
		t.Fatal(err)
	}
	if d.Fixed != 0 {
		t.Fatalf("fixed distance %d, want 0 for a wrapped call", d.Fixed)
	}
}

func TestCommentsCountSoProseMutantsHaveADistance(t *testing.T) {
	commented := "package p\n\n// Total adds the two parts before sum sees them.\nfunc Total(a, b int) int {\n\treturn sum(a, b)\n}\n\nfunc sum(a, b int) int { return a + b }\n"
	d, err := MeasureDistance(module(t, map[string]string{"p.go": original}), module(t, map[string]string{"p.go": commented}), module(t, map[string]string{"p.go": original}))
	if err != nil {
		t.Fatal(err)
	}
	if d.Mutant != 1 || d.Fixed != 0 {
		t.Fatalf("distance %+v, want the comment to cost one token and its removal to restore the original", d)
	}
}

func TestCodeMovedToANewFileCountsAsAddedThere(t *testing.T) {
	split := map[string]string{
		"p.go":   "package p\n\nfunc Total(a, b int) int {\n\treturn sum(a, b)\n}\n",
		"sum.go": "package p\n\nfunc sum(a, b int) int { return a + b }\n",
	}
	d, err := MeasureDistance(module(t, map[string]string{"p.go": original}), module(t, map[string]string{"p.go": mutant}), module(t, split))
	if err != nil {
		t.Fatal(err)
	}
	if d.Fixed == 0 {
		t.Fatal("moving a declaration to a new file scored as the original")
	}
}
