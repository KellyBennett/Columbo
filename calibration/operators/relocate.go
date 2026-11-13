package operators

import (
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"unicode"

	"github.com/KellyBennett/Columbo/calibration/harness"
	"golang.org/x/tools/go/packages"
)

// relocation is one Move Function over a declaration f. When to is a parameter index,
// that parameter becomes the receiver; when to is -1 the declaration becomes a free
// function. The old receiver, if any, becomes the first parameter. Every call site in the
// package and its tests is rewritten.
type relocation struct {
	f      fn
	to     int
	name   string
	params []param
	calls  []callSite
}

type param struct {
	name, typ string
	v         *types.Var
}

type callSite struct {
	u    unit
	call *ast.CallExpr
}

const toFunction = -1

func planRelocation(c *harness.Corpus, f fn, to int) (relocation, error) {
	d := f.decl
	if d.Body == nil || f.obj == nil || d.Type.TypeParams != nil {
		return relocation{}, errors.New("no body or generic")
	}
	sig := f.obj.Type().(*types.Signature)
	if sig.Recv() != nil && isGenericRecv(sig) {
		return relocation{}, errors.New("generic receiver")
	}
	if sig.Recv() == nil && to == toFunction {
		return relocation{}, errors.New("already a function")
	}
	r := relocation{f: f, to: to}
	params, err := flatParams(f)
	if err != nil {
		return relocation{}, err
	}
	r.params = params
	if sig.Recv() != nil {
		recvT, _ := localNamed(sig.Recv().Type(), f.pkg.Types)
		if recvT == nil || satisfiesInterface(c, recvT, d.Name.Name) {
			return relocation{}, errors.New("receiver type or an interface needs the method")
		}
	}
	if to == toFunction {
		r.name = freeName(c, f)
	} else {
		if to >= len(params) || sig.Variadic() && to == len(params)-1 {
			return relocation{}, errors.New("no such parameter")
		}
		target, ok := localNamed(params[to].v.Type(), f.pkg.Types)
		if !ok {
			return relocation{}, errors.New("parameter type cannot take a method")
		}
		r.name = d.Name.Name
		if obj, _, _ := types.LookupFieldOrMethod(target, true, f.pkg.Types, r.name); obj != nil {
			return relocation{}, fmt.Errorf("%s already has %s", target.Obj().Name(), r.name)
		}
	}
	calls, err := callSites(c, f)
	if err != nil {
		return relocation{}, err
	}
	for _, cs := range calls {
		if err := r.checkCall(cs); err != nil {
			return relocation{}, err
		}
	}
	r.calls = calls
	return r, nil
}

func flatParams(f fn) ([]param, error) {
	var out []param
	for _, field := range f.decl.Type.Params.List {
		if len(field.Names) == 0 {
			return nil, errors.New("unnamed parameters")
		}
		for _, n := range field.Names {
			v, _ := f.pkg.TypesInfo.Defs[n].(*types.Var)
			out = append(out, param{name: n.Name, typ: f.text(field.Type), v: v})
		}
	}
	return out, nil
}

// checkCall rejects a site the rewrite would change: an untyped argument cannot become a
// receiver, and moving an argument with calls ahead of earlier calls reorders effects.
func (r relocation) checkCall(cs callSite) error {
	if r.to == toFunction {
		return nil
	}
	info := cs.u.pkg.TypesInfo
	if len(cs.call.Args) <= r.to || cs.call.Ellipsis.IsValid() {
		return errors.New("call shape")
	}
	arg := cs.call.Args[r.to]
	if tv, ok := info.Types[arg]; !ok || untyped(tv.Type) {
		return errors.New("untyped receiver argument")
	}
	if !hasCall(arg) {
		return nil
	}
	earlier := append([]ast.Expr{}, cs.call.Args[:r.to]...)
	if s, ok := cs.call.Fun.(*ast.SelectorExpr); ok && r.f.decl.Recv != nil {
		earlier = append(earlier, s.X)
	}
	for _, e := range earlier {
		if hasCall(e) {
			return errors.New("reordering would change call order")
		}
	}
	return nil
}

func hasCall(e ast.Node) bool {
	found := false
	ast.Inspect(e, func(n ast.Node) bool {
		if _, ok := n.(*ast.CallExpr); ok {
			found = true
		}
		return !found
	})
	return found
}

