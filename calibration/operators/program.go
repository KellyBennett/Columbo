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

// program is a type-checked view of a module that includes _test.go files, so an operator
// that changes a signature can rewrite every call site, tests included. Functions are keyed
// by types.Func.FullName, which is stable across a package and its test variant.
type program struct {
	units      []unit
	calls      map[string][]clumpCallSite
	values     map[string]int
	ifaceNames map[string]bool
}

type clumpCallSite struct {
	unit
	call *ast.CallExpr
}

var (
	programsMu sync.Mutex
	programs   = map[string]*program{}
)

func programOf(c *harness.Corpus) (*program, error) {
	programsMu.Lock()
	defer programsMu.Unlock()
	if p, ok := programs[c.Root]; ok {
		return p, nil
	}
	p, err := loadProgram(c.Root)
	if err != nil {
		return nil, err
	}
	programs[c.Root] = p
	return p, nil
}

func loadProgram(root string) (*program, error) {
	cfg := &packages.Config{
		Dir:   root,
		Mode:  packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedImports,
		Tests: true,
		Env:   append(os.Environ(), "GOWORK=off", "GOFLAGS=-buildvcs=false"),
	}
	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, err
	}
	// A test variant ("p [p.test]") type-checks the package's own files together with its
	// internal tests, so prefer it over the plain package for files both contain.
	sort.SliceStable(pkgs, func(i, j int) bool { return strings.Contains(pkgs[i].ID, " [") && !strings.Contains(pkgs[j].ID, " [") })
	p := &program{calls: map[string][]clumpCallSite{}, values: map[string]int{}, ifaceNames: map[string]bool{}}
	seen := map[string]bool{}
	for _, pkg := range pkgs {
		if len(pkg.Errors) > 0 || strings.HasSuffix(pkg.ID, ".test") {
			continue
		}
		for i, f := range pkg.Syntax {
			path := pkg.CompiledGoFiles[i]
			rel, err := filepath.Rel(root, path)
			if err != nil || strings.HasPrefix(rel, "..") || seen[path] {
				continue
			}
			src, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			seen[path] = true
			p.units = append(p.units, unit{pkg: pkg, file: f, rel: filepath.ToSlash(rel), path: path, src: src})
		}
	}
	sort.Slice(p.units, func(i, j int) bool { return p.units[i].rel < p.units[j].rel })
	for _, u := range p.units {
		p.index(u)
	}
	return p, nil
}

func (p *program) index(u unit) {
	called := map[*ast.Ident]bool{}
	ast.Inspect(u.file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if id := calleeIdent(call.Fun); id != nil {
			if obj, ok := u.pkg.TypesInfo.Uses[id].(*types.Func); ok {
				called[id] = true
				k := funcKey(obj)
				p.calls[k] = append(p.calls[k], clumpCallSite{u, call})
			}
		}
		return true
	})
	for id, obj := range u.pkg.TypesInfo.Uses {
		if id.Pos() < u.file.FileStart || id.Pos() >= u.file.FileEnd {
			continue
		}
		if f, ok := obj.(*types.Func); ok && !called[id] {
			p.values[funcKey(f)]++
		}
		if tn, ok := obj.(*types.TypeName); ok {
			if it, ok := tn.Type().Underlying().(*types.Interface); ok {
				for i := 0; i < it.NumMethods(); i++ {
					p.ifaceNames[it.Method(i).Name()] = true
				}
			}
		}
	}
	for _, obj := range u.pkg.TypesInfo.Defs {
		if tn, ok := obj.(*types.TypeName); ok {
			if it, ok := tn.Type().Underlying().(*types.Interface); ok {
				for i := 0; i < it.NumMethods(); i++ {
					p.ifaceNames[it.Method(i).Name()] = true
				}
			}
		}
	}
}

func calleeIdent(e ast.Expr) *ast.Ident {
	switch e := ast.Unparen(e).(type) {
	case *ast.Ident:
		return e
	case *ast.SelectorExpr:
		return e.Sel
	}
	return nil
}

func funcKey(f *types.Func) string { return f.Origin().FullName() }

