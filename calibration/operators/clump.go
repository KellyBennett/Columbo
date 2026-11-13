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

type inlineParameterObject struct {
	id, target string
}

const maxClumpDose = 6

func (o inlineParameterObject) ID() string           { return o.id }
func (o inlineParameterObject) Class() harness.Class { return harness.Inject }
func (o inlineParameterObject) Target() string       { return o.target }

func (o inlineParameterObject) Sites(c *harness.Corpus) []harness.Site {
	p, err := programOf(c)
	if err != nil {
		return nil
	}
	var out []harness.Site
	for _, st := range paramStructs(p, int(threshold(c, "data-clump-size", 3))) {
		if len(o.doses(c, p, st)) == 0 {
			continue
		}
		if o.target == "data-clump" && support(p, st.pkgPath(), st.fieldTypes(), nil).count >= int(threshold(c, "data-clump-occurrences", 3)) {
			continue
		}
		out = append(out, st.site())
	}
	return out
}

func (o inlineParameterObject) Doses(c *harness.Corpus, site harness.Site) []int {
	p, err := programOf(c)
	if err != nil {
		return nil
	}
	st, ok := findStruct(p, site.Symbol)
	if !ok {
		return nil
	}
	return o.doses(c, p, st)
}

func (o inlineParameterObject) doses(c *harness.Corpus, p *program, st paramStruct) []int {
	n := len(o.expansions(c, p, st))
	if o.target == "long-parameter-list" {
		n = min(n, 1)
	}
	doses := make([]int, min(n, maxClumpDose))
	for i := range doses {
		doses[i] = i + 1
	}
	return doses
}

func (o inlineParameterObject) expansions(c *harness.Corpus, p *program, st paramStruct) []expansion {
	all := candidates(p, st)
	if o.target != "long-parameter-list" {
		return all
	}
	limit := int(threshold(c, "parameters", 4))
	var out []expansion
	for _, e := range all {
		if e.paramCount() > limit && !e.f.baselineHas(c, o.target, e.f.symbol) {
			out = append(out, e)
		}
	}
	return out
}

func (o inlineParameterObject) Apply(c *harness.Corpus, site harness.Site, dose int) (harness.Applied, error) {
	p, err := programOf(c)
	if err != nil {
		return harness.Applied{}, err
	}
	st, ok := findStruct(p, site.Symbol)
	if !ok {
		return harness.Applied{}, errors.New("site vanished")
	}
	es := o.expansions(c, p, st)
	if dose > len(es) {
		return harness.Applied{}, errors.New("dose exceeds candidates")
	}
	es = es[:dose]
	edit, err := expand(p, st, es)
	if err != nil {
		return harness.Applied{}, err
	}
	var kill bool
	var reason string
	touched := []string{st.symbol}
	switch o.target {
	case "long-parameter-list":
		limit := int(threshold(c, "parameters", 4))
		n := es[0].paramCount()
		kill, reason = n > limit, fmt.Sprintf("%s has %d parameters after expansion (limit %d)", es[0].f.decl.Name.Name, n, limit)
		touched = append(touched, es[0].f.symbol)
	default:
		occ := int(threshold(c, "data-clump-occurrences", 3))
		sup := support(p, st.pkgPath(), st.fieldTypes(), es)
		kill = sup.count >= occ
		reason = fmt.Sprintf("%d declarations carry {%s} after expanding %d (occurrences >= %d)", sup.count, strings.Join(st.fieldTypes(), ","), dose, occ)
		touched = append(touched, sup.symbols...)
		size := int(threshold(c, "data-clump-size", 3))
		if inc, ok := incidentalClump(p, st, es, size, occ); ok {
			touched = append(touched, inc.symbols...)
			if !kill {
				kill = true
				reason = fmt.Sprintf("expanded parameters newly share {%s} with %d declarations (size >= %d, occurrences >= %d)", strings.Join(inc.types, ","), len(inc.symbols), size, occ)
			}
		}
	}
	return harness.Applied{
		Edit: edit,
		Expectation: harness.Expectation{
			Touched:    touched,
			Prediction: harness.Prediction{Kill: &kill, Reason: reason},
		},
	}, nil
}

type paramStruct struct {
	unit
	spec   *ast.TypeSpec
	tn     *types.TypeName
	fields []*types.Var
	symbol string
}

func (s paramStruct) pkgPath() string { return s.pkg.PkgPath }

func (s paramStruct) site() harness.Site {
	return harness.Site{Package: s.pkgPath(), Symbol: s.symbol, Offset: s.offset(s.spec.Pos()), File: s.rel}
}

