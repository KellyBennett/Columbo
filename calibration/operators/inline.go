package operators

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"strings"

	"github.com/KellyBennett/Columbo/calibration/harness"
	"github.com/uudashr/gocognit"
)

// inlineFunction is Inline Function, the inverse of Extract Function. Dose is the number
// of call sites inlined into the parent, in source order.
type inlineFunction struct {
	id, target string
}

const maxInlineDose = 8

func (o inlineFunction) ID() string           { return o.id }
func (o inlineFunction) Class() harness.Class { return harness.Inject }
func (o inlineFunction) Target() string       { return o.target }

func (o inlineFunction) Sites(c *harness.Corpus) []harness.Site {
	var out []harness.Site
	for _, f := range funcs(c) {
		if f.decl.Body == nil || f.baselineHas(c, o.target, f.symbol) {
			continue
		}
		if len(inlineCalls(c, f)) > 0 {
			out = append(out, f.site())
		}
	}
	return out
}

func (o inlineFunction) Doses(c *harness.Corpus, site harness.Site) []int {
	f, ok := findFn(c, site.Symbol)
	if !ok {
		return nil
	}
	n := min(len(inlineCalls(c, f)), maxInlineDose)
	doses := make([]int, n)
	for i := range doses {
		doses[i] = i + 1
	}
	return doses
}

func (o inlineFunction) Apply(c *harness.Corpus, site harness.Site, dose int) (harness.Applied, error) {
	f, ok := findFn(c, site.Symbol)
	if !ok {
		return harness.Applied{}, errors.New("site vanished")
	}
	calls := inlineCalls(c, f)
	if dose > len(calls) {
		return harness.Applied{}, errors.New("dose exceeds call sites")
	}
	var ss []splice
	for k, call := range calls[:dose] {
		text, err := call.expand(k + 1)
		if err != nil {
			return harness.Applied{}, err
		}
		ss = append(ss, splice{f.offset(call.stmt.Pos()), f.offset(call.stmt.End()), text})
	}
	out, err := finish(f.path, applySplices(f.src, ss))
	if err != nil {
		return harness.Applied{}, err
	}
	lines, complexity, err := metricsOf(out, f.decl.Name.Name, f.decl.Recv != nil)
	if err != nil {
		return harness.Applied{}, err
	}
	var kill bool
	var reason string
	switch o.target {
	case "long-function":
		limit := int(threshold(c, "function-lines", 10))
		kill, reason = lines > limit, fmt.Sprintf("parent has %d lines after inlining (limit %d)", lines, limit)
	default:
		limit := int(threshold(c, "cognitive-complexity", 7))
		kill, reason = complexity > limit, fmt.Sprintf("parent has complexity %d after inlining (limit %d)", complexity, limit)
	}
	return harness.Applied{
		Edit: harness.Edit{f.rel: out},
		Expectation: harness.Expectation{
			Touched:    []string{f.symbol},
			Prediction: harness.Prediction{Kill: &kill, Reason: reason},
		},
	}, nil
}

func metricsOf(src []byte, name string, method bool) (lines, complexity int, err error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", src, parser.ParseComments)
	if err != nil {
		return 0, 0, err
	}
	for _, d := range file.Decls {
		if d, ok := d.(*ast.FuncDecl); ok && d.Name.Name == name && (d.Recv != nil) == method {
			return bodyLines(src, d.Body, fset), gocognit.Complexity(d), nil
		}
	}
	return 0, 0, fmt.Errorf("%s not found after rewrite", name)
}

type inlineCall struct {
	parent fn
	callee fn
	stmt   ast.Stmt
	call   *ast.CallExpr
	lhs    []ast.Expr
	define bool
	ret    bool
}

