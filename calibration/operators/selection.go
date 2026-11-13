package operators

import (
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"sort"
	"strings"

	"github.com/KellyBennett/Columbo/calibration/harness"
)

const selectionSmell = "selection-use-coupling"

// roleInterface reports whether t is an interface with at least one method, the only kind
// of role selection-use-coupling recognizes.
func roleInterface(t types.Type) bool {
	i, ok := types.Unalias(t).Underlying().(*types.Interface)
	return ok && i.NumMethods() > 0
}

// selection is one conditional statement in a function that assigns concrete
// implementations to a local of a role type, the shape SUC-001 calls a selection.
type selection struct {
	stmt    ast.Stmt
	v       *types.Var
	assigns []*ast.AssignStmt
	impls   map[string]bool
}

// selections finds the outermost if-chains and expression switches in f that assign
// concrete values to role-typed locals. Function literals are analysis boundaries.
func selections(f fn) []selection {
	var out []selection
	info := f.pkg.TypesInfo
	ast.Inspect(f.decl.Body, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.IfStmt, *ast.SwitchStmt:
		default:
			return true
		}
		byVar := map[*types.Var]*selection{}
		var order []*types.Var
		ast.Inspect(n, func(m ast.Node) bool {
			if _, lit := m.(*ast.FuncLit); lit {
				return false
			}
			a, ok := m.(*ast.AssignStmt)
			if !ok || len(a.Lhs) != 1 || len(a.Rhs) != 1 || a.Tok != token.ASSIGN {
				return true
			}
			id, ok := a.Lhs[0].(*ast.Ident)
			if !ok {
				return true
			}
			v, ok := info.Uses[id].(*types.Var)
			rt := info.TypeOf(a.Rhs[0])
			if !ok || !local(f, v) || !roleInterface(v.Type()) || rt == nil || !concreteNamed(rt) {
				return true
			}
			s := byVar[v]
			if s == nil {
				s = &selection{stmt: n.(ast.Stmt), v: v, impls: map[string]bool{}}
				byVar[v] = s
				order = append(order, v)
			}
			s.assigns = append(s.assigns, a)
			s.impls[implIdentity(rt)] = true
			return true
		})
		for _, v := range order {
			out = append(out, *byVar[v])
		}
		return len(order) == 0
	})
	return out
}

func implIdentity(t types.Type) string {
	if p, ok := types.Unalias(t).(*types.Pointer); ok {
		t = p.Elem()
	}
	return types.TypeString(t, nil)
}

// usedAsRole reports whether v receives an interface method call after pos outside any
// function literal, and whether it ever flows to a return, which exempts it.
func usedAsRole(f fn, v *types.Var, pos token.Pos) (called, returned bool) {
	info := f.pkg.TypesInfo
	ast.Inspect(f.decl.Body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.ReturnStmt:
			for _, r := range n.Results {
				if id, ok := ast.Unparen(r).(*ast.Ident); ok && info.Uses[id] == v {
					returned = true
				}
			}
		case *ast.CallExpr:
			sel, ok := n.Fun.(*ast.SelectorExpr)
			if !ok || n.Pos() < pos {
				return true
			}
			if id, ok := ast.Unparen(sel.X).(*ast.Ident); ok && info.Uses[id] == v {
				if s := info.Selections[sel]; s != nil && s.Kind() == types.MethodVal {
					called = true
				}
			}
		}
		return true
	})
	return called, returned
}

func eligibleUser(f fn) bool {
	d := f.decl
	if d.Body == nil || f.obj == nil || d.Type.TypeParams != nil || (d.Recv == nil && (d.Name.Name == "init" || d.Name.Name == "main" && f.pkg.Name == "main")) {
		return false
	}
	ok := true
	ast.Inspect(d.Body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.LabeledStmt:
			ok = false
		case *ast.BranchStmt:
			ok = ok && n.Tok != token.GOTO && n.Label == nil
		}
		return ok
	})
	return ok
}

// selectThenUse inlines a selecting factory into a caller that then sends the result a
// message, which is the inverse of Extract Factory. A factory qualifies when its body is
// only a decision whose every path returns one expression.
type selectThenUse struct{}

func (selectThenUse) ID() string           { return "select-then-use" }
func (selectThenUse) Class() harness.Class { return harness.Inject }
func (selectThenUse) Target() string       { return selectionSmell }