func freeName(c *harness.Corpus, f fn) string {
	base := lowerFirst(receiverName(f.decl.Recv.List[0].Type)) + upperFirst(f.decl.Name.Name)
	base = strings.TrimPrefix(base, "*")
	taken := identNames(c, f.pkg.PkgPath)
	name := base
	for i := 2; taken[name] || f.pkg.Types.Scope().Lookup(name) != nil || token.Lookup(name).IsKeyword(); i++ {
		name = fmt.Sprintf("%s%d", base, i)
	}
	return name
}

func lowerFirst(s string) string {
	s = strings.TrimPrefix(s, "*")
	r := []rune(s)
	if len(r) == 0 {
		return s
	}
	r[0] = unicode.ToLower(r[0])
	return string(r)
}

func upperFirst(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return s
	}
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

func identNames(c *harness.Corpus, pkgPath string) map[string]bool {
	names := map[string]bool{}
	us, _ := packageUnits(c, pkgPath)
	for _, u := range us {
		ast.Inspect(u.file, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok {
				names[id.Name] = true
			}
			return true
		})
	}
	return names
}

func (r relocation) newSymbol() string {
	d := &ast.FuncDecl{Name: ast.NewIdent(r.name)}
	if r.to != toFunction {
		d.Recv = &ast.FieldList{List: []*ast.Field{{Type: paramTypeExpr(r.f, r.to)}}}
	}
	return symbolOf(r.f.pkg.PkgPath, d)
}

func paramTypeExpr(f fn, idx int) ast.Expr {
	i := 0
	for _, field := range f.decl.Type.Params.List {
		for range field.Names {
			if i == idx {
				return field.Type
			}
			i++
		}
	}
	return nil
}

func (r relocation) header() string {
	var ps []string
	if r.f.decl.Recv != nil {
		recv := r.f.decl.Recv.List[0]
		name := "_"
		if len(recv.Names) > 0 {
			name = recv.Names[0].Name
		}
		ps = append(ps, name+" "+r.f.text(recv.Type))
	}
	recv := ""
	for i, p := range r.params {
		if i == r.to {
			recv = "(" + p.name + " " + p.typ + ") "
			continue
		}
		ps = append(ps, p.name+" "+p.typ)
	}
	return " " + recv + r.name + "(" + strings.Join(ps, ", ") + ")"
}

func (r relocation) edit() (harness.Edit, error) {
	byUnit := map[string][]splice{}
	units := map[string]unit{}
	d := r.f.decl
	units[r.f.rel] = r.f.unit
	byUnit[r.f.rel] = append(byUnit[r.f.rel], splice{r.f.offset(d.Type.Func) + len("func"), r.f.offset(d.Type.Params.End()), r.header()})
	for _, cs := range r.calls {
		if r.nested(cs) {
			continue
		}
		units[cs.u.rel] = cs.u
		byUnit[cs.u.rel] = append(byUnit[cs.u.rel], splice{cs.u.offset(cs.call.Pos()), cs.u.offset(cs.call.End()), r.render(cs)})
	}
	edit := harness.Edit{}
	for rel, ss := range byUnit {
		out, err := finish(units[rel].path, applySplices(units[rel].src, ss))
		if err != nil {
			return nil, err
		}
		edit[rel] = out
	}
	return edit, nil
}

func (r relocation) nested(cs callSite) bool {
	for _, o := range r.calls {
		if o.call != cs.call && o.u.rel == cs.u.rel && o.call.Pos() <= cs.call.Pos() && cs.call.End() <= o.call.End() {
			return true
		}
	}
	return false
}

// textOf prints n's source with every call site inside it rewritten, so nested calls
// such as x.M(x.M(1)) come out whole.
func (r relocation) textOf(u unit, n ast.Node) string {
	var ss []splice
	start := u.offset(n.Pos())
	for _, cs := range r.calls {
		if cs.u.rel != u.rel || cs.call.Pos() < n.Pos() || cs.call.End() > n.End() || ast.Node(cs.call) == n {
			continue
		}
		inner := false
		for _, o := range r.calls {
			if o.call != cs.call && o.u.rel == u.rel && o.call.Pos() >= n.Pos() && o.call.End() <= n.End() && ast.Node(o.call) != n &&
				o.call.Pos() <= cs.call.Pos() && cs.call.End() <= o.call.End() {
				inner = true
			}
		}
		if !inner {
			ss = append(ss, splice{u.offset(cs.call.Pos()) - start, u.offset(cs.call.End()) - start, r.render(cs)})
		}
	}
	return string(applySplices(u.src[start:u.offset(n.End())], ss))
}

