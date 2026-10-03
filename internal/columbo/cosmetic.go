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
	names []string
	sites []Site
}

func extendTrace(t expansion, d *declaration, owner *declaration, c *ast.CallExpr) expansion {
	return expansion{append(append([]string{}, t.names...), d.symbol), append(append([]Site{}, t.sites...), owner.callSite(c))}
}
func onStack(ds []*declaration, d *declaration) bool {
	for _, s := range ds {
		if s == d {
			return true
		}
	}
	return false
}

type helperCluster struct {
	owner        *declaration
	calls        []*ast.CallExpr
	helpers      []*declaration
	forwarding   map[*ast.CallExpr]map[string]bool
	p            map[string]bool
	meanP, meanD *big.Rat
	pairs        map[string]*big.Rat
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
		pairs:      map[string]*big.Rat{},
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
	cl.pairs[canonical([]string{helper.symbol, other.symbol})] = overlap
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
	trace := expansion{[]string{d.symbol}, []Site{}}
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
	limit := i.engine.config.Counts[threshold]
	if int64(value) > limit {
		i.finding.threshold(kind, value, limit, ">")
		return
	}
	i.finding.value(kind, value)
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
	return Cluster{cl.key(), cl.owner.symbol, cl.owner.file.rel, []Member{}}
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
	p.finding.Receipts = append(p.finding.Receipts, p.cluster.owner.declReceipt())
	p.members(&record)
	p.finding.Clusters = append(p.finding.Clusters, record)
	p.cluster.pairClues(p.finding)
	for _, helper := range p.cluster.helpers {
		p.helper(helper)
	}
}
func (p *clusterPresentation) summary() { p.cluster.summaryClues(p.finding, p.engine.config) }
func (p *clusterPresentation) members(record *Cluster) {
	for _, call := range p.cluster.calls {
		record.Members = append(record.Members, p.member(call))
	}
}
func (p *clusterPresentation) member(call *ast.CallExpr) Member {
	p.memberEvidence(call)
	helper := p.engine.calls[call]
	site := p.cluster.owner.callSite(call)
	return Member{helper.symbol, site.CallOffset}
}
func (p *clusterPresentation) memberEvidence(call *ast.CallExpr) {
	helper := p.engine.calls[call]
	p.cluster.memberClues(p.finding, call, helper.symbol)
	p.finding.Receipts = append(p.finding.Receipts, p.cluster.owner.helperCallReceipt(call, helper.symbol))
	p.cluster.owner.forwardingReceipts(p.finding, call)
}
func (cl *helperCluster) summaryClues(c *Case, config Config) {
	key := cl.key()
	c.Clues = append(c.Clues, metric("helper-count", key, len(cl.helpers)).compare(config.Counts["cosmetic-min-helpers"], ">="))
	c.Clues = append(c.Clues, metric("parent-input-set", key, sortedSet(cl.p)))
	c.Clues = append(c.Clues, metric("parameter-overlap", key+":mean", rounded(cl.meanP)).compare(config.Ratios["cosmetic-parameter-overlap"], ">="))
	c.Clues = append(c.Clues, metric("dependency-overlap", key+":mean", rounded(cl.meanD)).compare(config.Ratios["cosmetic-dependency-overlap"], ">="))
}
func (cl *helperCluster) memberClues(c *Case, call *ast.CallExpr, helper string) {
	subject := cl.key() + ":member:" + helper
	h := cl.forwarding[call]
	c.Clues = append(c.Clues, metric("forwarded-input-set", subject, sortedSet(h)), metric("parameter-overlap", subject, rounded(fraction(len(h), len(cl.p)))))
}
func (cl *helperCluster) pairClues(c *Case) {
	for pair, r := range cl.pairs {
		c.Clues = append(c.Clues, metric("dependency-overlap", cl.key()+":"+pair, rounded(r)))
	}
}
func (i *clusterPresentation) helper(h *declaration) {
	if i.helpers[h] {
		return
	}
	i.helpers[h] = true
	i.finding.Clues = append(i.finding.Clues, metric("dependency-set", h.symbol, sortedSet(h.deps)))
	i.finding.Receipts = append(i.finding.Receipts, h.declReceipt())
	appendSources(i.finding, h.depReceipts)
}
func (d *declaration) forwardingReceipts(c *Case, call *ast.CallExpr) {
	for _, arg := range forwardedExpressions(d, call) {
		if id, ok := unparen(arg).(*ast.Ident); ok {
			d.forwardedIdentifier(c, id)
		}
	}
}
func (d *declaration) forwardedIdentifier(c *Case, id *ast.Ident) {
	if key, ok := d.inputIdentity(id); ok {
		c.Receipts = append(c.Receipts, d.identifierReceipt(id, key))
	}
}
func forwardedExpressions(d *declaration, call *ast.CallExpr) []ast.Expr {
	out := append([]ast.Expr{}, call.Args...)
	if receiver := forwardedReceiver(d.file.typeInfo(), call); receiver != nil {
		out = append(out, receiver)
	}
	return out
}