type factoryCall struct {
	user    fn
	stmt    *ast.AssignStmt
	call    *ast.CallExpr
	factory fn
	plan    factoryShape
	v       *types.Var
}

type factoryShape struct {
	// tag is the switch tag text; empty for an if-chain.
	tag      string
	isSwitch bool
	heads    []string // case lists or conditions
	results  []ast.Expr
	fallback ast.Expr
}

func (selectThenUse) Sites(c *harness.Corpus) []harness.Site {
	var out []harness.Site
	for _, u := range funcs(c) {
		if !eligibleUser(u) || u.baselineHas(c, selectionSmell, u.symbol) {
			continue
		}
		for _, fc := range factoryCalls(c, u) {
			site := u.site()
			site.Offset = u.offset(fc.call.Pos())
			out = append(out, site)
		}
	}
	return out
}

func (selectThenUse) Doses(*harness.Corpus, harness.Site) []int { return []int{1} }

func factoryCalls(c *harness.Corpus, u fn) []factoryCall {
	var out []factoryCall
	info := u.pkg.TypesInfo
	ast.Inspect(u.decl.Body, func(n ast.Node) bool {
		if _, lit := n.(*ast.FuncLit); lit {
			return false
		}
		a, ok := n.(*ast.AssignStmt)
		if !ok || len(a.Lhs) != 1 || len(a.Rhs) != 1 {
			return true
		}
		id, ok := a.Lhs[0].(*ast.Ident)
		call, isCall := a.Rhs[0].(*ast.CallExpr)
		if !ok || !isCall || id.Name == "_" || call.Ellipsis.IsValid() {
			return true
		}
		if _, plain := call.Fun.(*ast.Ident); !plain {
			return true
		}
		v, _ := info.ObjectOf(id).(*types.Var)
		factory, ok := resolveCallee(c, u, call)
		if v == nil || !ok || factory.symbol == u.symbol || !roleInterface(v.Type()) {
			return true
		}
		shape, ok := selectingFactory(factory)
		if !ok || len(call.Args) != factory.obj.Type().(*types.Signature).Params().Len() || shadows(u, call, factory) {
			return true
		}
		for _, p := range factory.decl.Type.Params.List {
			for _, name := range p.Names {
				if name.Name == id.Name {
					return true
				}
			}
		}
		if called, returned := usedAsRole(u, v, a.End()); called && !returned {
			out = append(out, factoryCall{user: u, stmt: a, call: call, factory: factory, plan: shape, v: v})
		}
		return true
	})
	return out
}

// selectingFactory recognizes `switch t { case X: return a; ... }` or a run of
// `if cond { return a }`, closed by a final return or a default, and requires at least
// two distinct concrete results.
func selectingFactory(f fn) (factoryShape, bool) {
	d := f.decl
	if d.Body == nil || f.obj == nil || d.Recv != nil || d.Type.TypeParams != nil || namedResults(d) {
		return factoryShape{}, false
	}
	sig := f.obj.Type().(*types.Signature)
	if sig.Variadic() || sig.Results().Len() != 1 || !roleInterface(sig.Results().At(0).Type()) {
		return factoryShape{}, false
	}
	var shape factoryShape
	list := d.Body.List
	if sw, ok := list[0].(*ast.SwitchStmt); ok && len(list) <= 2 && sw.Init == nil {
		shape.isSwitch = true
		if sw.Tag != nil {
			shape.tag = f.text(sw.Tag)
		}
		for _, st := range sw.Body.List {
			cc := st.(*ast.CaseClause)
			r, ok := singleReturn(cc.Body)
			if !ok {
				return factoryShape{}, false
			}
			if cc.List == nil {
				if shape.fallback != nil || len(list) == 2 {
					return factoryShape{}, false
				}
				shape.fallback = r
				continue
			}
			var heads []string
			for _, e := range cc.List {
				heads = append(heads, f.text(e))
			}
			shape.heads = append(shape.heads, strings.Join(heads, ", "))
			shape.results = append(shape.results, r)
		}
		if len(list) == 2 {
			r, ok := singleReturn(list[1:])
			if !ok {
				return factoryShape{}, false
			}
			shape.fallback = r
		}
	} else {
		for i, st := range list {
			if i == len(list)-1 {
				r, ok := singleReturn(list[i:])
				if !ok {
					return factoryShape{}, false
				}
				shape.fallback = r
				break
			}
			is, ok := st.(*ast.IfStmt)
			if !ok || is.Init != nil || is.Else != nil {
				return factoryShape{}, false
			}
			r, ok := singleReturn(is.Body.List)
			if !ok {
				return factoryShape{}, false
			}
			shape.heads = append(shape.heads, f.text(is.Cond))
			shape.results = append(shape.results, r)
		}
		if len(shape.heads) == 0 {
			return factoryShape{}, false
		}
	}
	if impls := factoryImpls(f, shape); len(impls) < 2 {
		return factoryShape{}, false
	}
	return shape, true
}

