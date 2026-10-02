package columbo

import (
	"go/ast"
	"go/token"
	"go/types"
	"math"
	"math/big"
	"reflect"
	"sort"
	"strconv"
)

func canonicalType(t types.Type, sig *types.Signature) string {
	vars := map[*types.TypeParam]*types.TypeParam{}
	if sig != nil {
		for i := 0; i < sig.RecvTypeParams().Len(); i++ {
			old := sig.RecvTypeParams().At(i)
			vars[old] = types.NewTypeParam(types.NewTypeName(token.NoPos, nil, "R"+strconv.Itoa(i), nil), old.Constraint())
		}
		for i := 0; i < sig.TypeParams().Len(); i++ {
			old := sig.TypeParams().At(i)
			vars[old] = types.NewTypeParam(types.NewTypeName(token.NoPos, nil, "T"+strconv.Itoa(i), nil), old.Constraint())
		}
	}
	return types.TypeString(normalType(t, vars), func(p *types.Package) string { return p.Path() })
}

// Rebuild only structural components. Named underlying types remain opaque.
type typeNormalizer struct {
	vars     map[*types.TypeParam]*types.TypeParam
	handlers map[reflect.Type]normalization
}
type normalization func(*typeNormalizer, types.Type) types.Type

var normalizations = map[reflect.Type]normalization{
	reflect.TypeOf((*types.Basic)(nil)):     (*typeNormalizer).basic,
	reflect.TypeOf((*types.TypeParam)(nil)): (*typeNormalizer).typeParam,
	reflect.TypeOf((*types.Named)(nil)):     (*typeNormalizer).named,
	reflect.TypeOf((*types.Pointer)(nil)):   (*typeNormalizer).pointer,
	reflect.TypeOf((*types.Slice)(nil)):     (*typeNormalizer).slice,
	reflect.TypeOf((*types.Array)(nil)):     (*typeNormalizer).array,
	reflect.TypeOf((*types.Map)(nil)):       (*typeNormalizer).mapping,
	reflect.TypeOf((*types.Chan)(nil)):      (*typeNormalizer).channel,
	reflect.TypeOf((*types.Struct)(nil)):    (*typeNormalizer).structure,
	reflect.TypeOf((*types.Signature)(nil)): (*typeNormalizer).signature,
	reflect.TypeOf((*types.Interface)(nil)): (*typeNormalizer).iface,
	reflect.TypeOf((*types.Union)(nil)):     (*typeNormalizer).union,
}

