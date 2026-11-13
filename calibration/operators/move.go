package operators

import (
	"errors"
	"fmt"
	"go/types"
	"go/build"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/KellyBennett/Columbo/calibration/harness"
)

const envySmell = "feature-envy"

// moveMethod is the inverse of Move Function. Without collaborator it turns a method into
// a free function that takes the receiver as its first parameter. With collaborator it
// moves the method onto the same-package type of one of its parameters, the plan's
// "envied owner's collaborator". Either way the old receiver's own accesses become
// foreign accesses. Collaborator dose is the parameter index plus one.
type moveMethod struct {
	id           string
	collaborator bool
}

func (o moveMethod) ID() string           { return o.id }
func (o moveMethod) Class() harness.Class { return harness.Inject }
func (o moveMethod) Target() string       { return envySmell }

func (o moveMethod) Sites(c *harness.Corpus) []harness.Site {
	l := limitsOf(c)
	var out []harness.Site
	for _, f := range funcs(c) {
		recv := recvVarOf(f)
		if recv == nil || f.decl.Body == nil || f.baselineHas(c, envySmell, f.symbol) {
			continue
		}
		if profileOf(f.pkg.TypesInfo, f.decl.Body, recv).own < l.foreign-1 {
			continue
		}
		if len(o.Doses(c, f.site())) > 0 {
// moveDeclToFile moves one function declaration into a new file of the same package. It
// only moves declarations with no comments, because prose-comment cases are anchored per
// file and a moved comment would legitimately move its case.
type moveDeclToFile struct{}
func (moveDeclToFile) ID() string                                { return "move-decl-to-file" }
func (moveDeclToFile) Class() harness.Class                      { return harness.Neutral }
func (moveDeclToFile) Target() string                            { return "" }
func (moveDeclToFile) Doses(*harness.Corpus, harness.Site) []int { return []int{1} }
func (moveDeclToFile) Sites(c *harness.Corpus) []harness.Site {
		if movable(c, f) {
			out = append(out, f.site())
		}
	}
	return out
}

func (o moveMethod) Doses(c *harness.Corpus, site harness.Site) []int {
	f, ok := findFn(c, site.Symbol)
	if !ok {
		return nil
	}
	if !o.collaborator {
		if _, err := planRelocation(c, f, toFunction); err != nil {
			return nil
		}
		return []int{1}
	}
	recvT, _ := localNamed(recvVarOf(f).Type(), f.pkg.Types)
	sig := f.obj.Type().(*types.Signature)
	var doses []int
	for i := 0; i < sig.Params().Len(); i++ {
		if t, ok := localNamed(sig.Params().At(i).Type(), f.pkg.Types); !ok || t == recvT {
			continue
		}
		if _, err := planRelocation(c, f, i); err == nil {
			doses = append(doses, i+1)
		}
	}
	return doses
}

func (o moveMethod) Apply(c *harness.Corpus, site harness.Site, dose int) (harness.Applied, error) {
	f, ok := findFn(c, site.Symbol)
	if !ok {
		return harness.Applied{}, errors.New("site vanished")
	}
	to := toFunction
	var newOwn *types.Var
	if o.collaborator {
		to = dose - 1
		newOwn = f.obj.Type().(*types.Signature).Params().At(to)
	}
	r, err := planRelocation(c, f, to)
	if err != nil {
		return harness.Applied{}, err
	}
	edit, err := r.edit()
	if err != nil {
		return harness.Applied{}, err
	}
	moved := r.newSymbol()
	return harness.Applied{
		Edit: edit,
		Expectation: harness.Expectation{
			Touched:    []string{moved},
			Renames:    map[string]string{f.symbol: moved},
			Prediction: predictEnvy(profileOf(f.pkg.TypesInfo, f.decl.Body, newOwn), limitsOf(c), "after the move"),
		},
	}, nil
}

// moveFunction is Move Function, the cure. For each baseline feature-envy case whose
// qualifying group is a bare parameter of a same-package named type, it moves the
// declaration onto that type. Dose is the parameter index plus one.
type moveFunction struct{}

func (moveFunction) ID() string           { return "move-function" }
func (moveFunction) Class() harness.Class { return harness.Cure }
func (moveFunction) Target() string       { return envySmell }

func (o moveFunction) Sites(c *harness.Corpus) []harness.Site {
	var out []harness.Site
	for _, f := range funcs(c) {
		if f.decl.Body != nil && f.baselineHas(c, envySmell, f.symbol) && len(o.Doses(c, f.site())) > 0 {
			out = append(out, f.site())
		}
	}
	return out
}

func (moveFunction) Doses(c *harness.Corpus, site harness.Site) []int {
	f, ok := findFn(c, site.Symbol)
	if !ok || f.obj == nil {
		return nil
	}
	sig := f.obj.Type().(*types.Signature)
	params := map[*types.Var]int{}
	for i := 0; i < sig.Params().Len(); i++ {
		params[sig.Params().At(i)] = i
	}
	p := profileOf(f.pkg.TypesInfo, f.decl.Body, recvVarOf(f))
	groups := p.qualifying(limitsOf(c))
	sortedGroups(groups)
	var doses []int
	for _, g := range groups {
		i, isParam := params[g.root]
		if !g.bare() || !isParam {
			continue
		}
		if _, err := planRelocation(c, f, i); err == nil {
			doses = append(doses, i+1)
		}
	}
	return doses
}

func (moveFunction) Apply(c *harness.Corpus, site harness.Site, dose int) (harness.Applied, error) {
	f, ok := findFn(c, site.Symbol)
	if !ok {
		return harness.Applied{}, errors.New("site vanished")
	}
	to := dose - 1
	r, err := planRelocation(c, f, to)
	if err != nil {
		return harness.Applied{}, err
	}
	edit, err := r.edit()
	if err != nil {
		return harness.Applied{}, err
	}
	after := profileOf(f.pkg.TypesInfo, f.decl.Body, r.params[to].v)
	l := limitsOf(c)
	clears := len(after.qualifying(l)) == 0
	moved := r.newSymbol()
	return harness.Applied{
		Edit: edit,
		Expectation: harness.Expectation{
			Touched: []string{f.symbol},
			Renames: map[string]string{f.symbol: moved},
			Prediction: harness.Prediction{
				Kill:   &clears,
				Reason: fmt.Sprintf("on %s: %s", r.params[to].name, after.describe(l)),
			},
		},
	}, nil
}
func (moveDeclToFile) Apply(c *harness.Corpus, site harness.Site, _ int) (harness.Applied, error) {
	if !ok || !movable(c, f) {
		return harness.Applied{}, errors.New("site vanished or is no longer movable")
	rel := movedRel(f)
	start, end := f.offset(f.decl.Pos()), f.offset(f.decl.End())
	rest, err := finish(f.path, applySplices(f.src, []splice{{start, end, ""}}))
	moved := "package " + f.file.Name.Name + "\n\n" + importBlock(f.unit) + "\n" + string(f.src[start:end]) + "\n"
	created, err := finish(filepath.Join(c.Root, filepath.FromSlash(rel)), []byte(moved))
		Edit: harness.Edit{f.rel: rest, rel: created},
			Moves:   map[string]string{f.symbol: rel},
func movable(c *harness.Corpus, f fn) bool {
	name := f.decl.Name.Name
	if name == "_" || name == "init" && f.decl.Recv == nil || f.decl.Doc != nil {
		return false
	for _, g := range f.file.Comments {
		if g.Pos() < f.decl.End() && g.End() > f.decl.Pos() {
			return false
	for _, imp := range f.file.Imports {
		if imp.Path.Value == `"C"` || imp.Name != nil && imp.Name.Name == "." {
			return false
	if _, err := os.Stat(filepath.Join(c.Root, filepath.FromSlash(movedRel(f)))); err == nil {
		return false
	return unconstrained(f.path)
// unconstrained reports whether a file builds on every platform, so its declarations can
// move to a file with no build constraint. A //go:build line or a GOOS or GOARCH file-name
// suffix fails one of two disjoint platforms.
func unconstrained(file string) bool {
	for _, p := range [][2]string{{"linux", "amd64"}, {"windows", "arm64"}} {
		ctx := build.Default
		ctx.GOOS, ctx.GOARCH, ctx.CgoEnabled = p[0], p[1], false
		if ok, err := ctx.MatchFile(filepath.Dir(file), filepath.Base(file)); err != nil || !ok {
			return false
	return true
// movedRel names the new file after the declaration. The _decl suffix keeps the name clear
// of _test and of GOOS and GOARCH suffixes.
func movedRel(f fn) string {
	name := strings.ToLower(f.decl.Name.Name)
	if f.decl.Recv != nil {
		name = strings.ToLower(strings.TrimPrefix(receiverName(f.decl.Recv.List[0].Type), "*")) + "_" + name
	return path.Join(path.Dir(f.rel), "moved_"+name+"_decl.go")
// importBlock copies the source file's named and plain imports so finish resolves the moved
// declaration's selectors to the same packages; finish drops the ones it does not use.
func importBlock(u unit) string {
	var b strings.Builder
	b.WriteString("import (\n")
	for _, imp := range u.file.Imports {
		if imp.Name != nil && imp.Name.Name == "_" {
		b.WriteString(u.text(imp) + "\n")
	b.WriteString(")\n")
	return b.String()
