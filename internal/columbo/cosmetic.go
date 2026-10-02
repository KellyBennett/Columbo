package columbo

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"math/big"
	"sort"
	"strings"
)

type expansion struct {
	names []string
	sites []Site
}

func extendTrace(t expansion, d *declaration, owner *declaration, c *ast.CallExpr) expansion {
	return expansion{append(append([]string{}, t.names...), d.symbol), append(append([]Site{}, t.sites...), Site{owner.file.rel, owner.file.tf.Offset(c.Pos())})}
}
func onStack(ds []*declaration, d *declaration) bool {
	for _, s := range ds {
		if s == d {
			return true
		}
	}
	return false
}
func (a *engine) expandedLines(d *declaration, stack []*declaration, t expansion) []Source {
	removed := map[token.Pos]bool{}
	copies := []Source{}
	var visit func(ast.Node)
	visit = func(n ast.Node) {
		if n == nil {
			return
		}
		if call, ok := n.(*ast.CallExpr); ok {
			visit(call.Fun)
			for _, arg := range call.Args {
				visit(arg)
			}
			target := a.calls[call]
			if target == nil || !target.candidate || onStack(stack, target) {
				return
			}
			nested := []ast.Node{}
			ast.Inspect(call, func(n ast.Node) bool {
				if n == call {
					return true
				}
				if c, ok := n.(*ast.CallExpr); ok {
					nested = append(nested, c)
					return false
				}
				return true
			})
			for _, tok := range d.file.tokens {
				if tok.pos < call.Pos() || tok.pos >= call.End() {
					continue
				}
				retain := false
				for _, ch := range nested {
					if tok.pos >= ch.Pos() && tok.pos < ch.End() {
						retain = true
						break
					}
				}
				if !retain {
					removed[tok.pos] = true
				}
			}
			nt := extendTrace(t, target, d, call)
			copies = append(copies, a.expandedLines(target, append(stack, target), nt)...)
			return
		}
		ast.Inspect(n, func(ch ast.Node) bool {
			if ch == n {
				return true
			}
			if ch != nil {
				visit(ch)
			}
			return false
		})
	}
	visit(d.fn.Body)
	var trace *expansion
	if len(t.sites) > 0 {
		trace = &t
	}
	ss := d.linesEvidence("expanded-lines", removed, trace)
	return append(ss, copies...)
}
func (a *engine) expandedComplexity(d *declaration, stack []*declaration, t expansion, depth int) (int, []Source) {
	ss := []Source{}
	v := &complexityVisitor{name: d.fn.Name, nesting: depth, diagnosticsEnabled: true}
	v.hook = func(v *complexityVisitor, c *ast.CallExpr) bool {
		target := a.calls[c]
		if target == nil || !target.candidate || onStack(stack, target) {
			return false
		}
		ast.Walk(v, c.Fun)
		for _, arg := range c.Args {
			ast.Walk(v, arg)
		}
		n, rs := a.expandedComplexity(target, append(stack, target), extendTrace(t, target, d, c), v.nesting)
		v.complexity += n
		ss = append(ss, rs...)
		return true
	}
	ast.Walk(v, d.fn.Body)
	for _, ev := range v.diagnostics {
		r := d.event(ev, "expanded-complexity")
		if len(t.sites) > 0 {
			r.Detail.Expansion = append([]string{}, t.names...)
			r.Detail.ExpansionSites = append([]Site{}, t.sites...)
		}
		ss = append(ss, r)
	}
	return v.complexity, ss
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

func (a *engine) forwarding(d *declaration, c *ast.CallExpr) map[string]bool {
	h := map[string]bool{}
	add := func(e ast.Expr) {
		id, ok := unparen(e).(*ast.Ident)
		if !ok {
			return
		}
		v, ok := d.file.pkg.TypesInfo.Uses[id].(*types.Var)
		if ok {
			if key, yes := d.inputs[v]; yes {
				h[key] = true
			}
		}
	}
	for _, arg := range c.Args {
		add(arg)
	}
	fun := unparen(c.Fun)
	if s, ok := fun.(*ast.SelectorExpr); ok {
		if sel := d.file.pkg.TypesInfo.Selections[s]; sel != nil && sel.Kind() == types.MethodVal {
			add(s.X)
		}
	}
	return h
}
func (a *engine) exception(d *declaration, c *ast.CallExpr) bool {
	info := d.file.pkg.TypesInfo
	if tv := info.Types[c.Fun]; tv.IsType() {
		return true
	}
	if id, ok := unparen(c.Fun).(*ast.Ident); ok {
		if _, ok := info.Uses[id].(*types.Builtin); ok {
			return true
		}
	}
	f := callObject(info, c)
	if f == nil || f.Pkg() == nil {
		return false
	}
	if sig, ok := f.Type().(*types.Signature); ok && sig.Recv() != nil {
		return false
	}
	names := ""
	switch f.Pkg().Path() {
	case "fmt":
		names = "Errorf"
	case "log":
		names = "Print Printf Println Fatal Fatalf Fatalln Panic Panicf Panicln"
	case "log/slog":
		names = "Debug DebugContext Info InfoContext Warn WarnContext Error ErrorContext Log LogAttrs"
	}
	for _, name := range strings.Fields(names) {
		if name == f.Name() {
			return true
		}
	}
	return false
}
func (a *engine) qualify(d *declaration, calls []*ast.CallExpr) *helperCluster {
	distinct := map[*declaration]bool{}
	for _, call := range calls {
		distinct[a.calls[call]] = true
	}
	if int64(len(distinct)) < a.config.Counts["cosmetic-min-helpers"] {
		return nil
	}
	cl := &helperCluster{owner: d, calls: append([]*ast.CallExpr{}, calls...), forwarding: map[*ast.CallExpr]map[string]bool{}, p: map[string]bool{}, meanP: new(big.Rat), meanD: new(big.Rat), pairs: map[string]*big.Rat{}}
	for _, key := range d.inputs {
		cl.p[key] = true
	}
	for h := range distinct {
		cl.helpers = append(cl.helpers, h)
	}
	sort.Slice(cl.helpers, func(i, j int) bool { return cl.helpers[i].symbol < cl.helpers[j].symbol })
	for _, call := range calls {
		h := a.forwarding(d, call)
		cl.forwarding[call] = h
		cl.meanP.Add(cl.meanP, fraction(len(h), len(cl.p)))
	}
	cl.meanP.Quo(cl.meanP, big.NewRat(int64(len(calls)), 1))
	pairs := 0
	for i, h := range cl.helpers {
		for _, other := range cl.helpers[i+1:] {
			union := map[string]bool{}
			inter := 0
			for s := range h.deps {
				union[s] = true
				if other.deps[s] {
					inter++
				}
			}
			for s := range other.deps {
				union[s] = true
			}
			r := fraction(inter, len(union))
			cl.pairs[canonical([]string{h.symbol, other.symbol})] = r
			cl.meanD.Add(cl.meanD, r)
			pairs++
		}
	}
	cl.meanD.Quo(cl.meanD, big.NewRat(int64(pairs), 1))
	if !meets(cl.meanP, a.config.Ratios["cosmetic-parameter-overlap"]) || !meets(cl.meanD, a.config.Ratios["cosmetic-dependency-overlap"]) {
		return nil
	}
	return cl
}
func (a *engine) clusters(d *declaration) []*helperCluster {
	out := []*helperCluster{}
	scan := func(stmts []ast.Stmt) {
		sequence := []*ast.CallExpr{}
		flush := func() {
			if c := a.qualify(d, sequence); c != nil {
				out = append(out, c)
			}
			sequence = nil
		}
		for _, s := range stmts {
			boundary := false
			switch s.(type) {
			case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt, *ast.BlockStmt, *ast.LabeledStmt, *ast.GoStmt, *ast.DeferStmt, *ast.ReturnStmt, *ast.BranchStmt, *ast.EmptyStmt:
				boundary = true
			}
			if boundary {
				flush()
				continue
			}
			calls := []*ast.CallExpr{}
			ast.Inspect(s, func(n ast.Node) bool {
				switch n.(type) {
				case *ast.FuncLit, *ast.BlockStmt, *ast.CaseClause, *ast.CommClause:
					return false
				}
				if c, ok := n.(*ast.CallExpr); ok {
					calls = append(calls, c)
				}
				return true
			})
			sort.Slice(calls, func(i, j int) bool { return calls[i].Pos() < calls[j].Pos() })
			for _, call := range calls {
				h := a.calls[call]
				if h != nil && h.candidate {
					sequence = append(sequence, call)
				} else if !a.exception(d, call) {
					flush()
				}
			}
		}
		flush()
	}
	ast.Inspect(d.fn.Body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.BlockStmt:
			scan(n.List)
		case *ast.CaseClause:
			scan(n.Body)
		case *ast.CommClause:
			scan(n.Body)
		}
		return true
	})
	return out
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
	i := &cosmeticInvestigation{engine: a, parent: d, seen: map[*declaration]bool{}, helpers: map[*declaration]bool{}}
	i.discover(d)
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
	i.metrics()
	i.originalAndExpandedReceipts()
	i.sortClusters()
	for _, cl := range i.clusters {
		i.cluster(cl)
	}
	i.engine.report.Cases = append(i.engine.report.Cases, *c)
	return nil
}
func (i *cosmeticInvestigation) sortClusters() {
	sort.Slice(i.clusters, func(a, b int) bool {
		x, y := i.clusters[a], i.clusters[b]
		if x.owner.file.rel != y.owner.file.rel {
			return x.owner.file.rel < y.owner.file.rel
		}
		return x.calls[0].Pos() < y.calls[0].Pos()
	})
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
	return fmt.Sprintf("%s:%d", cl.owner.file.rel, cl.owner.file.tf.Offset(cl.calls[0].Pos()))
}
func (i *cosmeticInvestigation) cluster(cl *helperCluster) {
	record := Cluster{cl.key(), cl.owner.symbol, cl.owner.file.rel, []Member{}}
	cl.summaryClues(i.finding, i.engine.config)
	i.finding.Receipts = append(i.finding.Receipts, cl.owner.declReceipt())
	for _, call := range cl.calls {
		record.Members = append(record.Members, i.clusterMember(cl, call))
	}
	i.finding.Clusters = append(i.finding.Clusters, record)
	cl.pairClues(i.finding)
	for _, h := range cl.helpers {
		i.helper(h)
	}
}
func (cl *helperCluster) summaryClues(c *Case, config Config) {
	key := cl.key()
	c.Clues = append(c.Clues, metric("helper-count", key, len(cl.helpers)).compare(config.Counts["cosmetic-min-helpers"], ">="))
	c.Clues = append(c.Clues, metric("parent-input-set", key, sortedSet(cl.p)))
	c.Clues = append(c.Clues, metric("parameter-overlap", key+":mean", rounded(cl.meanP)).compare(config.Ratios["cosmetic-parameter-overlap"], ">="))
	c.Clues = append(c.Clues, metric("dependency-overlap", key+":mean", rounded(cl.meanD)).compare(config.Ratios["cosmetic-dependency-overlap"], ">="))
}
func (i *cosmeticInvestigation) clusterMember(cl *helperCluster, call *ast.CallExpr) Member {
	helper := i.engine.calls[call]
	cl.memberClues(i.finding, call, helper.symbol)
	i.finding.Receipts = append(i.finding.Receipts, cl.owner.file.receipt("helper-call", call.Pos(), call.End(), Detail{Subject: helper.symbol}))
	cl.owner.forwardingReceipts(i.finding, call)
	return Member{helper.symbol, cl.owner.file.tf.Offset(call.Pos())}
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
func (i *cosmeticInvestigation) helper(h *declaration) {
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
	v, ok := d.file.pkg.TypesInfo.Uses[id].(*types.Var)
	if !ok {
		return
	}
	if key, yes := d.inputs[v]; yes {
		c.Receipts = append(c.Receipts, d.file.receipt("parameter", id.Pos(), id.End(), Detail{Subject: key}))
	}
}
func forwardedExpressions(d *declaration, call *ast.CallExpr) []ast.Expr {
	out := append([]ast.Expr{}, call.Args...)
	if sel, ok := unparen(call.Fun).(*ast.SelectorExpr); ok {
		if s := d.file.pkg.TypesInfo.Selections[sel]; s != nil && s.Kind() == types.MethodVal {
			out = append(out, sel.X)
		}
	}
	return out
}