func normalType(t types.Type, vars map[*types.TypeParam]*types.TypeParam) types.Type {
	return (&typeNormalizer{vars: vars, handlers: normalizations}).normalize(t)
}
func (n *typeNormalizer) normalize(t types.Type) types.Type {
	t = types.Unalias(t)
	if f := n.handlers[reflect.TypeOf(t)]; f != nil {
		return f(n, t)
	}
	return t
}
func (n *typeNormalizer) basic(t types.Type) types.Type { return types.Typ[t.(*types.Basic).Kind()] }
func (n *typeNormalizer) typeParam(t types.Type) types.Type {
	if v := n.vars[t.(*types.TypeParam)]; v != nil {
		return v
	}
	return t
}
func (n *typeNormalizer) named(t types.Type) types.Type {
	named := t.(*types.Named)
	if named.TypeArgs().Len() == 0 {
		return t
	}
	args := n.typeArguments(named.TypeArgs())
	out, e := types.Instantiate(nil, named.Origin(), args, false)
	if e == nil {
		return out
	}
	return t
}
func (n *typeNormalizer) typeArguments(ts *types.TypeList) []types.Type {
	out := []types.Type{}
	for i := 0; i < ts.Len(); i++ {
		out = append(out, n.normalize(ts.At(i)))
	}
	return out
}
func (n *typeNormalizer) pointer(t types.Type) types.Type {
	return types.NewPointer(n.normalize(t.(*types.Pointer).Elem()))
}
func (n *typeNormalizer) slice(t types.Type) types.Type {
	return types.NewSlice(n.normalize(t.(*types.Slice).Elem()))
}
func (n *typeNormalizer) array(t types.Type) types.Type {
	a := t.(*types.Array)
	return types.NewArray(n.normalize(a.Elem()), a.Len())
}
func (n *typeNormalizer) mapping(t types.Type) types.Type {
	m := t.(*types.Map)
	return types.NewMap(n.normalize(m.Key()), n.normalize(m.Elem()))
}
func (n *typeNormalizer) channel(t types.Type) types.Type {
	c := t.(*types.Chan)
	return types.NewChan(c.Dir(), n.normalize(c.Elem()))
}
func (n *typeNormalizer) structure(t types.Type) types.Type {
	s := t.(*types.Struct)
	fields := []*types.Var{}
	tags := []string{}
	for i := 0; i < s.NumFields(); i++ {
		fields = append(fields, n.field(s.Field(i)))
		tags = append(tags, s.Tag(i))
	}
	return types.NewStruct(fields, tags)
}
func (n *typeNormalizer) field(f *types.Var) *types.Var {
	return types.NewField(f.Pos(), f.Pkg(), f.Name(), n.normalize(f.Type()), f.Embedded())
}
func (n *typeNormalizer) signature(t types.Type) types.Type {
	s := t.(*types.Signature)
	return types.NewSignatureType(nil, nil, nil, n.tuple(s.Params()), n.tuple(s.Results()), s.Variadic())
}
func (n *typeNormalizer) tuple(old *types.Tuple) *types.Tuple {
	vs := []*types.Var{}
	for i := 0; i < old.Len(); i++ {
		v := old.At(i)
		vs = append(vs, n.variable(v))
	}
	return types.NewTuple(vs...)
}
func (n *typeNormalizer) variable(v *types.Var) *types.Var {
	return types.NewVar(v.Pos(), v.Pkg(), "", n.normalize(v.Type()))
}
func (n *typeNormalizer) iface(t types.Type) types.Type {
	i := t.(*types.Interface)
	return types.NewInterfaceType(n.interfaceMethods(i), n.embeddedInterfaces(i)).Complete()
}
func (n *typeNormalizer) interfaceMethods(i *types.Interface) []*types.Func {
	out := []*types.Func{}
	for k := 0; k < i.NumExplicitMethods(); k++ {
		out = append(out, n.method(i.ExplicitMethod(k)))
	}
	return out
}
func (n *typeNormalizer) method(m *types.Func) *types.Func {
	return types.NewFunc(m.Pos(), m.Pkg(), m.Name(), n.normalize(m.Type()).(*types.Signature))
}
func (n *typeNormalizer) embeddedInterfaces(i *types.Interface) []types.Type {
	out := []types.Type{}
	for k := 0; k < i.NumEmbeddeds(); k++ {
		out = append(out, n.normalize(i.EmbeddedType(k)))
	}
	return out
}
func (n *typeNormalizer) union(t types.Type) types.Type {
	u := t.(*types.Union)
	out := []*types.Term{}
	for i := 0; i < u.Len(); i++ {
		out = append(out, n.term(u.Term(i)))
	}
	return types.NewUnion(out)
}
func (n *typeNormalizer) term(t *types.Term) *types.Term {
	return types.NewTerm(t.Tilde(), n.normalize(t.Type()))
}