func (r relocation) render(cs callSite) string {
	info := cs.u.pkg.TypesInfo
	var args []string
	if s, ok := cs.call.Fun.(*ast.SelectorExpr); ok && r.f.decl.Recv != nil {
		recvPtr := isPointer(r.f.obj.Type().(*types.Signature).Recv().Type())
		xPtr := isPointer(info.TypeOf(s.X))
		x := r.textOf(cs.u, s.X)
		switch {
		case recvPtr && !xPtr:
			x = "&" + operand(s.X, x)
		case !recvPtr && xPtr:
			x = "*" + operand(s.X, x)
		}
		args = append(args, x)
	}
	recv := ""
	for i, a := range cs.call.Args {
		text := r.textOf(cs.u, a)
		if i == r.to {
			recv = operand(a, text) + "."
			continue
		}
		args = append(args, text)
	}
	if cs.call.Ellipsis.IsValid() {
		args[len(args)-1] += "..."
	}
	return recv + r.name + "(" + strings.Join(args, ", ") + ")"
}

func isPointer(t types.Type) bool {
	_, ok := t.(*types.Pointer)
	return ok
}

// operand parenthesizes e's text unless it is already a primary expression.
func operand(e ast.Expr, text string) string {
	switch e.(type) {
	case *ast.Ident, *ast.SelectorExpr, *ast.IndexExpr, *ast.CallExpr, *ast.ParenExpr:
		return text
	}
	return "(" + text + ")"
}

// callSites finds every use of f. Each must be a direct call from f's own package or its
// internal tests; any other use (method value, promotion, another package) is an error.
func callSites(c *harness.Corpus, f fn) ([]callSite, error) {
	pkgUnits, err := packageUnits(c, f.pkg.PkgPath)
	if err != nil {
		return nil, err
	}
	for _, p := range c.Packages {
		if p.PkgPath == f.pkg.PkgPath {
			continue
		}
		for _, obj := range p.TypesInfo.Uses {
			if sameFunc(obj, f) {
				return nil, fmt.Errorf("used from %s", p.PkgPath)
			}
		}
	}
	if ast.IsExported(f.decl.Name.Name) && mentionedInOtherTests(c, f) {
		return nil, errors.New("possibly used from another package's tests")
	}
	var out []callSite
	for _, u := range pkgUnits {
		parents := parentMap(u.file)
		for id, obj := range u.pkg.TypesInfo.Uses {
			if !sameFunc(obj, f) || u.pkg.Fset.File(id.Pos()) != u.pkg.Fset.File(u.file.Pos()) {
				continue
			}
			if u.pkg.PkgPath != f.pkg.PkgPath {
				return nil, errors.New("used from an external test package")
			}
			call, err := directCall(u, parents, id, f.decl.Recv != nil)
			if err != nil {
				return nil, err
			}
			out = append(out, callSite{u: u, call: call})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].u.rel != out[j].u.rel {
			return out[i].u.rel < out[j].u.rel
		}
		return out[i].call.Pos() < out[j].call.Pos()
	})
	return out, nil
}

func directCall(u unit, parents map[ast.Node]ast.Node, id *ast.Ident, method bool) (*ast.CallExpr, error) {
	var fun ast.Expr = id
	if method {
		s, ok := parents[id].(*ast.SelectorExpr)
		if !ok || s.Sel != id {
			return nil, errors.New("unexpected method reference")
		}
		sel := u.pkg.TypesInfo.Selections[s]
		if sel == nil || sel.Kind() != types.MethodVal || len(sel.Index()) != 1 {
			return nil, errors.New("method expression or promoted method")
		}
		fun = s
	}
	call, ok := parents[fun].(*ast.CallExpr)
	if !ok || call.Fun != fun {
		return nil, errors.New("used as a value")
	}
	return call, nil
}

func sameFunc(obj types.Object, f fn) bool {
	g, ok := obj.(*types.Func)
	if !ok || g.Name() != f.decl.Name.Name || g.Pkg() == nil || g.Pkg().Path() != f.pkg.PkgPath {
		return false
	}
	sig := g.Type().(*types.Signature)
	want := f.obj.Type().(*types.Signature).Recv()
	if sig.Recv() == nil || want == nil {
		return sig.Recv() == nil && want == nil
	}
	return recvTypeName(sig.Recv().Type()) == recvTypeName(want.Type())
}

func recvTypeName(t types.Type) string {
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	if n, ok := t.(*types.Named); ok {
		return n.Obj().Name()
	}
	return ""
}

type testLoad struct {
	once  sync.Once
	units []unit
	err   error
}

var testLoads sync.Map