func singleReturn(list []ast.Stmt) (ast.Expr, bool) {
	if len(list) != 1 {
		return nil, false
	}
	r, ok := list[0].(*ast.ReturnStmt)
	if !ok || len(r.Results) != 1 {
		return nil, false
	}
	return r.Results[0], true
}

func factoryImpls(f fn, shape factoryShape) map[string]bool {
	impls := map[string]bool{}
	for _, r := range append(append([]ast.Expr{}, shape.results...), shape.fallback) {
		if r == nil {
			continue
		}
		if t := f.pkg.TypesInfo.TypeOf(r); t != nil && concreteNamed(t) {
			impls[implIdentity(t)] = true
		}
	}
	return impls
}

// opaque reports a result that is neither nil nor concrete: an interface-typed value,
// which SUC-001 treats as an unknown origin that can hide the alternatives.
func opaque(f fn, shape factoryShape) bool {
	for _, r := range append(append([]ast.Expr{}, shape.results...), shape.fallback) {
		if r == nil {
			continue
		}
		t := f.pkg.TypesInfo.TypeOf(r)
		if b, ok := t.(*types.Basic); ok && b.Kind() == types.UntypedNil {
			continue
		}
		if !concreteNamed(t) {
			return true
		}
	}
	return false
}

func (selectThenUse) Apply(c *harness.Corpus, site harness.Site, _ int) (harness.Applied, error) {
	u, ok := findFn(c, site.Symbol)
	if !ok {
		return harness.Applied{}, errors.New("site vanished")
	}
	var fc *factoryCall
	for _, cand := range factoryCalls(c, u) {
		if u.offset(cand.call.Pos()) == site.Offset {
			fc = &cand
			break
		}
	}
	if fc == nil {
		return harness.Applied{}, errors.New("call vanished")
	}
	fac, shape := fc.factory, fc.plan
	name := fc.v.Name()
	var b strings.Builder
	if fc.stmt.Tok == token.DEFINE {
		fmt.Fprintf(&b, "var %s %s\n", name, types.TypeString(fc.v.Type(), qualifier(u.unit)))
	}
	b.WriteString("{\n")
	if binding := bindParams(fac, fc.call, u); binding != "" {
		b.WriteString(binding + "\n")
	}
	assign := func(e ast.Expr) string { return name + " = " + fac.text(e) + "\n" }
	if shape.isSwitch {
		fmt.Fprintf(&b, "switch %s {\n", shape.tag)
		for i, h := range shape.heads {
			fmt.Fprintf(&b, "case %s:\n%s", h, assign(shape.results[i]))
		}
		if shape.fallback != nil {
			fmt.Fprintf(&b, "default:\n%s", assign(shape.fallback))
		}
		b.WriteString("}\n")
	} else {
		for i, h := range shape.heads {
			if i > 0 {
				b.WriteString(" else ")
			}
			fmt.Fprintf(&b, "if %s {\n%s}", h, assign(shape.results[i]))
		}
		fmt.Fprintf(&b, " else {\n%s}\n", assign(shape.fallback))
	}
	b.WriteString("}")
	out, err := finish(u.path, applySplices(u.src, []splice{{u.offset(fc.stmt.Pos()), u.offset(fc.stmt.End()), b.String()}}))
	if err != nil {
		return harness.Applied{}, err
	}
	impls := factoryImpls(fac, shape)
	limit := int(threshold(c, "selection-use-implementations", 2))
	reason := fmt.Sprintf("%d concrete implementations selected (limit %d)", len(impls), limit)
	pred := harness.Prediction{Reason: reason}
	if opaque(fac, shape) {
		pred.Reason += "; an interface-typed result may hide the alternatives"
	} else {
		kill := len(impls) >= limit
		pred.Kill = &kill
	}
	return harness.Applied{
		Edit:        harness.Edit{u.rel: out},
		Expectation: harness.Expectation{Touched: []string{u.symbol}, Prediction: pred},
	}, nil
}