// funcs lists the function declarations in non-test files.
func (p *program) funcs() []fn {
	var out []fn
	for _, u := range p.units {
		if strings.HasSuffix(u.rel, "_test.go") || generated(u.file) {
			continue
		}
		for _, d := range u.file.Decls {
			if d, ok := d.(*ast.FuncDecl); ok {
				obj, _ := u.pkg.TypesInfo.Defs[d.Name].(*types.Func)
				if obj != nil {
					out = append(out, fn{unit: u, decl: d, symbol: symbolOf(u.pkg.PkgPath, d), obj: obj})
				}
			}
		}
	}
	return out
}

func (p *program) fn(symbol string) (fn, bool) {
	for _, f := range p.funcs() {
		if f.symbol == symbol {
			return f, true
		}
	}
	return fn{}, false
}

// rewritable reports why a function's signature cannot change safely: it is generic, used
// as a value, may satisfy an interface, or has a call site that spreads a multi-value call.
func (p *program) rewritable(f fn) error {
	sig := f.obj.Type().(*types.Signature)
	if f.decl.Body == nil || f.decl.Type.TypeParams != nil || (sig.Recv() != nil && isGenericRecv(sig)) {
		return errors.New("generic or bodyless")
	}
	k := funcKey(f.obj)
	if p.values[k] > 0 {
		return errors.New("used as a value")
	}
	if sig.Recv() != nil && p.ifaceNames[f.obj.Name()] {
		return errors.New("method name appears in an interface")
	}
	for _, cs := range p.calls[k] {
		if len(cs.call.Args) != sig.Params().Len() || cs.call.Ellipsis.IsValid() {
			return errors.New("call site spreads arguments")
		}
	}
	return nil
}

// params expands a declaration's parameter list into one entry per declared name, as
// Columbo's Parameters metric counts them; an unnamed field counts once.
type clumpParam struct {
	field *ast.Field
	name  *ast.Ident
	v     *types.Var
}

func params(f fn) []clumpParam {
	sig := f.obj.Type().(*types.Signature)
	var out []clumpParam
	i := 0
	for _, fld := range f.decl.Type.Params.List {
		if len(fld.Names) == 0 {
			out = append(out, clumpParam{field: fld, v: sig.Params().At(i)})
			i++
			continue
		}
		for _, n := range fld.Names {
			out = append(out, clumpParam{field: fld, name: n, v: sig.Params().At(i)})
			i++
		}
	}
	return out
}

func (p clumpParam) named() bool { return p.name != nil && p.name.Name != "_" }

// paramText prints one parameter, keeping the declaration's own spelling of its type.
func paramText(f fn, p clumpParam, unnamed bool) string {
	typ := f.text(p.field.Type)
	if unnamed || p.name == nil {
		return typ
	}
	return p.name.Name + " " + typ
}

var byteRune = regexp.MustCompile(`\b(byte|rune)\b`)

// clumpCanonical approximates Columbo's clumpCanonical type string closely enough to compare
// parameter types within one package.
func clumpCanonical(t types.Type) string {
	s := types.TypeString(types.Unalias(t), nil)
	return byteRune.ReplaceAllStringFunc(s, func(m string) string {
		if m == "byte" {
			return "uint8"
		}
		return "int32"
	})
}

func multiset(ts []string) map[string]int {
	m := map[string]int{}
	for _, t := range ts {
		m[t]++
	}
	return m
}

func contains(have, want map[string]int) bool {
	for t, n := range want {
		if have[t] < n {
			return false
		}
	}
	return true
}

func paramTypes(f fn) []string {
	var out []string
	for _, p := range params(f) {
		out = append(out, clumpCanonical(p.v.Type()))
	}
	return out
}

// usedNames collects every identifier spelled inside a declaration, so a clumpFresh name
// cannot capture or shadow anything the declaration refers to.
func usedNames(n ast.Node) map[string]bool {
	used := map[string]bool{}
	ast.Inspect(n, func(x ast.Node) bool {
		if id, ok := x.(*ast.Ident); ok {
			used[id.Name] = true
		}
		return true
	})
	return used
}

func clumpFresh(base string, used map[string]bool) string {
	name := base
	for n := 2; used[name] || token.IsKeyword(name) || types.Universe.Lookup(name) != nil; n++ {
		name = fmt.Sprintf("%s%d", base, n)
	}
	used[name] = true
	return name
}

