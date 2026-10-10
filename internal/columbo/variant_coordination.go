package columbo

import (
	"go/ast"
	"go/token"
	"go/types"
)

type coordinationDecision struct {
	node       ast.Node
	selector   resolvedValuePath
	partitions []string
	writes     map[string]Source
	reads      map[string]Source
}

func (a *engine) variantCaseEvidence(c *Case, domain string, group variantGroup) {
	group.evidence(c, domain, a.config)
	a.variantRoles(c, group)
	a.variantCoordination(c, domain)
}

type coordinationScan struct {
	owner     *declaration
	domain    string
	info      *types.Info
	decisions []coordinationDecision
	parents   []coordinationDecision
}

func (a *engine) variantCoordination(c *Case, domain string) {
	for _, d := range a.declarations {
		if d.file.included && d.hasBody() {
			s := coordinationScan{owner: d, domain: domain, info: d.file.typeInfo()}
			ast.Inspect(d.fn.Body, s.visit)
			s.evidence(c)
		}
	}
}

func (s *coordinationScan) visit(node ast.Node) bool {
	if node == nil || coordinationClosure(node) {
		return false
	}
	s.leaveParents(node.Pos())
	d, ok := s.decision(node)
	if ok && !s.refines(d) {
		s.record(d)
	}
	return true
}
func coordinationClosure(node ast.Node) bool {
	_, ok := node.(*ast.FuncLit)
	return ok
}
func (s *coordinationScan) leaveParents(pos token.Pos) {
	for len(s.parents) > 0 && pos >= s.parents[len(s.parents)-1].node.End() {
		s.parents = s.parents[:len(s.parents)-1]
	}
}
func (s *coordinationScan) refines(d coordinationDecision) bool {
	for _, parent := range s.parents {
		if d.selector.same(parent.selector) {
			return true
		}
	}
	return false
}
func (s *coordinationScan) record(d coordinationDecision) {
	e := coordinationEffects{owner: s.owner, info: s.info, writes: map[string]Source{}, reads: map[string]Source{}}
	rootInitializer := d.initializer()
	ast.Inspect(d.node, func(node ast.Node) bool {
		return node != rootInitializer && e.visit(node)
	})
	d.writes, d.reads = e.writes, e.reads
	s.decisions = append(s.decisions, d)
	s.parents = append(s.parents, d)
}
func (d coordinationDecision) initializer() ast.Stmt {
	switch n := d.node.(type) {
	case *ast.IfStmt:
		return n.Init
	case *ast.SwitchStmt:
		return n.Init
	}
	return nil
}

func (s *coordinationScan) decision(node ast.Node) (coordinationDecision, bool) {
	d := coordinationDecision{node: node}
	p := coordinationPredicate{info: s.info, domain: s.domain, decision: &d}
	ok := p.interpret(node)
	return d, ok
}

type coordinationPredicate struct {
	info     *types.Info
	domain   string
	decision *coordinationDecision
}

func (p *coordinationPredicate) interpret(node ast.Node) bool {
	switch n := node.(type) {
	case *ast.IfStmt:
		return p.condition(n.Cond)
	case *ast.SwitchStmt:
		return p.valueSwitch(n)
	}
	return false
}
func (p *coordinationPredicate) valueSwitch(n *ast.SwitchStmt) bool {
	if n.Tag == nil || !p.selector(n.Tag) {
		return false
	}
	for _, stmt := range n.Body.List {
		if !p.caseValues(stmt.(*ast.CaseClause).List) {
			return false
		}
	}
	return len(p.decision.partitions) > 0
}
func (p *coordinationPredicate) caseValues(values []ast.Expr) bool {
	for _, expr := range values {
		if !p.constant(expr, "==") {
			return false
		}
	}
	return true
}
func (p *coordinationPredicate) constant(expr ast.Expr, op string) bool {
	v := p.info.Types[expr].Value
	if v == nil {
		return false
	}
	p.decision.partitions = append(p.decision.partitions, op+v.ExactString())
	return true
}
func (p *coordinationPredicate) condition(expr ast.Expr) bool {
	return walkBooleanLeaves(expr, p.conditionLeaf)
}
func (p *coordinationPredicate) conditionLeaf(expr ast.Expr) bool {
	n, ok := ast.Unparen(expr).(*ast.BinaryExpr)
	if !ok {
		return false
	}
	return p.comparison(n)
}
func (p *coordinationPredicate) comparison(n *ast.BinaryExpr) bool {
	if n.Op != token.EQL && n.Op != token.NEQ {
		return false
	}
	facts := variantSite{info: p.info}
	value, constant := facts.equalityOperands(n)
	return p.selector(value) && p.constant(constant, n.Op.String())
}
func (p *coordinationPredicate) selector(expr ast.Expr) bool {
	if valueVariantDomain(p.info.TypeOf(expr)) != p.domain {
		return false
	}
	path := resolveValuePath(p.info, expr, ast.Unparen)
	return p.decision.acceptSelector(path)
}
func (d *coordinationDecision) acceptSelector(path resolvedValuePath) bool {
	if !path.valid() || (d.selector.valid() && !path.same(d.selector)) {
		return false
	}
	d.selector = path
	return true
}
