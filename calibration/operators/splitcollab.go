package operators

import (
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"github.com/KellyBennett/Columbo/calibration/harness"
)

// splitByCollaborators is Extract Function along a collaborator seam: it moves one
// contiguous run of top-level statements into a helper when the run and the rest of the
// body each stay within the dependencies limit. It only extracts runs whose data flow it
// can carry through parameters and results without changing behavior.
type splitByCollaborators struct{}

func (splitByCollaborators) ID() string           { return "split-by-collaborators" }
func (splitByCollaborators) Class() harness.Class { return harness.Cure }
func (splitByCollaborators) Target() string       { return "excessive-dependencies" }
func (splitByCollaborators) Doses(*harness.Corpus, harness.Site) []int {
	return []int{1}
}

// Sites lists every baseline case, so a case with no honest seam is logged with its
// reason instead of disappearing from the run.
func (o splitByCollaborators) Sites(c *harness.Corpus) []harness.Site {
	var out []harness.Site
	for _, f := range funcs(c) {
		if f.baselineHas(c, o.Target(), f.symbol) {
			out = append(out, f.site())
		}
	}
	return out
}

type seam struct {
	from, to       int
	params, ins    []*types.Var
	outs           []*types.Var
	declares       bool
	parent, helper int
}

func (o splitByCollaborators) Apply(c *harness.Corpus, site harness.Site, _ int) (harness.Applied, error) {
	f, ok := findFn(c, site.Symbol)
	if !ok {
		return harness.Applied{}, errors.New("site vanished")
	}
	s, err := bestSeam(c, f)
	if err != nil {
		return harness.Applied{}, err
	}
	qual := qualifier(f.unit)
	name := freeName(f, lowerFirst(f.decl.Name.Name)+"Part")
	recv, call := "", name
	if f.decl.Recv != nil && usesReceiver(f, s) {
		recv = "(" + f.text(f.decl.Recv.List[0]) + ") "
		call = f.decl.Recv.List[0].Names[0].Name + "." + name
	}
	var params, args, results, outs []string
	for _, v := range append(append([]*types.Var{}, s.params...), s.ins...) {
		params = append(params, v.Name()+" "+types.TypeString(v.Type(), qual))
		args = append(args, v.Name())
	}
	for _, v := range s.outs {
		results = append(results, types.TypeString(v.Type(), qual))
		outs = append(outs, v.Name())
	}
	stmts := f.decl.Body.List
	text := string(f.src[f.offset(stmts[s.from].Pos()):f.offset(stmts[s.to-1].End())])
	invoke := fmt.Sprintf("%s(%s)", call, strings.Join(args, ", "))
	res := ""
	if len(outs) > 0 {
		op := "="
		if s.declares {
			op = ":="
		}
		invoke = fmt.Sprintf("%s %s %s", strings.Join(outs, ", "), op, invoke)
		text += "\nreturn " + strings.Join(outs, ", ")
		res = " (" + strings.Join(results, ", ") + ")"
	}
	helper := fmt.Sprintf("\n\nfunc %s%s(%s)%s {\n%s\n}", recv, name, strings.Join(params, ", "), res, text)
	ss := []splice{
		{f.offset(stmts[s.from].Pos()), f.offset(stmts[s.to-1].End()), invoke},
		{f.offset(f.decl.End()), f.offset(f.decl.End()), helper},
	}
	out, err := finish(f.path, applySplices(f.src, ss))
	if err != nil {
		return harness.Applied{}, err
	}
	helperRecv := f.decl.Recv
	if recv == "" {
		helperRecv = nil
	}
	limit := int(threshold(c, "dependencies", 5))
	kill := s.parent <= limit && s.helper <= limit
	return harness.Applied{
		Edit: harness.Edit{f.rel: out},
		Expectation: harness.Expectation{
			Touched: []string{f.symbol, symbolOf(f.pkg.PkgPath, &ast.FuncDecl{Recv: helperRecv, Name: ast.NewIdent(name)})},
			Prediction: harness.Prediction{Kill: &kill, Reason: fmt.Sprintf(
				"estimated dependencies: parent %d, helper %d (limit %d), statements %d-%d of %d",
				s.parent, s.helper, limit, s.from+1, s.to, len(stmts))},
		},
	}, nil
}

