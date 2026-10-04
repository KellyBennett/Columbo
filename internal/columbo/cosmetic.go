package columbo

import (
	"fmt"
	"go/ast"
	"go/types"
	"math/big"
	"reflect"
	"sort"
	"strings"
)

type expansion struct {
	names        []string
	sites        []Site
	declarations []DeclarationRef
}

func extendTrace(t expansion, d *declaration, owner *declaration, c *ast.CallExpr) expansion {
	declarations := append([]DeclarationRef{}, t.declarations...)
	if len(declarations) == 0 {
		declarations = append(declarations, owner.ref())
	}
	return expansion{names: append(append([]string{}, t.names...), d.symbol), sites: append(append([]Site{}, t.sites...), owner.callSite(c)), declarations: append(declarations, d.ref())}
}
func onStack(ds []*declaration, d *declaration) bool {
	for _, s := range ds {
		if s == d {
			return true
		}
	}
	return false
}

type helperPair struct {
	left, right *declaration
	overlap     *big.Rat
}

type helperCluster struct {
	owner        *declaration
	calls        []*ast.CallExpr
	helpers      []*declaration
	forwarding   map[*ast.CallExpr]map[string]bool
	p            map[string]bool
	meanP, meanD *big.Rat
	pairs        map[string]helperPair
}

func (a *engine) forwarding(d *declaration, call *ast.CallExpr) map[string]bool {
	out := map[string]bool{}
	for _, expr := range forwardedExpressions(d, call) {
		if key, ok := d.inputIdentity(expr); ok {
			out[key] = true
		}
	}
	return out
}
func (a *engine) exception(d *declaration, call *ast.CallExpr) bool {
	info := d.file.typeInfo()
	return primitiveCall(info, call) || loggingException(callObject(info, call))
}
func loggingException(fn *types.Func) bool {
	if fn == nil || fn.Pkg() == nil {
		return false
	}
	if sig, ok := fn.Type().(*types.Signature); ok && sig.Recv() != nil {
		return false
	}
	return loggingName(fn.Pkg().Path(), fn.Name())
}

var clusterLogging = map[string]string{
	"fmt":      "Errorf",
	"log":      "Print Printf Println Fatal Fatalf Fatalln Panic Panicf Panicln",
	"log/slog": "Debug DebugContext Info InfoContext Warn WarnContext Error ErrorContext Log LogAttrs",
}

func loggingName(path, name string) bool {
	for _, allowed := range strings.Fields(clusterLogging[path]) {
		if name == allowed {
			return true
		}
	}
	return false
}
func newHelperCluster(owner *declaration, calls []*ast.CallExpr) *helperCluster {
	return &helperCluster{
		owner:      owner,
		calls:      append([]*ast.CallExpr{}, calls...),
		forwarding: map[*ast.CallExpr]map[string]bool{},
		p:          map[string]bool{},
		meanP:      new(big.Rat),
		meanD:      new(big.Rat),
		pairs:      map[string]helperPair{},
	}
}
func (a *engine) qualify(owner *declaration, calls []*ast.CallExpr) *helperCluster {
	cl := newHelperCluster(owner, calls)
	cl.helpers = a.clusterHelpers(calls)
	if int64(len(cl.helpers)) < a.config.Counts["cosmetic-min-helpers"] {
		return nil
	}
	cl.measureForwarding(a)
	cl.measureDependencies()
	if !cl.matches(a.config) {
		return nil
	}
	return cl
}
func (a *engine) clusterHelpers(calls []*ast.CallExpr) []*declaration {
	distinct := map[*declaration]bool{}
	for _, call := range calls {
		distinct[a.calls[call]] = true
	}
	out := []*declaration{}
	for helper := range distinct {
		out = append(out, helper)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].symbol < out[j].symbol })
	return out
}
func (cl *helperCluster) measureForwarding(a *engine) {
	for _, key := range cl.owner.inputs {
		cl.p[key] = true
	}
	for _, call := range cl.calls {
		inputs := a.forwarding(cl.owner, call)
		cl.forwarding[call] = inputs
		cl.meanP.Add(cl.meanP, fraction(len(inputs), len(cl.p)))
	}
	cl.meanP.Quo(cl.meanP, big.NewRat(int64(len(cl.calls)), 1))
}
func (cl *helperCluster) measureDependencies() {
	pairs := 0
	for i, helper := range cl.helpers {
		for _, other := range cl.helpers[i+1:] {
			cl.recordPair(helper, other)
			pairs++
		}
	}
	cl.meanD.Quo(cl.meanD, big.NewRat(int64(pairs), 1))
}
func (cl *helperCluster) recordPair(helper, other *declaration) {
	overlap := setOverlap(helper.deps, other.deps)
	cl.pairs[canonical([]string{helper.symbol, other.symbol})] = helperPair{helper, other, overlap}
	cl.meanD.Add(cl.meanD, overlap)
}
func setOverlap(left, right map[string]bool) *big.Rat {
	union := map[string]bool{}
	intersection := 0
	for key := range left {
		union[key] = true
		if right[key] {
			intersection++
		}
	}
	for key := range right {
		union[key] = true
	}
	return fraction(intersection, len(union))
}
func (cl *helperCluster) matches(config Config) bool {
	return meets(cl.meanP, config.Ratios["cosmetic-parameter-overlap"]) && meets(cl.meanD, config.Ratios["cosmetic-dependency-overlap"])
}