// packageUnits is the package's own files plus its internal and external test files, each
// type-checked in its test variant, because the corpus load excludes tests.
func packageUnits(c *harness.Corpus, pkgPath string) ([]unit, error) {
	var out []unit
	for _, u := range units(c) {
		if u.pkg.PkgPath == pkgPath {
			out = append(out, u)
		}
	}
	v, _ := testLoads.LoadOrStore(c.Root+"|"+pkgPath, &testLoad{})
	tl := v.(*testLoad)
	tl.once.Do(func() { tl.units, tl.err = loadTestUnits(c.Root, pkgPath) })
	return append(out, tl.units...), tl.err
}

func loadTestUnits(root, pkgPath string) ([]unit, error) {
	cfg := &packages.Config{
		Dir:   root,
		Tests: true,
		Mode:  packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo,
		Env:   append(os.Environ(), "GOWORK=off", "GOFLAGS=-buildvcs=false"),
	}
	pkgs, err := packages.Load(cfg, pkgPath)
	if err != nil {
		return nil, err
	}
	var out []unit
	for _, p := range pkgs {
		if !strings.Contains(p.ID, ".test]") || (p.PkgPath != pkgPath && p.PkgPath != pkgPath+"_test") {
			continue
		}
		if len(p.Errors) > 0 {
			return nil, fmt.Errorf("load tests %s: %v", p.ID, p.Errors[0])
		}
		for i, f := range p.Syntax {
			path := p.CompiledGoFiles[i]
			if !strings.HasSuffix(path, "_test.go") {
				continue
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return nil, err
			}
			src, err := os.ReadFile(path)
			if err != nil {
				return nil, err
			}
			out = append(out, unit{pkg: p, file: f, rel: filepath.ToSlash(rel), path: path, src: src})
		}
	}
	return out, nil
}

func mentionedInOtherTests(c *harness.Corpus, f fn) bool {
	re := regexp.MustCompile(`\b` + regexp.QuoteMeta(f.decl.Name.Name) + `\b`)
	dir := filepath.Dir(f.path)
	found := false
	_ = filepath.WalkDir(c.Root, func(path string, d os.DirEntry, err error) error {
		if err != nil || found {
			return filepath.SkipDir
		}
		if d.IsDir() {
			if path != c.Root && (d.Name() == "testdata" || strings.HasPrefix(d.Name(), ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, "_test.go") && filepath.Dir(path) != dir {
			if src, err := os.ReadFile(path); err == nil && re.Match(src) {
				found = true
			}
		}
		return nil
	})
	return found
}

type ifaceSet struct {
	once   sync.Once
	ifaces []*types.Interface
}

var ifaceSets sync.Map

// satisfiesInterface reports whether t or *t implements any interface the corpus or its
// imports declare that names method, because moving the method off t would break it.
func satisfiesInterface(c *harness.Corpus, t *types.Named, method string) bool {
	v, _ := ifaceSets.LoadOrStore(c, &ifaceSet{})
	set := v.(*ifaceSet)
	set.once.Do(func() { set.ifaces = collectInterfaces(c) })
	for _, iface := range set.ifaces {
		has := false
		for i := 0; i < iface.NumMethods(); i++ {
			has = has || iface.Method(i).Name() == method
		}
		if has && (types.Implements(t, iface) || types.Implements(types.NewPointer(t), iface)) {
			return true
		}
	}
	return false
}

func collectInterfaces(c *harness.Corpus) []*types.Interface {
	var out []*types.Interface
	seen := map[*types.Package]bool{}
	add := func(t types.Type) {
		if n, ok := t.(*types.Named); ok && n.TypeParams().Len() > 0 {
			return
		}
		if i, ok := t.Underlying().(*types.Interface); ok && i.NumMethods() > 0 {
			out = append(out, i.Complete())
		}
	}
	var visit func(p *types.Package)
	visit = func(p *types.Package) {
		if seen[p] {
			return
		}
		seen[p] = true
		for _, name := range p.Scope().Names() {
			if tn, ok := p.Scope().Lookup(name).(*types.TypeName); ok {
				add(tn.Type())
			}
		}
		for _, imp := range p.Imports() {
			visit(imp)
		}
	}
	add(types.Universe.Lookup("error").Type())
	for _, p := range c.Packages {
		visit(p.Types)
		for _, tv := range p.TypesInfo.Types {
			if _, isNamed := tv.Type.(*types.Named); !isNamed && tv.Type != nil {
				if i, ok := tv.Type.(*types.Interface); ok && i.NumMethods() > 0 {
					out = append(out, i.Complete())
				}
			}
		}
	}
	return out
}