func usesReceiver(f fn, s seam) bool {
	if f.decl.Recv == nil {
		return false
	}
	rv := recvVar(f)
	used := false
	for _, st := range f.decl.Body.List[s.from:s.to] {
		ast.Inspect(st, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && rv != nil && f.pkg.TypesInfo.Uses[id] == rv {
				used = true
			}
			return !used
		})
	}
	return used
}

// bestSeam picks the extractable run that minimizes the larger of the two estimated
// dependency counts, keeping the helper within the parameter limit.
func bestSeam(c *harness.Corpus, f fn) (seam, error) {
	if err := splittable(f); err != nil {
		return seam{}, err
	}
	stmts := f.decl.Body.List
	limit := int(threshold(c, "dependencies", 5))
	maxParams := int(threshold(c, "parameters", 4))
	est := newDepEstimator(f)
	sigDeps := est.ofType(f.obj.Type().(*types.Signature).Params(), f.obj.Type().(*types.Signature).Results())
	stmtDeps := make([]depSet, len(stmts))
	for i, s := range stmts {
		stmtDeps[i] = est.ofNode(s)
	}
	split := func(from, to int, edge depSet) (parent, helper int) {
		p, h := depSet{}, depSet{}
		p.add(sigDeps, edge)
		h.add(edge)
		for i, d := range stmtDeps {
			if i >= from && i < to {
				h.add(d)
			} else {
				p.add(d)
			}
		}
		return p.count(), h.count()
	}
	var best, closest *seam
	runs, extractable, wide, partition := 0, 0, 0, -1
	for from := 0; from < len(stmts); from++ {
		for to := from + 1; to <= len(stmts); to++ {
			if from == 0 && to == len(stmts) {
				continue
			}
			runs++
			if p, h := split(from, to, depSet{}); partition < 0 || max(p, h) < partition {
				partition = max(p, h)
			}
			s, err := seamAt(f, from, to)
			if err != nil {
				continue
			}
			extractable++
			if len(s.params)+len(s.ins) > maxParams {
				wide++
				continue
			}
			var boundary []types.Type
			for _, v := range append(append(append([]*types.Var{}, s.params...), s.ins...), s.outs...) {
				boundary = append(boundary, v.Type())
			}
			s.parent, s.helper = split(from, to, est.ofTypes(boundary...))
			if closest == nil || max(s.parent, s.helper) < max(closest.parent, closest.helper) {
				closest = &s
			}
			if s.parent <= limit && s.helper <= limit && (best == nil || max(s.parent, s.helper) < max(best.parent, best.helper)) {
				best = &s
			}
		}
	}
	if best != nil {
		return *best, nil
	}
	reason := fmt.Sprintf("no seam within limit %d: statement partition alone leaves at least %d; %d of %d runs extractable, %d over the parameter limit",
		limit, partition, extractable, runs, wide)
	if closest != nil {
		reason += fmt.Sprintf("; closest extractable run leaves parent %d, helper %d", closest.parent, closest.helper)
	}
	return seam{}, errors.New(reason)
}

// splittable rejects bodies whose statements cannot be moved without changing meaning:
// local types or constants a helper could not name, labels, and goto.
func splittable(f fn) error {
	d := f.decl
	if d.Body == nil || f.obj == nil || d.Type.TypeParams != nil || len(d.Body.List) < 2 {
		return errors.New("no body to split")
	}
	if f.decl.Recv != nil && (len(f.decl.Recv.List[0].Names) == 0 || f.decl.Recv.List[0].Names[0].Name == "_") {
		return errors.New("unnamed receiver")
	}
	var err error
	ast.Inspect(d.Body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.DeclStmt:
			if gd := n.Decl.(*ast.GenDecl); gd.Tok == token.TYPE || gd.Tok == token.CONST {
				err = errors.New("local type or constant")
			}
		case *ast.LabeledStmt:
			err = errors.New("labels")
		case *ast.BranchStmt:
			if n.Tok == token.GOTO {
				err = errors.New("goto")
			}
		}
		return err == nil
	})
	return err
}