func inlineCalls(c *harness.Corpus, parent fn) []inlineCall {
	var out []inlineCall
	ast.Inspect(parent.decl.Body, func(n ast.Node) bool {
		if _, lit := n.(*ast.FuncLit); lit {
			return false
		}
		stmt, ok := n.(ast.Stmt)
		if !ok {
			return true
		}
		ic, ok := asInlineCall(stmt)
		if !ok {
			return true
		}
		ic.parent = parent
		callee, ok := resolveCallee(c, parent, ic.call)
		if !ok || callee.symbol == parent.symbol || !inlinable(callee) || !compatible(ic, callee) || shadows(parent, ic.call, callee) {
			return true
		}
		ic.callee = callee
		out = append(out, ic)
		return true
	})
	return out
}

func asInlineCall(stmt ast.Stmt) (inlineCall, bool) {
	switch s := stmt.(type) {
	case *ast.ExprStmt:
		if call, ok := s.X.(*ast.CallExpr); ok {
			return inlineCall{stmt: s, call: call}, true
		}
	case *ast.AssignStmt:
		if len(s.Rhs) != 1 || (s.Tok != token.DEFINE && s.Tok != token.ASSIGN) {
			return inlineCall{}, false
		}
		if call, ok := s.Rhs[0].(*ast.CallExpr); ok {
			return inlineCall{stmt: s, call: call, lhs: s.Lhs, define: s.Tok == token.DEFINE}, true
		}
	case *ast.ReturnStmt:
		if len(s.Results) == 1 {
			if call, ok := s.Results[0].(*ast.CallExpr); ok {
				return inlineCall{stmt: s, call: call, ret: true}, true
			}
		}
	}
	return inlineCall{}, false
}

func resolveCallee(c *harness.Corpus, parent fn, call *ast.CallExpr) (fn, bool) {
	var id *ast.Ident
	switch f := call.Fun.(type) {
	case *ast.Ident:
		id = f
	case *ast.SelectorExpr:
		if _, isIdent := f.X.(*ast.Ident); !isIdent {
			return fn{}, false
		}
		sel := parent.pkg.TypesInfo.Selections[f]
		if sel == nil || sel.Kind() != types.MethodVal || sel.Indirect() {
			return fn{}, false
		}
		id = f.Sel
	default:
		return fn{}, false
	}
	obj, ok := parent.pkg.TypesInfo.Uses[id].(*types.Func)
	if !ok || obj.Pkg() != parent.pkg.Types {
		return fn{}, false
	}
	for _, f := range funcs(c) {
		if f.obj == obj {
			return f, true
		}
	}
	return fn{}, false
}

// inlinable is the restricted straight-line case the plan names: no defer, recover, go,
// labels, named results, generics, variadics, or early returns.
func inlinable(f fn) bool {
	d := f.decl
	if d.Body == nil || d.Type.TypeParams != nil || f.obj == nil {
		return false
	}
	sig := f.obj.Type().(*types.Signature)
	if sig.Variadic() || (sig.Recv() != nil && isGenericRecv(sig)) {
		return false
	}
	if d.Type.Results != nil {
		for _, r := range d.Type.Results.List {
			if len(r.Names) > 0 {
				return false
			}
		}
	}
	ok := true
	returns := 0
	ast.Inspect(d.Body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.DeferStmt, *ast.GoStmt, *ast.LabeledStmt, *ast.BranchStmt:
			if b, isBranch := n.(*ast.BranchStmt); isBranch && b.Label == nil && b.Tok != token.GOTO {
				return true
			}
			ok = false
		case *ast.FuncLit:
			return false
		case *ast.ReturnStmt:
			returns++
		case *ast.CallExpr:
			if id, isIdent := n.Fun.(*ast.Ident); isIdent && (id.Name == "recover" || id.Name == "panic") {
				ok = false
			}
		}
		return ok
	})
	if !ok {
		return false
	}
	list := d.Body.List
	if sig.Results().Len() == 0 {
		return returns == 0
	}
	if returns != 1 || len(list) == 0 {
		return false
	}
	last, isReturn := list[len(list)-1].(*ast.ReturnStmt)
	return isReturn && len(last.Results) == sig.Results().Len()
}