// These grammar tables classify lexical scopes and sequence boundaries. They do
// not select helpers or relax policy: every boundary from the original rule is kept.
var clusterBoundaries = map[reflect.Type]bool{
	reflect.TypeOf((*ast.IfStmt)(nil)):         true,
	reflect.TypeOf((*ast.ForStmt)(nil)):        true,
	reflect.TypeOf((*ast.RangeStmt)(nil)):      true,
	reflect.TypeOf((*ast.SwitchStmt)(nil)):     true,
	reflect.TypeOf((*ast.TypeSwitchStmt)(nil)): true,
	reflect.TypeOf((*ast.SelectStmt)(nil)):     true,
	reflect.TypeOf((*ast.BlockStmt)(nil)):      true,
	reflect.TypeOf((*ast.LabeledStmt)(nil)):    true,
	reflect.TypeOf((*ast.GoStmt)(nil)):         true,
	reflect.TypeOf((*ast.DeferStmt)(nil)):      true,
	reflect.TypeOf((*ast.ReturnStmt)(nil)):     true,
	reflect.TypeOf((*ast.BranchStmt)(nil)):     true,
	reflect.TypeOf((*ast.EmptyStmt)(nil)):      true,
}
var nestedStatementScopes = map[reflect.Type]bool{
	reflect.TypeOf((*ast.FuncLit)(nil)):    true,
	reflect.TypeOf((*ast.BlockStmt)(nil)):  true,
	reflect.TypeOf((*ast.CaseClause)(nil)): true,
	reflect.TypeOf((*ast.CommClause)(nil)): true,
}

type clusterScan struct {
	engine   *engine
	owner    *declaration
	out      []*helperCluster
	sequence []*ast.CallExpr
}

func (a *engine) clusters(d *declaration) []*helperCluster {
	scan := &clusterScan{engine: a, owner: d, out: []*helperCluster{}}
	d.inspectBody(scan.visit)
	return scan.out
}
func nestedFunction(n ast.Node) bool { _, ok := n.(*ast.FuncLit); return ok }
func (s *clusterScan) visit(n ast.Node) bool {
	if nestedFunction(n) {
		return false
	}
	switch n := n.(type) {
	case *ast.BlockStmt:
		s.statements(n.List)
	case *ast.CaseClause:
		s.statements(n.Body)
	case *ast.CommClause:
		s.statements(n.Body)
	}
	return true
}
func (s *clusterScan) statements(statements []ast.Stmt) {
	for _, statement := range statements {
		s.statement(statement)
	}
	s.flush()
}
func (s *clusterScan) statement(statement ast.Stmt) {
	if clusterBoundaries[reflect.TypeOf(statement)] {
		s.flush()
		return
	}
	for _, call := range lexicalCalls(statement) {
		helper := s.engine.calls[call]
		if helper != nil && helper.candidate {
			s.sequence = append(s.sequence, call)
		} else if !s.engine.exception(s.owner, call) {
			s.flush()
		}
	}
}
func (s *clusterScan) flush() {
	if cluster := s.engine.qualify(s.owner, s.sequence); cluster != nil {
		s.out = append(s.out, cluster)
	}
	s.sequence = nil
}

type lexicalCallScan struct{ calls []*ast.CallExpr }