// seamAt checks that statements [from, to) can become a helper and returns its data flow.
func seamAt(f fn, from, to int) (seam, error) {
	stmts := f.decl.Body.List
	for _, s := range stmts[from:to] {
		if escapes(f, s) {
			return seam{}, errors.New("control flow or closure escapes the run")
		}
	}
	flows := chunkFlows(f, stmts, []int{0, from, to, len(stmts)})
	fl := flows[1]
	for _, v := range append(append([]*types.Var{}, fl.in...), fl.out...) {
		if addressed(f, v) || capturedOutside(f, v, from, to) {
			return seam{}, errors.New("aliased local crosses the run")
		}
	}
	params, err := paramsUsed(f, stmts[from:to])
	if err != nil {
		return seam{}, err
	}
	return seam{from: from, to: to, params: params, ins: fl.in, outs: fl.out, declares: fl.declares}, nil
}

// escapes reports a statement whose meaning depends on staying in the parent: a return,
// defer, recover, or a closure that may outlive the run.
func escapes(f fn, s ast.Stmt) bool {
	bad := false
	ast.Inspect(s, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.ReturnStmt, *ast.DeferStmt:
			bad = true
		case *ast.CallExpr:
			if id, ok := n.Fun.(*ast.Ident); ok && id.Name == "recover" {
				bad = true
			}
		case *ast.FuncLit:
			return false
		}
		return !bad
	})
	if bad {
		return true
	}
	return storesClosure(f, s)
}

// storesClosure reports a function literal in s that is not passed directly as a call
// argument, since such a closure may run after the helper has returned.
func storesClosure(f fn, s ast.Stmt) bool {
	direct := map[*ast.FuncLit]bool{}
	ast.Inspect(s, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			for _, a := range call.Args {
				if lit, ok := a.(*ast.FuncLit); ok {
					direct[lit] = true
				}
			}
		}
		return true
	})
	stored := false
	ast.Inspect(s, func(n ast.Node) bool {
		if lit, ok := n.(*ast.FuncLit); ok {
			if !direct[lit] && capturesLocal(f, lit) {
				stored = true
			}
			return false
		}
		return !stored
	})
	return stored
}

func capturesLocal(f fn, lit *ast.FuncLit) bool {
	captured := false
	ast.Inspect(lit.Body, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok {
			if v, ok := f.pkg.TypesInfo.Uses[id].(*types.Var); ok && local(f, v) && v.Pos() < lit.Pos() {
				captured = true
			}
		}
		return !captured
	})
	return captured
}

func addressed(f fn, v *types.Var) bool {
	found := false
	ast.Inspect(f.decl.Body, func(n ast.Node) bool {
		if u, ok := n.(*ast.UnaryExpr); ok && u.Op == token.AND && refersTo(f, u.X, v) {
			found = true
		}
		return !found
	})
	return found
}

// capturedOutside reports v referenced by a function literal outside the run, which would
// observe the parent's copy rather than the helper's.
func capturedOutside(f fn, v *types.Var, from, to int) bool {
	found := false
	for i, s := range f.decl.Body.List {
		if i >= from && i < to {
			continue
		}
		ast.Inspect(s, func(n ast.Node) bool {
			lit, ok := n.(*ast.FuncLit)
			if !ok {
				return !found
			}
			ast.Inspect(lit.Body, func(m ast.Node) bool {
				if id, ok := m.(*ast.Ident); ok && f.pkg.TypesInfo.Uses[id] == v {
					found = true
				}
				return !found
			})
			return false
		})
	}
	return found
}