func isGenericRecv(sig *types.Signature) bool {
	return sig.RecvTypeParams() != nil && sig.RecvTypeParams().Len() > 0
}

func compatible(ic inlineCall, callee fn) bool {
	results := callee.obj.Type().(*types.Signature).Results().Len()
	switch {
	case ic.ret:
		return results >= 1
	case ic.lhs != nil:
		return len(ic.lhs) == results
	}
	return true
}

// shadows reports whether a package-level name the callee uses resolves to something
// else at the call site, which would silently rebind it after inlining.
func shadows(parent fn, call *ast.CallExpr, callee fn) bool {
	scope := parent.pkg.Types.Scope().Innermost(call.Pos())
	if scope == nil {
		return true
	}
	bad := false
	ast.Inspect(callee.decl.Body, func(n ast.Node) bool {
		id, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		obj := callee.pkg.TypesInfo.Uses[id]
		if obj == nil || local(callee, obj) {
			return true
		}
		if _, got := scope.LookupParent(id.Name, call.Pos()); got != nil && got != obj {
			if _, isPkg := obj.(*types.PkgName); !isPkg {
				bad = true
			}
		}
		return !bad
	})
	return bad
}

func local(f fn, obj types.Object) bool {
	return obj.Pos() >= f.decl.Pos() && obj.Pos() < f.decl.End() && obj.Parent() != nil && obj.Parent() != obj.Pkg().Scope()
}

func (ic inlineCall) expand(k int) (string, error) {
	callee := ic.callee
	suffix := fmt.Sprintf("Inl%d", k)
	sig := callee.obj.Type().(*types.Signature)
	qual := qualifier(ic.parent.unit)
	subst := map[types.Object]string{}
	var b strings.Builder
	bind := func(v *types.Var, argExpr ast.Expr) {
		arg := ic.parent.text(argExpr)
		if v == nil || v.Name() == "" || v.Name() == "_" {
			fmt.Fprintf(&b, "_ = %s\n", arg)
			return
		}
		uses, mutated := usage(callee, v)
		if simple(argExpr) && !mutated {
			subst[v] = arg
			return
		}
		fmt.Fprintf(&b, "var %s %s = %s\n", v.Name()+suffix, types.TypeString(v.Type(), qual), arg)
		if uses == 0 {
			fmt.Fprintf(&b, "_ = %s\n", v.Name()+suffix)
		}
	}
	if sig.Recv() != nil {
		bind(recvVar(callee), ic.call.Fun.(*ast.SelectorExpr).X)
	}
	for i := 0; i < sig.Params().Len(); i++ {
		bind(sig.Params().At(i), ic.call.Args[i])
	}
	body := callee.decl.Body.List
	var final *ast.ReturnStmt
	if sig.Results().Len() > 0 {
		final = body[len(body)-1].(*ast.ReturnStmt)
		body = body[:len(body)-1]
	}
	for _, s := range body {
		b.WriteString(renameLocals(callee, s, suffix, subst))
		b.WriteString("\n")
	}
	if final == nil {
		return b.String(), nil
	}
	results := make([]string, len(final.Results))
	for i, r := range final.Results {
		results[i] = renameLocals(callee, r, suffix, subst)
		if tv, ok := callee.pkg.TypesInfo.Types[r]; ok && untyped(tv.Type) && !ic.ret {
			results[i] = fmt.Sprintf("(%s)(%s)", types.TypeString(sig.Results().At(i).Type(), qual), results[i])
		}
	}
	joined := strings.Join(results, ", ")
	switch {
	case ic.ret:
		fmt.Fprintf(&b, "return %s", joined)
	case ic.lhs != nil:
		lhs := make([]string, len(ic.lhs))
		for i, l := range ic.lhs {
			lhs[i] = ic.parent.text(l)
		}
		op := "="
		if ic.define {
			op = ":="
		}
		fmt.Fprintf(&b, "%s %s %s", strings.Join(lhs, ", "), op, joined)
	default:
		for _, r := range results {
			fmt.Fprintf(&b, "_ = %s\n", r)
		}
	}
	return b.String(), nil
}

