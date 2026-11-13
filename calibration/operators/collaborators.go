package operators

import (
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"slices"
	"strings"
	"unicode"

	"github.com/KellyBennett/Columbo/calibration/harness"
)

// collaborator is a foreign named type whose zero value is safe to construct and query,
// so adding it changes a declaration's dependency set and nothing else.
type collaborator struct {
	path, name, typ, use string
}

func (c collaborator) identity() string { return "type:" + c.path + "." + c.typ }

var collaboratorPool = []collaborator{
	{"strings", "strings", "Builder", "Len()"},
	{"bytes", "bytes", "Buffer", "Len()"},
	{"container/list", "list", "List", "Len()"},
	{"math/big", "big", "Int", "Sign()"},
	{"sort", "sort", "IntSlice", "Len()"},
	{"time", "time", "Duration", "Hours()"},
	{"net/url", "url", "URL", "String()"},
	{"text/scanner", "scanner", "Position", "IsValid()"},
	{"image", "image", "Point", "String()"},
	{"bufio", "bufio", "Writer", "Buffered()"},
	{"hash/maphash", "maphash", "Hash", "Sum64()"},
	{"go/token", "token", "Position", "IsValid()"},
	{"container/ring", "ring", "Ring", "Len()"},
	{"math/big", "big", "Float", "Sign()"},
	{"math/big", "big", "Rat", "Sign()"},
	{"strings", "strings", "Reader", "Len()"},
	{"bytes", "bytes", "Reader", "Len()"},
	{"time", "time", "Month", "String()"},
}

const collaboratorVar = "calCollab"

// addCollaborators is the inverse of splitting a declaration by collaborator group: it
// makes a function construct and query dose new foreign named types. Doses straddle the
// dependencies limit: the largest that should stay quiet and the smallest that should fire.
type addCollaborators struct{}

func (addCollaborators) ID() string           { return "add-collaborators" }
func (addCollaborators) Class() harness.Class { return harness.Inject }
func (addCollaborators) Target() string       { return "excessive-dependencies" }

func (o addCollaborators) Sites(c *harness.Corpus) []harness.Site {
	var out []harness.Site
	for _, f := range funcs(c) {
		if f.decl.Body == nil || f.baselineHas(c, o.Target(), f.symbol) {
			continue
		}
		if len(o.Doses(c, f.site())) > 0 {
			out = append(out, f.site())
		}
	}
	return out
}

func (o addCollaborators) Doses(c *harness.Corpus, site harness.Site) []int {
	f, ok := findFn(c, site.Symbol)
	if !ok {
		return nil
	}
	need := int(threshold(c, "dependencies", 5)) - len(c.Baseline.Dependencies[f.symbol]) + 1
	if need > len(freshCollaborators(c, f)) {
		return nil
	}
	if need <= 1 {
		return []int{1}
	}
	return []int{need - 1, need}
}

func (o addCollaborators) Apply(c *harness.Corpus, site harness.Site, dose int) (harness.Applied, error) {
	f, ok := findFn(c, site.Symbol)
	if !ok {
		return harness.Applied{}, errors.New("site vanished")
	}
	fresh := freshCollaborators(c, f)
	if dose > len(fresh) {
		return harness.Applied{}, errors.New("not enough fresh collaborators")
	}
	var b strings.Builder
	for i, col := range fresh[:dose] {
		name := fmt.Sprintf("%s%d", collaboratorVar, i+1)
		fmt.Fprintf(&b, "\nvar %s %s.%s\n_ = %s.%s", name, col.name, col.typ, name, col.use)
	}
	at := f.offset(f.decl.Body.Lbrace) + 1
	out, err := finish(f.path, applySplices(f.src, []splice{{at, at, b.String()}}))
	if err != nil {
		return harness.Applied{}, err
	}
	before := len(c.Baseline.Dependencies[f.symbol])
	limit := int(threshold(c, "dependencies", 5))
	kill := before+dose > limit
	return harness.Applied{
		Edit: harness.Edit{f.rel: out},
		Expectation: harness.Expectation{
			Touched: []string{f.symbol},
			Prediction: harness.Prediction{Kill: &kill, Reason: fmt.Sprintf(
				"%d scored dependencies + %d new collaborators = %d (limit %d)", before, dose, before+dose, limit)},
		},
	}, nil
}

// freshCollaborators lists the pool entries that would each add exactly one scored
// identity: not already scored, not the function's own package, its package not already
// scored as a bare package reference, and its package name free in the file.
func freshCollaborators(c *harness.Corpus, f fn) []collaborator {
	scored := c.Baseline.Dependencies[f.symbol]
	var out []collaborator
	for _, col := range collaboratorPool {
		if col.path == f.pkg.PkgPath || slices.Contains(scored, col.identity()) ||
			slices.Contains(scored, "package:"+col.path) || !nameFree(f.unit, col) {
			continue
		}
		out = append(out, col)
	}
	return out
}

func nameFree(u unit, col collaborator) bool {
	for _, imp := range u.file.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		name := pathBase(path)
		if imp.Name != nil {
			name = imp.Name.Name
		}
		if name == col.name {
			return path == col.path && imp.Name == nil
		}
	}
	for id := range u.pkg.TypesInfo.Defs {
		if id.Name == col.name && u.pkg.Fset.File(id.Pos()) == u.pkg.Fset.File(u.file.Pos()) {
			return false
		}
	}
	return u.pkg.Types.Scope().Lookup(col.name) == nil
}

