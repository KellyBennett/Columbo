package operators

import (
	"errors"
	"go/build"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/KellyBennett/Columbo/calibration/harness"
)

// moveDeclToFile moves one function declaration into a new file of the same package. It
// only moves declarations with no comments, because prose-comment cases are anchored per
// file and a moved comment would legitimately move its case.
type moveDeclToFile struct{}

func (moveDeclToFile) ID() string                                { return "move-decl-to-file" }
func (moveDeclToFile) Class() harness.Class                      { return harness.Neutral }
func (moveDeclToFile) Target() string                            { return "" }
func (moveDeclToFile) Doses(*harness.Corpus, harness.Site) []int { return []int{1} }

func (moveDeclToFile) Sites(c *harness.Corpus) []harness.Site {
	var out []harness.Site
	for _, f := range funcs(c) {
		if declMovable(c, f) {
			out = append(out, f.site())
		}
	}
	return out
}

func (moveDeclToFile) Apply(c *harness.Corpus, site harness.Site, _ int) (harness.Applied, error) {
	f, ok := findFn(c, site.Symbol)
	if !ok || !declMovable(c, f) {
		return harness.Applied{}, errors.New("site vanished or is no longer declMovable")
	}
	rel := movedRel(f)
	start, end := f.offset(f.decl.Pos()), f.offset(f.decl.End())
	rest, err := finish(f.path, applySplices(f.src, []splice{{start, end, ""}}))
	if err != nil {
		return harness.Applied{}, err
	}
	moved := "package " + f.file.Name.Name + "\n\n" + importBlock(f.unit) + "\n" + string(f.src[start:end]) + "\n"
	created, err := finish(filepath.Join(c.Root, filepath.FromSlash(rel)), []byte(moved))
	if err != nil {
		return harness.Applied{}, err
	}
	return harness.Applied{
		Edit: harness.Edit{f.rel: rest, rel: created},
		Expectation: harness.Expectation{
			Touched: []string{f.symbol},
			Moves:   map[string]string{f.symbol: rel},
		},
	}, nil
}

func declMovable(c *harness.Corpus, f fn) bool {
	name := f.decl.Name.Name
	if name == "_" || name == "init" && f.decl.Recv == nil || f.decl.Doc != nil {
		return false
	}
	for _, g := range f.file.Comments {
		if g.Pos() < f.decl.End() && g.End() > f.decl.Pos() {
			return false
		}
	}
	for _, imp := range f.file.Imports {
		if imp.Path.Value == `"C"` || imp.Name != nil && imp.Name.Name == "." {
			return false
		}
	}
	if _, err := os.Stat(filepath.Join(c.Root, filepath.FromSlash(movedRel(f)))); err == nil {
		return false
	}
	return unconstrained(f.path)
}

// unconstrained reports whether a file builds on every platform, so its declarations can
// move to a file with no build constraint. A //go:build line or a GOOS or GOARCH file-name
// suffix fails one of two disjoint platforms.
func unconstrained(file string) bool {
	for _, p := range [][2]string{{"linux", "amd64"}, {"windows", "arm64"}} {
		ctx := build.Default
		ctx.GOOS, ctx.GOARCH, ctx.CgoEnabled = p[0], p[1], false
		if ok, err := ctx.MatchFile(filepath.Dir(file), filepath.Base(file)); err != nil || !ok {
			return false
		}
	}
	return true
}

// movedRel names the new file after the declaration. The _decl suffix keeps the name clear
// of _test and of GOOS and GOARCH suffixes.
func movedRel(f fn) string {
	name := strings.ToLower(f.decl.Name.Name)
	if f.decl.Recv != nil {
		name = strings.ToLower(strings.TrimPrefix(receiverName(f.decl.Recv.List[0].Type), "*")) + "_" + name
	}
	return path.Join(path.Dir(f.rel), "moved_"+name+"_decl.go")
}

// importBlock copies the source file's named and plain imports so finish resolves the moved
// declaration's selectors to the same packages; finish drops the ones it does not use.
func importBlock(u unit) string {
	var b strings.Builder
	b.WriteString("import (\n")
	for _, imp := range u.file.Imports {
		if imp.Name != nil && imp.Name.Name == "_" {
			continue
		}
		b.WriteString(u.text(imp) + "\n")
	}
	b.WriteString(")\n")
	return b.String()
}