func (s paramStruct) fieldTypes() []string {
	out := make([]string, len(s.fields))
	for i, f := range s.fields {
		out[i] = clumpCanonical(f.Type())
	}
	return out
}

func paramStructs(p *program, size int) []paramStruct {
	var out []paramStruct
	for _, u := range p.units {
		if strings.HasSuffix(u.rel, "_test.go") || generated(u.file) {
			continue
		}
		for _, d := range u.file.Decls {
			g, ok := d.(*ast.GenDecl)
			if !ok || g.Tok != token.TYPE {
				continue
			}
			for _, s := range g.Specs {
				ts := s.(*ast.TypeSpec)
				if st, ok := asParamStruct(u, ts); ok && len(st.fields) >= size {
					out = append(out, st)
				}
			}
		}
	}
	return out
}

func asParamStruct(u unit, ts *ast.TypeSpec) (paramStruct, bool) {
	if ts.TypeParams != nil || ts.Assign.IsValid() {
		return paramStruct{}, false
	}
	tn, ok := u.pkg.TypesInfo.Defs[ts.Name].(*types.TypeName)
	if !ok {
		return paramStruct{}, false
	}
	s, ok := tn.Type().Underlying().(*types.Struct)
	if !ok {
		return paramStruct{}, false
	}
	var fields []*types.Var
	for i := 0; i < s.NumFields(); i++ {
		f := s.Field(i)
		if f.Embedded() || f.Name() == "_" {
			return paramStruct{}, false
		}
		fields = append(fields, f)
	}
	return paramStruct{unit: u, spec: ts, tn: tn, fields: fields, symbol: u.pkg.PkgPath + "." + ts.Name.Name}, true
}

func findStruct(p *program, symbol string) (paramStruct, bool) {
	for _, st := range paramStructs(p, 0) {
		if st.symbol == symbol {
			return st, true
		}
	}
	return paramStruct{}, false
}

type expansion struct {
	f     fn
	index int
	ptr   bool
	names []string
	// whole: the body uses the struct as a value, so it is rebuilt locally from the new parameters.
	whole bool
	reads []*ast.SelectorExpr
}

func (e expansion) paramCount() int { return len(params(e.f)) - 1 + len(e.names) }

func candidates(p *program, st paramStruct) []expansion {
	var out []expansion
	for _, f := range p.funcs() {
		if f.pkg.PkgPath != st.pkgPath() {
			continue
		}
		if e, err := plan(p, st, f); err == nil {
			out = append(out, e)
		}
	}
	return out
}

func plan(p *program, st paramStruct, f fn) (expansion, error) {
	if err := p.rewritable(f); err != nil {
		return expansion{}, err
	}
	e := expansion{f: f, index: -1}
	want := clumpCanonical(st.tn.Type())
	sig := f.obj.Type().(*types.Signature)
	for i, q := range params(f) {
		t := clumpCanonical(q.v.Type())
		if t != want && t != "*"+want {
			continue
		}
		if e.index >= 0 || (sig.Variadic() && i == sig.Params().Len()-1) {
			return expansion{}, errors.New("ambiguous or variadic struct parameter")
		}
		e.index, e.ptr = i, t != want
	}
	if e.index < 0 {
		return expansion{}, errors.New("no struct parameter")
	}
	q := params(f)[e.index]
	if q.named() {
		var err error
		if e.reads, e.whole, err = fieldReads(f, q.v, e.ptr); err != nil {
			return expansion{}, err
		}
	}
	used := usedNames(f.decl)
	if q.named() && !e.whole {
		delete(used, q.name.Name)
	}
	for _, fld := range st.fields {
		e.names = append(e.names, clumpFresh(clumpLowerFirst(fld.Name()), used))
	}
	for _, cs := range p.calls[funcKey(f.obj)] {
		if _, err := fieldArgs(cs, cs.call.Args[e.index], st, func(n ast.Node) string { return cs.text(n) }); err != nil {
			return expansion{}, err
		}
	}
	return e, nil
}

// Through a pointer, a write or a whole use would change what the caller sees, so the function is not a candidate.
func fieldReads(f fn, v *types.Var, ptr bool) ([]*ast.SelectorExpr, bool, error) {
	info := f.pkg.TypesInfo
	var reads []*ast.SelectorExpr
	whole := false
	var stack []ast.Node
	ast.Inspect(f.decl.Body, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		if id, ok := n.(*ast.Ident); ok && info.Uses[id] == v {
			sel, ok := stack[len(stack)-1].(*ast.SelectorExpr)
			if ok && sel.X == id && info.Selections[sel] != nil && info.Selections[sel].Kind() == types.FieldVal {
				reads = append(reads, sel)
			} else {
				whole = true
			}
		}
		stack = append(stack, n)
		return true
	})
	if !ptr {
		return reads, whole, nil
	}
	if whole {
		return nil, false, errors.New("pointer parameter used whole")
	}
	written := writeRoots(f)
	for _, r := range reads {
		if written[r] {
			return nil, false, errors.New("pointer parameter field written")
		}
	}
	return reads, false, nil
}