func recvVar(f fn) *types.Var {
	names := f.decl.Recv.List[0].Names
	if len(names) == 0 {
		return nil
	}
	v, _ := f.pkg.TypesInfo.Defs[names[0]].(*types.Var)
	return v
}

func untyped(t types.Type) bool {
	b, ok := t.(*types.Basic)
	return ok && b.Info()&types.IsUntyped != 0
}

// simple arguments are substituted textually: re-evaluating them has no effect and no cost.
func simple(e ast.Expr) bool {
	switch e := e.(type) {
	case *ast.Ident, *ast.BasicLit:
		return true
	case *ast.SelectorExpr:
		return simple(e.X)
	case *ast.ParenExpr:
		return simple(e.X)
	}
	return false
}

func usage(f fn, v *types.Var) (uses int, mutated bool) {
	ast.Inspect(f.decl.Body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.Ident:
			if f.pkg.TypesInfo.Uses[n] == v {
				uses++
			}
		case *ast.AssignStmt:
			for _, l := range n.Lhs {
				if refersTo(f, l, v) {
					mutated = true
				}
			}
		case *ast.IncDecStmt:
			mutated = mutated || refersTo(f, n.X, v)
		case *ast.UnaryExpr:
			mutated = mutated || n.Op == token.AND && refersTo(f, n.X, v)
		case *ast.RangeStmt:
			mutated = mutated || refersTo(f, n.Key, v) || refersTo(f, n.Value, v)
		}
		return true
	})
	return uses, mutated
}

func refersTo(f fn, e ast.Expr, v *types.Var) bool {
	for e != nil {
		switch x := e.(type) {
		case *ast.Ident:
			return f.pkg.TypesInfo.Uses[x] == v
		case *ast.ParenExpr:
			e = x.X
		case *ast.StarExpr:
			e = x.X
		case *ast.SelectorExpr:
			e = x.X
		case *ast.IndexExpr:
			e = x.X
		default:
			return false
		}
	}
	return false
}

// qualifier prints types the way the target file names their packages, falling back to the
// package name so the import fixer can add a missing import.
func qualifier(u unit) types.Qualifier {
	aliases := map[string]string{}
	for _, imp := range u.file.Imports {
		if imp.Name != nil {
			aliases[strings.Trim(imp.Path.Value, `"`)] = imp.Name.Name
		}
	}
	return func(p *types.Package) string {
		if p == u.pkg.Types {
			return ""
		}
		if a, ok := aliases[p.Path()]; ok {
			return a
		}
		return p.Name()
	}
}

// renameLocals prints n from the callee's source with every callee-local identifier
// suffixed, so inlined names cannot collide with the parent's; substituted parameters
// are replaced by their argument text.
func renameLocals(callee fn, n ast.Node, suffix string, subst map[types.Object]string) string {
	start := callee.offset(n.Pos())
	var ss []splice
	ast.Inspect(n, func(x ast.Node) bool {
		id, ok := x.(*ast.Ident)
		if !ok || id.Name == "_" {
			return true
		}
		obj := callee.pkg.TypesInfo.Defs[id]
		if obj == nil {
			obj = callee.pkg.TypesInfo.Uses[id]
		}
		if obj == nil || !local(callee, obj) {
			return true
		}
		if _, isLabel := obj.(*types.Label); isLabel {
			return true
		}
		off := callee.offset(id.Pos()) - start
		text := id.Name + suffix
		if r, ok := subst[obj]; ok {
			text = r
		}
		ss = append(ss, splice{off, off + len(id.Name), text})
		return true
	})
	text := callee.src[start:callee.offset(n.End())]
	return string(applySplices(text, ss))
}