// bindParams evaluates the call's arguments once, in order, into the factory's parameter
// names, discarding the ones the body never reads and skipping arguments that already
// carry the parameter's name.
func bindParams(fac fn, call *ast.CallExpr, u fn) string {
	sig := fac.obj.Type().(*types.Signature)
	if sig.Params().Len() == 0 {
		return ""
	}
	var names, args []string
	anyUsed := false
	for i := 0; i < sig.Params().Len(); i++ {
		p := sig.Params().At(i)
		arg := u.text(call.Args[i])
		name := "_"
		if uses, _ := usage(fac, p); uses > 0 && p.Name() != "_" {
			if arg == p.Name() {
				continue
			}
			name, anyUsed = p.Name(), true
		}
		names = append(names, name)
		args = append(args, arg)
	}
	if len(names) == 0 {
		return ""
	}
	op := ":="
	if !anyUsed {
		op = "="
	}
	return strings.Join(names, ", ") + " " + op + " " + strings.Join(args, ", ")
}

// launderSelection is the cosmetic evade: every concrete assignment in the selection is
// passed through an identity helper typed as the role. The caller still chooses each
// concrete player; the rule only stops seeing their types.
type launderSelection struct{}

func (launderSelection) ID() string           { return "launder-selection-through-role" }
func (launderSelection) Class() harness.Class { return harness.Evade }
func (launderSelection) Target() string       { return selectionSmell }
func (launderSelection) Follows() string      { return "select-then-use" }

func (launderSelection) Sites(c *harness.Corpus) []harness.Site {
	var out []harness.Site
	for _, f := range funcs(c) {
		if f.decl.Body != nil && len(selections(f)) > 0 {
			out = append(out, f.site())
		}
	}
	return out
}

func (launderSelection) Doses(*harness.Corpus, harness.Site) []int { return []int{1} }

func (launderSelection) Apply(c *harness.Corpus, site harness.Site, _ int) (harness.Applied, error) {
	f, ok := findFn(c, site.Symbol)
	if !ok {
		return harness.Applied{}, errors.New("site vanished")
	}
	qual := qualifier(f.unit)
	helpers := map[string]string{}
	var roles []string
	var ss []splice
	for _, s := range selections(f) {
		role := types.TypeString(s.v.Type(), qual)
		if _, ok := helpers[role]; !ok {
			base := "asRole"
			if named, isNamed := types.Unalias(s.v.Type()).(*types.Named); isNamed {
				base = "as" + named.Obj().Name()
			}
			name := uniqueFuncName(f, base)
			for _, other := range helpers {
				if other == name {
					name = uniqueFuncName(f, base+"Role")
				}
			}
			helpers[role] = name
			roles = append(roles, role)
		}
		for _, a := range s.assigns {
			rhs := a.Rhs[0]
			ss = append(ss, splice{f.offset(rhs.Pos()), f.offset(rhs.End()), helpers[role] + "(" + f.text(rhs) + ")"})
		}
	}
	var decls strings.Builder
	var touched = []string{f.symbol}
	for _, role := range roles {
		fmt.Fprintf(&decls, "\n\nfunc %s(r %s) %s {\nreturn r\n}", helpers[role], role, role)
		touched = append(touched, symbolOf(f.pkg.PkgPath, &ast.FuncDecl{Name: ast.NewIdent(helpers[role])}))
	}
	end := f.offset(f.decl.End())
	ss = append(ss, splice{end, end, decls.String()})
	out, err := finish(f.path, applySplices(f.src, ss))
	if err != nil {
		return harness.Applied{}, err
	}
	return harness.Applied{
		Edit:        harness.Edit{f.rel: out},
		Expectation: harness.Expectation{Touched: touched, Catches: []string{selectionSmell}},
	}, nil
}

// extractFactory is Extract Function aimed at the selection: each selection statement
// moves into a helper that takes the role's current value and returns the role, so the
// caller keeps only collaboration.
type extractFactory struct{}

func (extractFactory) ID() string           { return "extract-factory" }
func (extractFactory) Class() harness.Class { return harness.Cure }
func (extractFactory) Target() string       { return selectionSmell }