func lexicalCalls(statement ast.Stmt) []*ast.CallExpr {
	scan := &lexicalCallScan{calls: []*ast.CallExpr{}}
	ast.Inspect(statement, scan.visit)
	sort.Slice(scan.calls, func(i, j int) bool { return scan.calls[i].Pos() < scan.calls[j].Pos() })
	return scan.calls
}
func (s *lexicalCallScan) visit(n ast.Node) bool {
	if nestedStatementScopes[reflect.TypeOf(n)] {
		return false
	}
	if call, ok := n.(*ast.CallExpr); ok {
		s.calls = append(s.calls, call)
	}
	return true
}

type cosmeticInvestigation struct {
	engine                    *engine
	parent                    *declaration
	clusters                  []*helperCluster
	seen                      map[*declaration]bool
	helpers                   map[*declaration]bool
	lines, complexityReceipts []Source
	complexity                int
	finding                   *Case
}

func (a *engine) cosmetic() error {
	for _, d := range a.declarations {
		if d.file.included && d.fn.Body != nil {
			if e := a.cosmeticParent(d); e != nil {
				return e
			}
		}
	}
	return nil
}
func (a *engine) cosmeticParent(d *declaration) error {
	investigation := &cosmeticInvestigation{engine: a, parent: d, seen: map[*declaration]bool{}, helpers: map[*declaration]bool{}}
	return investigation.run()
}
func (i *cosmeticInvestigation) run() error {
	i.discover(i.parent)
	if len(i.clusters) == 0 {
		return nil
	}
	i.expand()
	if !i.exceedsLimits() {
		return nil
	}
	return i.report()
}
func (i *cosmeticInvestigation) discover(d *declaration) {
	if i.seen[d] {
		return
	}
	i.seen[d] = true
	i.clusters = append(i.clusters, i.engine.clusters(d)...)
	ast.Inspect(d.fn.Body, i.discoverCall)
}
func (i *cosmeticInvestigation) discoverCall(n ast.Node) bool {
	if c, ok := n.(*ast.CallExpr); ok {
		if target := i.engine.calls[c]; target != nil && target.candidate {
			i.discover(target)
		}
	}
	return true
}
func (i *cosmeticInvestigation) expand() {
	d := i.parent
	trace := d.rootExpansion()
	i.lines = i.engine.expandedLines(d, []*declaration{d}, trace)
	i.complexity, i.complexityReceipts = i.engine.expandedComplexity(d, []*declaration{d}, trace, 0)
}
func (i *cosmeticInvestigation) exceedsLimits() bool {
	return int64(len(i.lines)) > i.engine.config.Counts["function-lines"] || int64(i.complexity) > i.engine.config.Counts["cognitive-complexity"]
}
func (i *cosmeticInvestigation) report() error {
	c, e := i.engine.newCase(i.parent, "cosmetic-extraction", "")
	if e != nil || c == nil {
		return e
	}
	i.finding = c
	i.presentation()
	i.engine.report.Cases = append(i.engine.report.Cases, *c)
	return nil
}
func (i *cosmeticInvestigation) presentation() {
	i.metrics()
	i.originalAndExpandedReceipts()
	i.sortClusters()
	for _, cluster := range i.clusters {
		i.cluster(cluster)
	}
}
func (i *cosmeticInvestigation) sortClusters() {
	sort.Slice(i.clusters, func(a, b int) bool { return i.clusters[a].before(i.clusters[b]) })
}
func (cl *helperCluster) before(other *helperCluster) bool {
	left, right := cl.owner.callSite(cl.calls[0]), other.owner.callSite(other.calls[0])
	if left.File != right.File {
		return left.File < right.File
	}
	return left.CallOffset < right.CallOffset
}
func (i *cosmeticInvestigation) metrics() {
	i.metric("function-lines", i.parent.lines, "function-lines")
	i.metric("cognitive-complexity", i.parent.complexity, "cognitive-complexity")
	i.metric("expanded-lines", len(i.lines), "function-lines")
	i.metric("expanded-complexity", i.complexity, "cognitive-complexity")
}
func (i *cosmeticInvestigation) metric(kind string, value int, threshold string) {
	clue := i.finding.metric(kind, value).supportedBy(i.metricReceipts(kind))
	limit := i.engine.config.Counts[threshold]
	if int64(value) > limit {
		clue = clue.compare(limit, ">")
	}
	i.finding.Clues = append(i.finding.Clues, clue)
}
func (i *cosmeticInvestigation) originalAndExpandedReceipts() {
	appendSources(i.finding, i.parent.lineReceipts)
	appendSources(i.finding, i.parent.complexityReceipts)
	appendSources(i.finding, i.lines)
	appendSources(i.finding, i.complexityReceipts)
}
func (cl *helperCluster) key() string {
	site := cl.owner.callSite(cl.calls[0])
	return fmt.Sprintf("%s:%d", site.File, site.CallOffset)
}
func (cl *helperCluster) record() Cluster {
	ref := cl.owner.ref()
	return Cluster{Key: cl.key(), Owner: cl.owner.symbol, File: cl.owner.file.rel, Members: []Member{}, OwnerDeclaration: &ref}
}

