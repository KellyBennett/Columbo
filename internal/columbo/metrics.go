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
	n := newTypeNormalizer(sig)
	return types.TypeString(n.normalize(t), func(p *types.Package) string { return p.Path() })
}
func newTypeNormalizer(sig *types.Signature) *typeNormalizer {
	n := &typeNormalizer{vars: map[*types.TypeParam]*types.TypeParam{}, handlers: normalizations}
	if sig != nil {
		n.bindParameters(sig.RecvTypeParams(), "R")
		n.bindParameters(sig.TypeParams(), "T")
	}
	return n
}
func alphaParameter(old *types.TypeParam, name string) *types.TypeParam {
	return types.NewTypeParam(parameterTypeName(name), old.Constraint())
}
func parameterTypeName(name string) *types.TypeName {
	return types.NewTypeName(token.NoPos, nil, name, nil)
}
func (n *typeNormalizer) bindParameters(list *types.TypeParamList, prefix string) {
	for i := 0; i < list.Len(); i++ {
		old := list.At(i)
		n.vars[old] = alphaParameter(old, prefix+strconv.Itoa(i))
	}
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
		fields = append(fields, normalizedField(s.Field(i), n.normalize))
		tags = append(tags, s.Tag(i))
	}
	return types.NewStruct(fields, tags)
}
func normalizedField(f *types.Var, normalize func(types.Type) types.Type) *types.Var {
	return types.NewField(f.Pos(), f.Pkg(), f.Name(), normalize(f.Type()), f.Embedded())
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

// dependencyCollector owns recursive type policy; dependencyScan owns source sites.
type dependencyCollector struct {
	engine      *engine
	declaration *declaration
	identities  map[string]bool
	packages    map[string]bool
	signature   *types.Signature
}

func (c *dependencyCollector) walk(t types.Type) {
	if t == nil {
		return
	}
	t = types.Unalias(t)
	switch t := t.(type) {
	case *types.Named:
		c.named(t)
	case *types.Interface:
		c.identities["interface:"+canonicalType(t, c.signature)] = true
	default:
		c.components(t)
	}
}
func (c *dependencyCollector) components(t types.Type) {
	for _, part := range typeComponents(t) {
		c.walk(part)
	}
}
func (c *dependencyCollector) named(t *types.Named) {
	c.recordNamed(t)
	for i := 0; i < t.TypeArgs().Len(); i++ {
		c.walk(t.TypeArgs().At(i))
	}
}
func (c *dependencyCollector) recordNamed(t *types.Named) {
	if c.excluded(t) {
		return
	}
	c.identities["type:"+canonicalType(t, c.signature)] = true
	if p := t.Obj().Pkg(); p != nil {
		c.packages[p.Path()] = true
	}
}
func (c *dependencyCollector) excluded(t *types.Named) bool {
	return c.engine.privateType(t.Obj(), c.declaration.file.path) || c.declaration.receiverType(t)
}
func (d *declaration) receiverType(t *types.Named) bool {
	if d.signature.Recv() == nil {
		return false
	}
	recv, ok := stripPointer(d.signature.Recv().Type()).(*types.Named)
	return ok && recv.Origin() == t.Origin()
}

type dependencyScan struct {
	declaration *declaration
	collector   *dependencyCollector
	info        *types.Info
	packagePath string
	sites       map[string]Source
}

func (d *declaration) dependencyScan(a *engine) *dependencyScan {
	d.deps = map[string]bool{}
	d.depTypePackages = map[string]bool{}
	c := &dependencyCollector{engine: a, declaration: d, packages: d.depTypePackages, signature: d.signature}
	return &dependencyScan{declaration: d, collector: c, info: d.file.typeInfo(), packagePath: d.file.packagePath(), sites: map[string]Source{}}
}
func (s *dependencyScan) add(n ast.Node, t types.Type) {
	if n == nil {
		return
	}
	s.collector.identities = map[string]bool{}
	s.collector.walk(t)
	for id := range s.collector.identities {
		s.record(n, id)
	}
}
func (s *dependencyScan) record(n ast.Node, id string) {
	s.declaration.deps[id] = true
	r := s.declaration.dependencyReceipt(n, id)
	s.sites[canonical(r)] = r
}
func (d *declaration) dependencyReceipt(n ast.Node, id string) Source {
	source := d.source("dependency", n.Pos(), n.End(), Detail{Subject: id, Value: nil, Nesting: nil})
	source.DependencyIdentity = id
	return source
}
func (s *dependencyScan) visit(n ast.Node) bool {
	s.explicitType(n)
	switch n := n.(type) {
	case *ast.Ident:
		s.identifier(n)
	case *ast.CallExpr:
		s.call(n)
	case ast.Expr:
		s.expression(n)
	}
	return true
}
func (s *dependencyScan) explicitType(n ast.Node) {
	e, ok := n.(ast.Expr)
	if !ok {
		return
	}
	if tv, yes := s.info.Types[e]; yes && tv.IsType() {
		s.add(e, tv.Type)
	}
}
func (s *dependencyScan) identifier(n *ast.Ident) {
	obj := s.info.Uses[n]
	if obj != nil && obj.Pkg() != nil && obj.Pkg().Path() != s.packagePath {
		s.record(n, "package:"+obj.Pkg().Path())
	}
}
func (s *dependencyScan) call(n *ast.CallExpr) {
	s.add(n, s.info.TypeOf(n.Fun))
	s.add(n, s.info.TypeOf(n))
	for _, arg := range n.Args {
		s.add(arg, s.info.TypeOf(arg))
	}
}
func (s *dependencyScan) expression(e ast.Expr) {
	switch n := e.(type) {
	case *ast.CompositeLit:
		s.expressionType(n)
	case *ast.TypeAssertExpr:
		s.expressionType(n)
	case *ast.SelectorExpr:
		s.selection(n)
	}
}
func (s *dependencyScan) expressionType(e ast.Expr) { s.add(e, s.info.TypeOf(e)) }
func (s *dependencyScan) selection(n *ast.SelectorExpr) {
	if sel := s.info.Selections[n]; sel != nil {
		s.add(n, sel.Recv())
		s.add(n, sel.Type())
	}
}
func (s *dependencyScan) finish() {
	s.declaration.scoreDependencies()
	s.declaration.depReceipts = []Source{}
	for _, r := range s.sites {
		r.DependencyScored = s.declaration.deps[r.DependencyIdentity]
		if !r.DependencyScored {
			r.Kind = "dependency-inventory"
		}
		s.declaration.depReceipts = append(s.declaration.depReceipts, r)
	}
}
func (d *declaration) scoreDependencies() {
	delete(d.deps, "type:error")
	delete(d.deps, "interface:interface{}")
	for path := range d.depTypePackages {
		delete(d.deps, "package:"+path)
	}
}
func (d *declaration) measure(a *engine) {
	scan := d.dependencyScan(a)
	ast.Inspect(d.fn, scan.visit)
	scan.finish()
	if d.fn.Body != nil {
		d.measureBody()
	}
}
func (d *declaration) measureBody() {
	d.lineReceipts = d.linesEvidence("function-lines", nil, nil)
	d.lines = len(d.lineReceipts)
	d.measureComplexity()
}
func (d *declaration) measureComplexity() {
	v := scanComplexity(d.fn)
	d.complexity = v.complexity
	d.complexityReceipts = []Source{}
	for _, ev := range v.diagnostics {
		d.complexityReceipts = append(d.complexityReceipts, d.event(ev, "cognitive-complexity"))
	}
}
func (d *declaration) event(ev diagnostic, kind string) Source {
	pos := ev.position()
	source := d.source("metric-contribution", pos, d.file.tokenEnd(pos), ev.detail(kind))
	source.AggregateContributions = true
	return source
}

type lineEvidence struct {
	declaration *declaration
	file        *file
	start, end  token.Pos
	removed     map[token.Pos]bool
	lines       map[int]bool
	kind        string
	trace       *expansion
}

func (d *declaration) linesEvidence(kind string, removed map[token.Pos]bool, trace *expansion) []Source {
	start, end := d.bodyRange()
	if start == 0 {
		return []Source{}
	}
	evidence := &lineEvidence{declaration: d, file: d.file, start: start, end: end, removed: removed, lines: map[int]bool{}, kind: kind, trace: trace}
	evidence.scan()
	return evidence.receipts()
}
func (e *lineEvidence) includes(t lexToken) bool {
	return t.within(e.start, e.end) && t.countsAsCode() && !e.removed[t.pos]
}
func (e *lineEvidence) scan() {
	for _, t := range e.file.tokens {
		if !e.includes(t) {
			continue
		}
		for line := e.file.tf.Line(t.pos); line <= e.file.tf.Line(t.end-1); line++ {
			e.lines[line] = true
		}
	}
}
func (e *lineEvidence) receipts() []Source {
	keys := []int{}
	for line := range e.lines {
		keys = append(keys, line)
	}
	sort.Ints(keys)
	out := []Source{}
	for _, line := range keys {
		out = append(out, e.declaration.lineContribution(line, e.kind, e.trace))
	}
	return out
}

type metricCheck struct {
	smell, kind string
	value       int
	receipts    []Source
}

func (d *declaration) ordinaryChecks() []metricCheck {
	checks := []metricCheck{{"long-parameter-list", "parameters", len(d.params), nil}}
	if d.fn.Body == nil {
		return checks
	}
	return append(checks,
		metricCheck{"long-function", "function-lines", d.lines, d.lineReceipts},
		metricCheck{"high-cognitive-complexity", "cognitive-complexity", d.complexity, d.complexityReceipts},
		metricCheck{"excessive-dependencies", "dependencies", len(d.deps), d.depReceipts})
}

type ordinaryInvestigation struct {
	engine      *engine
	declaration *declaration
}

func (a *engine) ordinary(d *declaration) error {
	investigation := &ordinaryInvestigation{a, d}
	if e := investigation.run(); e != nil {
		return e
	}
	return a.envy(d)
}
func (i *ordinaryInvestigation) run() error {
	for _, check := range i.declaration.ordinaryChecks() {
		if e := i.check(check); e != nil {
			return e
		}
	}
	return nil
}
func (i *ordinaryInvestigation) check(check metricCheck) error {
	limit := i.engine.config.Counts[check.kind]
	if int64(check.value) <= limit {
		return nil
	}
	c, e := i.engine.newCase(i.declaration, check.smell, "")
	if e != nil || c == nil {
		return e
	}
	check.explain(c, limit)
	i.declaration.ordinaryEvidence(c, check.smell)
	i.engine.report.Cases = append(i.engine.report.Cases, *c)
	return nil
}
func (check metricCheck) explain(c *Case, limit int64) {
	c.threshold(check.kind, check.value, limit, ">")
	support := check.receipts
	if check.kind == "dependencies" {
		support = scoredDependencyReceipts(check.receipts)
	}
	c.Clues[len(c.Clues)-1] = c.Clues[len(c.Clues)-1].supportedBy(support)
	appendSources(c, check.receipts)
}
func (d *declaration) ordinaryEvidence(c *Case, smell string) {
	switch smell {
	case "excessive-dependencies":
		c.Clues = append(c.Clues, d.dependencyClue())
	case "long-parameter-list":
		d.parameterEvidence(c)
	}
}
func (d *declaration) parameterEvidence(c *Case) {
	for _, p := range d.params {
		c.parameterMetricReceipt(d.parameterReceipt(p))
	}
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
	q := roundedQuotient(new(big.Rat).Mul(r, big.NewRat(1000000, 1)))
	f, _ := new(big.Rat).SetFrac(q, big.NewInt(1000000)).Float64()
	if math.IsInf(f, 0) {
		return 0
	}
	return f
}
func roundedQuotient(r *big.Rat) *big.Int {
	q, rem := new(big.Int), new(big.Int)
	q.QuoRem(r.Num(), r.Denom(), rem)
	cmp := new(big.Int).Lsh(rem, 1).Cmp(r.Denom())
	if cmp > 0 || cmp == 0 && q.Bit(0) == 1 {
		q.Add(q, big.NewInt(1))
	}
	return q
}

func scoredDependencyReceipts(receipts []Source) []Source {
	out := []Source{}
	for _, receipt := range receipts {
		if receipt.DependencyScored {
			out = append(out, receipt)
		}
	}
	return out
}