// paramsUsed lists the parameters the run reads, refusing a run that assigns to one.
func paramsUsed(f fn, run []ast.Stmt) ([]*types.Var, error) {
	sig := f.obj.Type().(*types.Signature)
	var out []*types.Var
	for i := 0; i < sig.Params().Len(); i++ {
		v := sig.Params().At(i)
		used, mutated := false, false
		for _, s := range run {
			ast.Inspect(s, func(n ast.Node) bool {
				switch n := n.(type) {
				case *ast.Ident:
					used = used || f.pkg.TypesInfo.Uses[n] == v
				case *ast.AssignStmt:
					for _, l := range n.Lhs {
						mutated = mutated || refersTo(f, l, v)
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
		}
		if mutated {
			return nil, errors.New("run assigns a parameter")
		}
		if used {
			if v.Name() == "" || v.Name() == "_" {
				return nil, errors.New("unnamed parameter")
			}
			out = append(out, v)
		}
	}
	if sig.Variadic() && len(out) > 0 && out[len(out)-1] == sig.Params().At(sig.Params().Len()-1) {
		return nil, errors.New("variadic parameter")
	}
	return out, nil
}

// depSet approximates Columbo's scored dependency set: foreign named types, nonempty
// unnamed interfaces, and foreign package references, with a package dropped once one of
// its named types is present.
type depSet map[string]string

func (d depSet) add(others ...depSet) {
	for _, o := range others {
		for k, v := range o {
			d[k] = v
		}
	}
}

func (d depSet) count() int {
	owned := map[string]bool{}
	for k, owner := range d {
		if strings.HasPrefix(k, "type:") {
			owned[owner] = true
		}
	}
	n := 0
	for k, owner := range d {
		if strings.HasPrefix(k, "package:") && owned[owner] {
			continue
		}
		n++
	}
	return n
}

type depEstimator struct {
	f    fn
	recv *types.TypeName
}

func newDepEstimator(f fn) depEstimator {
	e := depEstimator{f: f}
	if sig := f.obj.Type().(*types.Signature); sig.Recv() != nil {
		t := sig.Recv().Type()
		if p, ok := t.(*types.Pointer); ok {
			t = p.Elem()
		}
		if n, ok := types.Unalias(t).(*types.Named); ok {
			e.recv = n.Origin().Obj()
		}
	}
	return e
}

func (e depEstimator) ofNode(n ast.Node) depSet {
	out := depSet{}
	info := e.f.pkg.TypesInfo
	ast.Inspect(n, func(x ast.Node) bool {
		switch x := x.(type) {
		case ast.Expr:
			if tv, ok := info.Types[x]; ok && tv.Type != nil {
				e.collect(tv.Type, out, map[types.Type]bool{})
			}
			if id, ok := x.(*ast.Ident); ok {
				obj := info.Uses[id]
				if obj != nil && obj.Pkg() != nil && obj.Pkg() != e.f.pkg.Types {
					if _, isPkg := obj.(*types.PkgName); !isPkg {
						out["package:"+obj.Pkg().Path()] = obj.Pkg().Path()
					}
				}
			}
		}
		return true
	})
	return out
}

func (e depEstimator) ofType(tuples ...*types.Tuple) depSet {
	var ts []types.Type
	for _, t := range tuples {
		for i := 0; i < t.Len(); i++ {
			ts = append(ts, t.At(i).Type())
		}
	}
	return e.ofTypes(ts...)
}

func (e depEstimator) ofTypes(ts ...types.Type) depSet {
	out := depSet{}
	for _, t := range ts {
		e.collect(t, out, map[types.Type]bool{})
	}
	return out
}

func (e depEstimator) collect(t types.Type, out depSet, seen map[types.Type]bool) {
	if seen[t] {
		return
	}
	seen[t] = true
	switch t := types.Unalias(t).(type) {
	case *types.Pointer:
		e.collect(t.Elem(), out, seen)
	case *types.Slice:
		e.collect(t.Elem(), out, seen)
	case *types.Array:
		e.collect(t.Elem(), out, seen)
	case *types.Chan:
		e.collect(t.Elem(), out, seen)
	case *types.Map:
		e.collect(t.Key(), out, seen)
		e.collect(t.Elem(), out, seen)
	case *types.Signature:
		for _, tup := range []*types.Tuple{t.Params(), t.Results()} {
			for i := 0; i < tup.Len(); i++ {
				e.collect(tup.At(i).Type(), out, seen)
			}
		}
	case *types.Tuple:
		for i := 0; i < t.Len(); i++ {
			e.collect(t.At(i).Type(), out, seen)
		}
	case *types.Struct:
		for i := 0; i < t.NumFields(); i++ {
			e.collect(t.Field(i).Type(), out, seen)
		}
	case *types.Interface:
		if !t.Empty() {
			out["interface:"+t.String()] = ""
		}
	case *types.Named:
		obj := t.Origin().Obj()
		if obj.Pkg() == nil || obj == e.recv || (obj.Pkg() == e.f.pkg.Types && !obj.Exported()) {
			return
		}
		out["type:"+types.TypeString(t, nil)] = obj.Pkg().Path()
		for i := 0; i < t.TypeArgs().Len(); i++ {
			e.collect(t.TypeArgs().At(i), out, seen)
		}
	}
}