type clusterPresentation struct {
	engine  *engine
	cluster *helperCluster
	finding *Case
	helpers map[*declaration]bool
}

func (i *cosmeticInvestigation) cluster(cl *helperCluster) {
	presentation := &clusterPresentation{i.engine, cl, i.finding, i.helpers}
	presentation.emit()
}
func (p *clusterPresentation) emit() {
	record := p.cluster.record()
	p.summary()
	p.finding.includeDeclaration(p.cluster.owner, "cluster-owner")
	p.members(&record)
	p.finding.Clusters = append(p.finding.Clusters, record)
	p.cluster.pairClues(p.finding, p.engine)
	for _, helper := range p.cluster.helpers {
		p.helper(helper)
	}
}
func (p *clusterPresentation) summary() { p.cluster.summaryClues(p.finding, p.engine) }
func (p *clusterPresentation) members(record *Cluster) {
	for _, call := range p.cluster.calls {
		record.Members = append(record.Members, p.member(call))
	}
}
func (p *clusterPresentation) member(call *ast.CallExpr) Member {
	p.memberEvidence(call)
	helper := p.engine.calls[call]
	site := p.cluster.owner.callSite(call)
	return helper.memberAt(site)
}
func (p *clusterPresentation) memberEvidence(call *ast.CallExpr) {
	helper := p.engine.calls[call]
	sources := p.cluster.memberReceipts(call, helper)
	p.cluster.memberClues(p.finding, call, helper, sources)
	appendSources(p.finding, sources)
}

// clusterEvidence owns the distinct evidence roles behind aggregate overlaps.
type clusterEvidence struct {
	cluster *helperCluster
	engine  *engine
}