func writeRoots(f fn) map[*ast.SelectorExpr]bool {
	info := f.pkg.TypesInfo
	out := map[*ast.SelectorExpr]bool{}
	mark := func(e ast.Expr) {
		for e != nil {
			switch x := e.(type) {
			case *ast.ParenExpr:
				e = x.X
			case *ast.IndexExpr:
				e = x.X
			case *ast.SelectorExpr:
				out[x] = true
				e = x.X
			default:
				return
			}
		}
	}
	ast.Inspect(f.decl.Body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.AssignStmt:
			for _, l := range n.Lhs {
				mark(l)
			}
		case *ast.IncDecStmt:
			mark(n.X)
		case *ast.UnaryExpr:
			if n.Op == token.AND {
				mark(n.X)
			}
		case *ast.RangeStmt:
			if n.Tok == token.ASSIGN {
				mark(n.Key)
				mark(n.Value)
			}
		case *ast.SelectorExpr:
			if sel := info.Selections[n]; sel != nil && sel.Kind() == types.MethodVal {
				if sig, ok := sel.Obj().Type().(*types.Signature); ok && sig.Recv() != nil {
					if _, isPtr := sig.Recv().Type().(*types.Pointer); isPtr {
						mark(n.X)
					}
				}
			}
		}
		return true
	})
	return out
}

func fieldArgs(cs clumpCallSite, arg ast.Expr, st paramStruct, text func(ast.Node) string) ([]string, error) {
	e := ast.Unparen(arg)
	if u, ok := e.(*ast.UnaryExpr); ok && u.Op == token.AND {
		e = ast.Unparen(u.X)
	}
	for _, f := range st.fields {
		if !f.Exported() && cs.pkg.PkgPath != st.pkgPath() {
			return nil, errors.New("unexported field read from another package")
		}
	}
	if lit, ok := e.(*ast.CompositeLit); ok {
		return literalFields(cs, lit, st, text)
	}
	if tv, ok := cs.pkg.TypesInfo.Types[e]; !ok || tv.IsNil() || !simple(e) {
		return nil, errors.New("struct argument is not side-effect free")
	}
	base := text(e)
	out := make([]string, len(st.fields))
	for i, f := range st.fields {
		out[i] = base + "." + f.Name()
	}
	return out, nil
}

func literalFields(cs clumpCallSite, lit *ast.CompositeLit, st paramStruct, text func(ast.Node) string) ([]string, error) {
	out := make([]string, len(st.fields))
	byName := map[string]int{}
	for i, f := range st.fields {
		byName[f.Name()] = i
	}
	for i, el := range lit.Elts {
		val, idx := el, i
		if kv, ok := el.(*ast.KeyValueExpr); ok {
			id, _ := kv.Key.(*ast.Ident)
			if id == nil {
				return nil, errors.New("unexpected literal key")
			}
			val, idx = kv.Value, byName[id.Name]
		}
		if !pure(val) {
			return nil, errors.New("literal element has effects")
		}
		out[idx] = text(val)
	}
	qual := qualifier(cs.unit)
	for i, f := range st.fields {
		if out[i] == "" {
			out[i] = "*new(" + types.TypeString(f.Type(), qual) + ")"
		}
	}
	return out, nil
}