func pathBase(path string) string {
	if i := strings.LastIndex(path, "/"); i >= 0 {
		return path[i+1:]
	}
	return path
}

// routeThroughFacade is the cosmetic evade for added collaborators: it moves them into one
// same-package unexported struct whose methods forward each query, so the function names
// only the facade, which the dependency metric never scores.
type routeThroughFacade struct{}

func (routeThroughFacade) ID() string           { return "route-through-facade" }
func (routeThroughFacade) Class() harness.Class { return harness.Evade }
func (routeThroughFacade) Target() string       { return "excessive-dependencies" }
func (routeThroughFacade) Follows() string      { return "add-collaborators" }
func (routeThroughFacade) Doses(*harness.Corpus, harness.Site) []int {
	return []int{1}
}

func (o routeThroughFacade) Sites(c *harness.Corpus) []harness.Site {
	var out []harness.Site
	for _, f := range funcs(c) {
		if len(addedCollaborators(f)) > 0 {
			out = append(out, f.site())
		}
	}
	return out
}

type addedCollaborator struct {
	name, typ, use, result string
	decl, query            ast.Stmt
}

// addedCollaborators recognizes the statement pairs add-collaborators inserts.
func addedCollaborators(f fn) []addedCollaborator {
	if f.decl.Body == nil {
		return nil
	}
	qual := qualifier(f.unit)
	var out []addedCollaborator
	stmts := f.decl.Body.List
	for i := 0; i+1 < len(stmts); i++ {
		spec, ok := varSpec(stmts[i])
		if !ok || len(spec.Names) != 1 || !strings.HasPrefix(spec.Names[0].Name, collaboratorVar) || spec.Type == nil {
			continue
		}
		assign, ok := stmts[i+1].(*ast.AssignStmt)
		if !ok || len(assign.Rhs) != 1 {
			continue
		}
		call, ok := assign.Rhs[0].(*ast.CallExpr)
		if !ok {
			continue
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			continue
		}
		if id, ok := sel.X.(*ast.Ident); !ok || id.Name != spec.Names[0].Name {
			continue
		}
		tv := f.pkg.TypesInfo.Types[call]
		out = append(out, addedCollaborator{
			name: spec.Names[0].Name, typ: f.text(spec.Type), use: f.text(call)[len(spec.Names[0].Name)+1:],
			result: types.TypeString(tv.Type, qual), decl: stmts[i], query: stmts[i+1],
		})
	}
	return out
}

func varSpec(s ast.Stmt) (*ast.ValueSpec, bool) {
	ds, ok := s.(*ast.DeclStmt)
	if !ok {
		return nil, false
	}
	gd, ok := ds.Decl.(*ast.GenDecl)
	if !ok || gd.Tok != token.VAR || len(gd.Specs) != 1 {
		return nil, false
	}
	vs, ok := gd.Specs[0].(*ast.ValueSpec)
	return vs, ok && len(vs.Values) == 0
}

func (o routeThroughFacade) Apply(c *harness.Corpus, site harness.Site, _ int) (harness.Applied, error) {
	f, ok := findFn(c, site.Symbol)
	if !ok {
		return harness.Applied{}, errors.New("site vanished")
	}
	added := addedCollaborators(f)
	if len(added) == 0 {
		return harness.Applied{}, errors.New("no added collaborators")
	}
	facade := freeName(f, lowerFirst(f.decl.Name.Name)+"Collaborators")
	local := freeName(f, "calFacade")
	var fields, methods strings.Builder
	var ss []splice
	for i, a := range added {
		fmt.Fprintf(&fields, "%s %s\n", a.name, a.typ)
		fmt.Fprintf(&methods, "\n\nfunc (f *%s) %sValue() %s {\nreturn f.%s.%s\n}", facade, a.name, a.result, a.name, a.use)
		ss = append(ss, lineAware(f.src, f.offset(a.decl.Pos()), f.offset(a.decl.End())))
		query := fmt.Sprintf("_ = %s.%sValue()", local, a.name)
		if i == 0 {
			query = fmt.Sprintf("var %s %s\n%s", local, facade, query)
		}
		ss = append(ss, splice{f.offset(a.query.Pos()), f.offset(a.query.End()), query})
	}
	end := f.offset(f.decl.End())
	ss = append(ss, splice{end, end, fmt.Sprintf("\n\ntype %s struct {\n%s}%s", facade, fields.String(), methods.String())})
	out, err := finish(f.path, applySplices(f.src, ss))
	if err != nil {
		return harness.Applied{}, err
	}
	return harness.Applied{
		Edit:        harness.Edit{f.rel: out},
		Expectation: harness.Expectation{Touched: []string{f.symbol}, Catches: []string{"excessive-dependencies", "cosmetic-extraction"}},
	}, nil
}

func lowerFirst(s string) string {
	r := []rune(s)
	r[0] = unicode.ToLower(r[0])
	return string(r)
}

// freeName returns base, or base with the smallest numeric suffix, that no package-level
// object and no identifier in the function's file already uses.
func freeName(f fn, base string) string {
	used := map[string]bool{}
	ast.Inspect(f.file, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok {
			used[id.Name] = true
		}
		return true
	})
	for n := 0; ; n++ {
		name := base
		if n > 0 {
			name = fmt.Sprintf("%s%d", base, n)
		}
		if !used[name] && f.pkg.Types.Scope().Lookup(name) == nil {
			return name
		}
	}
}