func (cl *helperCluster) summaryClues(c *Case, engine *engine) {
	evidence := clusterEvidence{cl, engine}
	c.Clues = append(c.Clues, evidence.clues()...)
}
func (e clusterEvidence) clues() []Clue {
	cl, config, key := e.cluster, e.engine.config, e.cluster.key()
	parameters, dependencies := cl.displayedOverlaps()
	return []Clue{
		e.link(metric("helper-count", key, len(cl.helpers)).compare(config.Counts["cosmetic-min-helpers"], ">="), e.calls()),
		e.link(metric("parent-input-set", key, sortedSet(cl.p)), e.ownerSources()),
		e.link(metric("parameter-overlap", key+":mean", parameters).compare(config.Ratios["cosmetic-parameter-overlap"], ">="), e.forwarding()),
		e.link(metric("dependency-overlap", key+":mean", dependencies).compare(config.Ratios["cosmetic-dependency-overlap"], ">="), e.dependencies()),
	}
}
func (e clusterEvidence) link(q Clue, receipts []Source) Clue {
	owner := e.cluster.owner.ref()
	q.ClusterKey = e.cluster.key()
	return q.forDeclaration(&owner).supportedBy(receipts)
}
func (e clusterEvidence) calls() []Source {
	out := []Source{}
	for _, call := range e.cluster.calls {
		helper := e.engine.calls[call]
		out = append(out, e.cluster.owner.helperCallReceipt(call, helper.symbol), helper.declReceipt())
	}
	return out
}
func (e clusterEvidence) forwarding() []Source {
	out := []Source{e.cluster.owner.declReceipt()}
	for _, call := range e.cluster.calls {
		out = append(out, e.cluster.memberReceipts(call, e.engine.calls[call])...)
	}
	return out
}
func (e clusterEvidence) dependencies() []Source {
	out := []Source{}
	for _, helper := range e.cluster.helpers {
		out = append(out, helper.scoredEvidence()...)
	}
	return out
}
func (cl *helperCluster) memberClues(c *Case, call *ast.CallExpr, helper *declaration, receipts []Source) {
	subject := cl.key() + ":member:" + helper.symbol
	h := cl.forwarding[call]
	inputs := metric("forwarded-input-set", subject, sortedSet(h)).supportedBy(receipts)
	overlap := metric("parameter-overlap", subject, cl.forwardingOverlap(call)).supportedBy(append(append([]Source{}, receipts...), cl.owner.declReceipt()))
	c.Clues = append(c.Clues, cl.linkMember(inputs, call, helper), cl.linkMember(overlap, call, helper))
}
func (cl *helperCluster) linkMember(q Clue, call *ast.CallExpr, helper *declaration) Clue {
	ref := helper.memberReference(cl.key(), cl.owner.callSite(call))
	return cl.link(q).forMember(ref)
}
func (cl *helperCluster) link(q Clue) Clue {
	owner := cl.owner.ref()
	q.ClusterKey = cl.key()
	return q.forDeclaration(&owner)
}
func (cl *helperCluster) forwardingOverlap(call *ast.CallExpr) float64 {
	return rounded(fraction(len(cl.forwarding[call]), len(cl.p)))
}
func (e clusterEvidence) ownerSources() []Source {
	return []Source{e.cluster.owner.declReceipt()}
}
func (pair helperPair) value() float64 {
	return rounded(pair.overlap)
}
func (cl *helperCluster) pairClues(c *Case, engine *engine) {
	for key, pair := range cl.pairs {
		c.Clues = append(c.Clues, cl.pairClue(engine, key, pair))
	}
}
func (cl *helperCluster) pairClue(engine *engine, key string, pair helperPair) Clue {
	evidence := clusterEvidence{cl, engine}
	sources := append(pair.left.scoredEvidence(), pair.right.scoredEvidence()...)
	q := evidence.link(metric("dependency-overlap", cl.key()+":"+key, pair.value()), sources)
	q.Pair = []MemberRef{cl.helperReference(engine, pair.left), cl.helperReference(engine, pair.right)}
	return q
}
func (d *declaration) scoredEvidence() []Source {
	return append([]Source{d.declReceipt()}, scoredDependencyReceipts(d.depReceipts)...)
}
func (cl *helperCluster) helperReference(engine *engine, helper *declaration) MemberRef {
	for _, call := range cl.calls {
		if engine.calls[call] == helper {
			return helper.memberReference(cl.key(), cl.owner.callSite(call))
		}
	}
	panic("cluster helper missing its call")
}
func (i *clusterPresentation) helper(h *declaration) {
	if i.helpers[h] {
		return
	}
	i.helpers[h] = true
	i.finding.Clues = append(i.finding.Clues, h.dependencyClue())
	i.finding.includeDeclaration(h, "helper")
	appendSources(i.finding, h.depReceipts)
}
func (cl *helperCluster) memberReceipts(call *ast.CallExpr, helper *declaration) []Source {
	out := []Source{cl.owner.helperCallReceipt(call, helper.symbol)}
	return append(out, cl.owner.forwardingEvidence(call)...)
}
func (d *declaration) dependencyClue() Clue {
	ref := d.ref()
	return metric("dependency-set", d.symbol, sortedSet(d.deps)).forDeclaration(&ref).supportedBy(scoredDependencyReceipts(d.depReceipts))
}
func (d *declaration) forwardingEvidence(call *ast.CallExpr) []Source {
	out := []Source{}
	for _, arg := range forwardedExpressions(d, call) {
		if source, ok := d.forwardedEvidence(arg); ok {
			out = append(out, source)
		}
	}
	return out
}
func (d *declaration) forwardedEvidence(arg ast.Expr) (Source, bool) {
	id, ok := unparen(arg).(*ast.Ident)
	if !ok {
		return Source{}, false
	}
	key, ok := d.inputIdentity(id)
	return d.identifierReceipt(id, key), ok
}
func forwardedExpressions(d *declaration, call *ast.CallExpr) []ast.Expr {
	out := append([]ast.Expr{}, call.Args...)
	if receiver := forwardedReceiver(d.file.typeInfo(), call); receiver != nil {
		out = append(out, receiver)
	}
	return out
}
func (i *cosmeticInvestigation) metricReceipts(kind string) []Source {
	switch kind {
	case "function-lines":
		return i.parent.lineReceipts
	case "cognitive-complexity":
		return i.parent.complexityReceipts
	case "expanded-lines":
		return i.lines
	case "expanded-complexity":
		return i.complexityReceipts
	}
	return nil
}

func (cl *helperCluster) displayedOverlaps() (float64, float64) {
	return rounded(cl.meanP), rounded(cl.meanD)
}