func (a *engine) collectDependencies(d *declaration, t types.Type, set map[string]bool) {
	if t == nil {
		return
	}
	t = types.Unalias(t)
	switch t := t.(type) {
	case *types.Named:
		obj := t.Obj()
		excluded := a.private[obj] && a.fset.PositionFor(obj.Pos(), false).Filename == d.file.path
		if d.signature.Recv() != nil {
			if recv, ok := stripPointer(d.signature.Recv().Type()).(*types.Named); ok && recv.Origin() == t.Origin() {
				excluded = true
			}
		}
		if !excluded {
			set["type:"+canonicalType(t, d.signature)] = true
		}
		for i := 0; i < t.TypeArgs().Len(); i++ {
			a.collectDependencies(d, t.TypeArgs().At(i), set)
		}
	case *types.Interface:
		set["interface:"+canonicalType(t, d.signature)] = true
	case *types.Pointer:
		a.collectDependencies(d, t.Elem(), set)
	case *types.Array:
		a.collectDependencies(d, t.Elem(), set)
	case *types.Slice:
		a.collectDependencies(d, t.Elem(), set)
	case *types.Map:
		a.collectDependencies(d, t.Key(), set)
		a.collectDependencies(d, t.Elem(), set)
	case *types.Chan:
		a.collectDependencies(d, t.Elem(), set)
	case *types.Struct:
		for i := 0; i < t.NumFields(); i++ {
			a.collectDependencies(d, t.Field(i).Type(), set)
		}
	case *types.Signature:
		for i := 0; i < t.Params().Len(); i++ {
			a.collectDependencies(d, t.Params().At(i).Type(), set)
		}
		for i := 0; i < t.Results().Len(); i++ {
			a.collectDependencies(d, t.Results().At(i).Type(), set)
		}
	}
}
func (d *declaration) measure(a *engine) {
	d.depReceipts = []Source{}
	info := d.file.pkg.TypesInfo
	sites := map[string]Source{}
	add := func(node ast.Node, t types.Type) {
		if node == nil {
			return
		}
		set := map[string]bool{}
		a.collectDependencies(d, t, set)
		for id := range set {
			d.deps[id] = true
			r := d.file.receipt("dependency", node.Pos(), node.End(), Detail{Subject: id, Value: nil, Nesting: nil})
			sites[canonical(r)] = r
		}
	}
	ast.Inspect(d.fn, func(n ast.Node) bool {
		if n == nil {
			return true
		}
		if e, ok := n.(ast.Expr); ok {
			if tv, yes := info.Types[e]; yes && tv.IsType() {
				add(e, tv.Type)
			}
		}
		switch n := n.(type) {
		case *ast.Ident:
			obj := info.Uses[n]
			if obj != nil && obj.Pkg() != nil && obj.Pkg().Path() != d.file.pkg.PkgPath {
				id := "package:" + obj.Pkg().Path()
				d.deps[id] = true
				r := d.file.receipt("dependency", n.Pos(), n.End(), Detail{Subject: id, Value: nil, Nesting: nil})
				sites[canonical(r)] = r
			}
		case *ast.CallExpr:
			add(n, info.TypeOf(n.Fun))
			add(n, info.TypeOf(n))
			for _, arg := range n.Args {
				add(arg, info.TypeOf(arg))
			}
		case *ast.CompositeLit:
			add(n, info.TypeOf(n))
		case *ast.TypeAssertExpr:
			add(n, info.TypeOf(n))
		case *ast.SelectorExpr:
			if sel := info.Selections[n]; sel != nil {
				add(n, sel.Recv())
				add(n, sel.Type())
			}
		}
		return true
	})
	for _, r := range sites {
		d.depReceipts = append(d.depReceipts, r)
	}
	if d.fn.Body != nil {
		d.lineReceipts = d.linesEvidence("function-lines", nil, nil)
		d.lines = len(d.lineReceipts)
		v := &complexityVisitor{name: d.fn.Name, diagnosticsEnabled: true}
		ast.Walk(v, d.fn)
		d.complexity = v.complexity
		for _, ev := range v.diagnostics {
			d.complexityReceipts = append(d.complexityReceipts, d.event(ev, "cognitive-complexity"))
		}
	}
}
func (d *declaration) event(ev diagnostic, kind string) Source {
	end := ev.Pos + 1
	for _, t := range d.file.tokens {
		if t.pos == ev.Pos {
			end = t.end
			break
		}
	}
	return d.file.receipt("metric-contribution", ev.Pos, end, Detail{Subject: kind, Value: ev.Inc, Nesting: ev.Nesting})
}
func (d *declaration) linesEvidence(kind string, removed map[token.Pos]bool, trace *expansion) []Source {
	if d.fn.Body == nil {
		return []Source{}
	}
	lines := map[int]bool{}
	for _, t := range d.file.tokens {
		if t.pos <= d.fn.Body.Lbrace || t.pos >= d.fn.Body.Rbrace || t.tok == token.COMMENT || t.tok == token.LBRACE || t.tok == token.RBRACE || t.tok == token.SEMICOLON || removed[t.pos] {
			continue
		}
		for line := d.file.tf.Line(t.pos); line <= d.file.tf.Line(t.end-1); line++ {
			lines[line] = true
		}
	}
	keys := []int{}
	for l := range lines {
		keys = append(keys, l)
	}
	sort.Ints(keys)
	ss := []Source{}
	for _, l := range keys {
		start := d.file.tf.LineStart(l)
		end := d.file.tf.Pos(d.file.tf.Size())
		if l < d.file.tf.LineCount() {
			end = d.file.tf.LineStart(l + 1)
		}
		r := d.file.receipt("metric-contribution", start, end, Detail{Subject: kind, Value: 1, Nesting: nil})
		r.EndLine = l
		if trace != nil {
			r.Detail.Expansion = append([]string{}, trace.names...)
			r.Detail.ExpansionSites = append([]Site{}, trace.sites...)
		}
		ss = append(ss, r)
	}
	return ss
}
func (a *engine) ordinary(d *declaration) error {
	checks := []struct {
		smell, kind string
		value       int
		receipts    []Source
	}{{"long-parameter-list", "parameters", len(d.params), nil}}
	if d.fn.Body != nil {
		checks = append(checks, struct {
			smell, kind string
			value       int
			receipts    []Source
		}{"long-function", "function-lines", d.lines, d.lineReceipts}, struct {
			smell, kind string
			value       int
			receipts    []Source
		}{"high-cognitive-complexity", "cognitive-complexity", d.complexity, d.complexityReceipts}, struct {
			smell, kind string
			value       int
			receipts    []Source
		}{"excessive-dependencies", "dependencies", len(d.deps), d.depReceipts})
	}
	for _, x := range checks {
		if int64(x.value) <= a.config.Counts[x.kind] {
			continue
		}
		c, e := a.newCase(d, x.smell, "")
		if e != nil {
			return e
		}
		if c == nil {
			continue
		}
		c.Clues = append(c.Clues, metric(x.kind, d.symbol, x.value).compare(a.config.Counts[x.kind], ">"))
		appendSources(c, x.receipts)
		if x.smell == "long-parameter-list" {
			for _, p := range d.params {
				c.Receipts = append(c.Receipts, d.parameterReceipt(p))
			}
		}
		a.report.Cases = append(a.report.Cases, *c)
	}
	return a.envy(d)
}
func (d *declaration) parameterReceipt(p parameter) Source {
	return d.file.receipt("parameter", p.field.Pos(), p.field.End(), Detail{Subject: strconv.Itoa(p.index) + ":" + canonicalType(p.typ, d.signature), Value: max(1, len(p.field.Names)), Nesting: nil})
}
func sortedSet(m map[string]bool) []string {
	out := []string{}
	for s := range m {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}
func fraction(n, d int) *big.Rat {
	if d == 0 {
		return new(big.Rat)
	}
	return big.NewRat(int64(n), int64(d))
}
func meets(r *big.Rat, f float64) bool { return r.Cmp(new(big.Rat).SetFloat64(f)) >= 0 }
func rounded(r *big.Rat) float64 {
	scaled := new(big.Rat).Mul(r, big.NewRat(1000000, 1))
	q, rem := new(big.Int), new(big.Int)
	q.QuoRem(scaled.Num(), scaled.Denom(), rem)
	twice := new(big.Int).Lsh(rem, 1)
	cmp := twice.Cmp(scaled.Denom())
	if cmp > 0 || cmp == 0 && q.Bit(0) == 1 {
		q.Add(q, big.NewInt(1))
	}
	f, _ := new(big.Rat).SetFrac(q, big.NewInt(1000000)).Float64()
	if math.IsInf(f, 0) {
		return 0
	}
	return f
}