func (extractFactory) Sites(c *harness.Corpus) []harness.Site {
	var out []harness.Site
	for _, f := range funcs(c) {
		if !f.baselineHas(c, selectionSmell, f.symbol) || !eligibleUser(f) {
			continue
		}
		sels := selections(f)
		ok := len(sels) > 0
		for _, s := range sels {
			if _, err := factoryPlan(f, s); err != nil {
				ok = false
			}
		}
		if ok && !sharedStatements(sels) {
			out = append(out, f.site())
		}
	}
	return out
}

// sharedStatements rejects one statement selecting two roles; extracting it twice would
// need a helper with two results.
func sharedStatements(sels []selection) bool {
	seen := map[ast.Stmt]bool{}
	for _, s := range sels {
		if seen[s.stmt] {
			return true
		}
		seen[s.stmt] = true
	}
	return false
}

func (extractFactory) Doses(*harness.Corpus, harness.Site) []int { return []int{1} }

type extraction struct {
	free []*types.Var
}

// factoryPlan checks that the selection statement can move into a function unchanged:
// it may not jump out, return, defer, or write any local other than the role variable,
// and every local it reads must be copyable without changing meaning.
func factoryPlan(f fn, s selection) (extraction, error) {
	info := f.pkg.TypesInfo
	start, end := s.stmt.Pos(), s.stmt.End()
	inside := func(p token.Pos) bool { return p >= start && p < end }
	var err error
	fail := func(msg string) { err = errors.New(msg) }
	seen := map[*types.Var]bool{}
	var free []*types.Var
	outerLocal := func(e ast.Expr) bool {
		for {
			switch x := e.(type) {
			case *ast.Ident:
				v, ok := info.Uses[x].(*types.Var)
				return ok && local(f, v) && !inside(v.Pos()) && v != s.v
			case *ast.SelectorExpr:
				e = x.X
			case *ast.IndexExpr:
				e = x.X
			case *ast.StarExpr:
				e = x.X
			case *ast.ParenExpr:
				e = x.X
			default:
				return false
			}
		}
	}
	var visit func(n ast.Node, loops, breaks int) bool
	visit = func(n ast.Node, loops, breaks int) bool {
		if err != nil || n == nil {
			return false
		}
		switch n := n.(type) {
		case *ast.ReturnStmt, *ast.DeferStmt, *ast.LabeledStmt, *ast.GoStmt:
			fail("statement leaves or outlives the selection")
		case *ast.BranchStmt:
			if n.Label != nil || n.Tok == token.GOTO ||
				n.Tok == token.CONTINUE && loops == 0 || n.Tok == token.BREAK && breaks == 0 {
				fail("jump out of the selection")
			}
		case *ast.ForStmt, *ast.RangeStmt:
			if r, ok := n.(*ast.RangeStmt); ok {
				for _, e := range []ast.Expr{r.Key, r.Value} {
					if e != nil && r.Tok == token.ASSIGN && outerLocal(e) {
						fail("writes an outer local")
					}
				}
			}
			ast.Inspect(n, func(m ast.Node) bool {
				if m == n {
					return true
				}
				return visit(m, loops+1, breaks+1)
			})
			return false
		case *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
			ast.Inspect(n, func(m ast.Node) bool {
				if m == n {
					return true
				}
				return visit(m, loops, breaks+1)
			})
			return false
		case *ast.AssignStmt:
			for _, l := range n.Lhs {
				if outerLocal(l) {
					fail("writes an outer local")
				}
			}
		case *ast.IncDecStmt:
			if outerLocal(n.X) {
				fail("writes an outer local")
			}
		case *ast.UnaryExpr:
			if n.Op == token.AND && outerLocal(n.X) {
				fail("takes the address of an outer local")
			}
		case *ast.Ident:
			obj := info.Uses[n]
			if obj == nil || !local(f, obj) || inside(obj.Pos()) {
				return true
			}
			v, isVar := obj.(*types.Var)
			if !isVar {
				fail("uses a local type or constant")
				return false
			}
			if v == s.v || seen[v] {
				return true
			}
			if !copyable(f, v.Type()) {
				fail("reads a local that a copy would change")
				return false
			}
			seen[v] = true
			free = append(free, v)
		}
		return err == nil
	}
	ast.Inspect(s.stmt, func(m ast.Node) bool {
		if m == s.stmt {
			if _, isSwitch := m.(*ast.SwitchStmt); isSwitch {
				ast.Inspect(m, func(k ast.Node) bool {
					if k == m {
						return true
					}
					return visit(k, 0, 1)
				})
				return false
			}
			return true
		}
		return visit(m, 0, 0)
	})
	if err != nil {
		return extraction{}, err
	}
	sortVars(free)
	return extraction{free: free}, nil
}