func expand(p *program, st paramStruct, es []expansion) (harness.Edit, error) {
	r := newRewrite()
	var calls []clumpCallSite
	byKey := map[string]expansion{}
	for _, e := range es {
		f := e.f
		qual := qualifier(f.unit)
		if e.whole {
			name := params(f)[e.index].name.Name
			kv := make([]string, len(st.fields))
			for i, fld := range st.fields {
				kv[i] = fld.Name() + ": " + e.names[i]
			}
			lit := st.tn.Name() + "{" + strings.Join(kv, ", ") + "}"
			if e.ptr {
				lit = "&" + lit
			}
			at := f.offset(f.decl.Body.Lbrace) + 1
			r.add(f.unit, at, at, "\n"+name+" := "+lit+"\n")
		}
		for _, sel := range e.reads {
			if !e.whole {
				r.add(f.unit, f.offset(sel.Pos()), f.offset(sel.End()), e.names[fieldIndex(st, sel.Sel.Name)])
			}
		}
		ps := params(f)
		unnamed := ps[0].name == nil
		var list []string
		for i, q := range ps {
			if i != e.index {
				list = append(list, paramText(f, q, unnamed))
				continue
			}
			for j, fld := range st.fields {
				t := types.TypeString(fld.Type(), qual)
				if !unnamed {
					t = e.names[j] + " " + t
				}
				list = append(list, t)
			}
		}
		replaceParams(r, f, list)
		k := funcKey(f.obj)
		byKey[k] = e
		calls = append(calls, p.calls[k]...)
	}
	sortCallsInnermostFirst(calls)
	for _, cs := range calls {
		obj := cs.pkg.TypesInfo.Uses[calleeIdent(cs.call.Fun)].(*types.Func)
		e := byKey[funcKey(obj)]
		var args []string
		for i, a := range cs.call.Args {
			if i != e.index {
				args = append(args, r.nodeText(cs.unit, a))
				continue
			}
			fa, err := fieldArgs(cs, a, st, func(n ast.Node) string { return r.nodeText(cs.unit, n) })
			if err != nil {
				return nil, err
			}
			args = append(args, fa...)
		}
		replaceArgs(r, cs, args)
	}
	return r.edit()
}

func fieldIndex(st paramStruct, name string) int {
	for i, f := range st.fields {
		if f.Name() == name {
			return i
		}
	}
	return -1
}

type supportSet struct {
	count   int
	symbols []string
}

// support counts as Columbo's Data Clump support does.
func support(p *program, pkgPath string, clump []string, es []expansion) supportSet {
	want := multiset(clump)
	expanded := map[string]expansion{}
	for _, e := range es {
		expanded[e.f.symbol] = e
	}
	var s supportSet
	for _, f := range p.funcs() {
		if f.pkg.PkgPath != pkgPath {
			continue
		}
		ts := paramTypes(f)
		if e, ok := expanded[f.symbol]; ok {
			ts = append(append(append([]string{}, ts[:e.index]...), ts[e.index+1:]...), clump...)
		}
		if contains(multiset(ts), want) {
			s.count++
			s.symbols = append(s.symbols, f.symbol)
		}
	}
	sort.Strings(s.symbols)
	return s
}

type clumpDecl struct {
	symbol     string
	now, was   map[string]int
	isExpanded bool
}

type incidental struct {
	types   []string
	symbols []string
}

// Columbo reports every closed frequent multiset, so a wide struct can complete a clump of common types with unrelated declarations.
func incidentalClump(p *program, st paramStruct, es []expansion, size, occ int) (incidental, bool) {
	expanded := map[string]expansion{}
	for _, e := range es {
		expanded[e.f.symbol] = e
	}
	var decls []clumpDecl
	for _, f := range p.funcs() {
		if f.pkg.PkgPath != st.pkgPath() {
			continue
		}
		ts := paramTypes(f)
		d := clumpDecl{symbol: f.symbol, was: multiset(ts), now: multiset(ts)}
		if e, ok := expanded[f.symbol]; ok {
			d.now[ts[e.index]]--
			for _, t := range st.fieldTypes() {
				d.now[t]++
			}
			d.isExpanded = true
		}
		if total(d.now) >= size {
			decls = append(decls, d)
		}
	}
	for _, e := range decls {
		if !e.isExpanded {
			continue
		}
		if common, chosen, ok := shareWith(decls, e, e.now, []string{e.symbol}, 0, size, occ); ok {
			var types []string
			for t, n := range common {
				for range n {
					types = append(types, t)
				}
			}
			sort.Strings(types)
			return incidental{types: types, symbols: chosen}, true
		}
	}
	return incidental{}, false
}

func shareWith(decls []clumpDecl, e clumpDecl, common map[string]int, chosen []string, from, size, occ int) (map[string]int, []string, bool) {
	if len(chosen) == occ {
		return common, chosen, !contains(e.was, common)
	}
	for i := from; i < len(decls); i++ {
		d := decls[i]
		if d.symbol == e.symbol {
			continue
		}
		next := intersect(common, d.now)
		if total(next) < size || contains(e.was, next) {
			continue
		}
		if c, ch, ok := shareWith(decls, e, next, append(append([]string{}, chosen...), d.symbol), i+1, size, occ); ok {
			return c, ch, true
		}
	}
	return nil, nil, false
}

func intersect(a, b map[string]int) map[string]int {
	out := map[string]int{}
	for t, n := range a {
		if m := min(n, b[t]); m > 0 {
			out[t] = m
		}
	}
	return out
}

func total(m map[string]int) int {
	n := 0
	for _, c := range m {
		n += c
	}
	return n
}