func clumpLowerFirst(s string) string {
	r := []rune(s)
	r[0] = unicode.ToLower(r[0])
	return string(r)
}

func clumpUpperFirst(s string) string {
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

// pure expressions have no calls or receives, so moving or repeating them changes nothing.
func pure(e ast.Expr) bool {
	ok := true
	ast.Inspect(e, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.CallExpr, *ast.FuncLit:
			ok = false
		case *ast.UnaryExpr:
			ok = ok && n.Op != token.ARROW
		}
		return ok
	})
	return ok
}

// rewrite collects splices across files. Splices may nest: render returns a range's text
// with every already-recorded splice inside it applied and consumed, so an outer rewrite
// (a call-site argument) can carry an inner one (a renamed parameter use) along.
type rewrite struct {
	units map[string]unit
	edits map[string][]splice
}

func newRewrite() *rewrite { return &rewrite{units: map[string]unit{}, edits: map[string][]splice{}} }

func (r *rewrite) add(u unit, start, end int, text string) {
	r.units[u.rel] = u
	r.edits[u.rel] = append(r.edits[u.rel], splice{start, end, text})
}

func (r *rewrite) render(u unit, start, end int) string {
	var inner, rest []splice
	for _, s := range r.edits[u.rel] {
		if s.start >= start && s.end <= end && s.start < s.end {
			inner = append(inner, splice{s.start - start, s.end - start, s.text})
		} else {
			rest = append(rest, s)
		}
	}
	r.edits[u.rel] = rest
	return string(applySplices(u.src[start:end], inner))
}

func (r *rewrite) nodeText(u unit, n ast.Node) string {
	return r.render(u, u.offset(n.Pos()), u.offset(n.End()))
}

func (r *rewrite) edit() (harness.Edit, error) {
	out := harness.Edit{}
	for rel, ss := range r.edits {
		sort.Slice(ss, func(i, j int) bool { return ss[i].start < ss[j].start })
		for i := 1; i < len(ss); i++ {
			if ss[i].start < ss[i-1].end {
				return nil, fmt.Errorf("overlapping rewrites in %s", rel)
			}
		}
		u := r.units[rel]
		data, err := finish(u.path, applySplices(u.src, ss))
		if err != nil {
			return nil, err
		}
		out[rel] = data
	}
	return out, nil
}

// replaceParams splices a new parameter list text in place of a declaration's own.
func replaceParams(r *rewrite, f fn, list []string) {
	fl := f.decl.Type.Params
	r.add(f.unit, f.offset(fl.Opening)+1, f.offset(fl.Closing), strings.Join(list, ", "))
}

// replaceArgs splices new argument texts for a call, rendering nested rewrites first.
func replaceArgs(r *rewrite, cs clumpCallSite, args []string) {
	r.add(cs.unit, cs.offset(cs.call.Lparen)+1, cs.offset(cs.call.Rparen), strings.Join(args, ", "))
}

func argTexts(r *rewrite, cs clumpCallSite) []string {
	out := make([]string, len(cs.call.Args))
	for i, a := range cs.call.Args {
		out[i] = r.nodeText(cs.unit, a)
	}
	return out
}

// sortCallsInnermostFirst orders call sites so a call nested in another's arguments is
// rewritten first and its text travels inside the outer rewrite.
func sortCallsInnermostFirst(cs []clumpCallSite) {
	sort.SliceStable(cs, func(i, j int) bool {
		return cs[i].call.End()-cs[i].call.Pos() < cs[j].call.End()-cs[j].call.Pos()
	})
}

// qualifierFor names pkgPath as the unit spells it, or "" for the unit's own package.
func qualifierFor(u unit, pkgPath string) (string, bool) {
	if u.pkg.PkgPath == pkgPath {
		return "", true
	}
	for _, imp := range u.file.Imports {
		if strings.Trim(imp.Path.Value, `"`) != pkgPath {
			continue
		}
		if imp.Name != nil {
			if imp.Name.Name == "." || imp.Name.Name == "_" {
				return "", false
			}
			return imp.Name.Name + ".", true
		}
		for _, p := range u.pkg.Imports {
			if p.PkgPath == pkgPath {
				return p.Name + ".", true
			}
		}
	}
	return "", false
}