// copyable accepts types whose copy shares or equals the original: scalars, strings,
// pointers, interfaces, maps, slices, channels, and functions, all nameable outside f.
func copyable(f fn, t types.Type) bool {
	switch u := types.Unalias(t).(type) {
	case *types.Named:
		obj := u.Obj()
		if obj.Pkg() != nil && obj.Pkg() != f.pkg.Types && !obj.Exported() {
			return false
		}
		if obj.Parent() != nil && obj.Pkg() != nil && obj.Parent() != obj.Pkg().Scope() {
			return false
		}
		if u.TypeArgs() != nil {
			return false
		}
		switch u.Underlying().(type) {
		case *types.Struct, *types.Array:
			return false
		}
		return true
	case *types.Basic, *types.Interface, *types.Signature, *types.Chan:
		return true
	case *types.Pointer:
		return nameable(f, u.Elem())
	case *types.Slice:
		return nameable(f, u.Elem())
	case *types.Map:
		return nameable(f, u.Key()) && nameable(f, u.Elem())
	}
	return false
}

func nameable(f fn, t types.Type) bool {
	switch u := types.Unalias(t).(type) {
	case *types.Named:
		obj := u.Obj()
		if obj.Pkg() != nil && obj.Pkg() != f.pkg.Types && !obj.Exported() {
			return false
		}
		return obj.Pkg() == nil || obj.Parent() == obj.Pkg().Scope()
	case *types.Pointer:
		return nameable(f, u.Elem())
	case *types.Slice:
		return nameable(f, u.Elem())
	case *types.Array:
		return nameable(f, u.Elem())
	case *types.Map:
		return nameable(f, u.Key()) && nameable(f, u.Elem())
	case *types.Basic, *types.Interface:
		return true
	}
	return false
}

func (extractFactory) Apply(c *harness.Corpus, site harness.Site, _ int) (harness.Applied, error) {
	f, ok := findFn(c, site.Symbol)
	if !ok {
		return harness.Applied{}, errors.New("site vanished")
	}
	qual := qualifier(f.unit)
	var ss []splice
	var helpers strings.Builder
	touched := []string{f.symbol}
	used := map[string]bool{}
	for _, s := range selections(f) {
		ex, err := factoryPlan(f, s)
		if err != nil {
			return harness.Applied{}, err
		}
		role := types.TypeString(s.v.Type(), qual)
		base := lowerFirst(f.decl.Name.Name) + "Select"
		if named, isNamed := types.Unalias(s.v.Type()).(*types.Named); isNamed {
			base += named.Obj().Name()
		}
		name := uniqueFuncName(f, base)
		for n := 2; used[name]; n++ {
			name = uniqueFuncName(f, fmt.Sprintf("%s%d", base, n))
		}
		used[name] = true
		params := []string{s.v.Name() + " " + role}
		args := []string{s.v.Name()}
		for _, v := range ex.free {
			params = append(params, v.Name()+" "+types.TypeString(v.Type(), qual))
			args = append(args, v.Name())
		}
		ss = append(ss, splice{f.offset(s.stmt.Pos()), f.offset(s.stmt.End()),
			fmt.Sprintf("%s = %s(%s)", s.v.Name(), name, strings.Join(args, ", "))})
		fmt.Fprintf(&helpers, "\n\nfunc %s(%s) %s {\n%s\nreturn %s\n}", name, strings.Join(params, ", "), role, f.text(s.stmt), s.v.Name())
		touched = append(touched, symbolOf(f.pkg.PkgPath, &ast.FuncDecl{Name: ast.NewIdent(name)}))
	}
	end := f.offset(f.decl.End())
	ss = append(ss, splice{end, end, helpers.String()})
	sort.Slice(ss, func(i, j int) bool { return ss[i].start < ss[j].start })
	out, err := finish(f.path, applySplices(f.src, ss))
	if err != nil {
		return harness.Applied{}, err
	}
	kill := true
	return harness.Applied{
		Edit: harness.Edit{f.rel: out},
		Expectation: harness.Expectation{
			Touched:    touched,
			Prediction: harness.Prediction{Kill: &kill, Reason: "every selection moved behind a helper returning the role"},
		},
	}, nil
}
