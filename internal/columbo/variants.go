package columbo

import (
	"fmt"
	"go/ast"
	"go/types"
	"sort"
)

const variantSmell = "repeated-variant-decision"

// A site owns its physical evidence. Correlation uses only typed domain and
// variant identities; branch bodies never participate.
type variantSite struct {
	node     ast.Node
	domain   string
	owner    *declaration
	info     *types.Info
	facts    variantTypeFacts
	decision Source
	arms     map[string][]Source
}

// The index owns domain correlation across all included declarations.
type variantIndex struct {
	engine  *engine
	domains map[string][]variantSite
}

func (a *engine) repeatedVariants() error {
	if a.config.Severity[variantSmell] == "off" {
		return nil
	}
	index := variantIndex{engine: a, domains: map[string][]variantSite{}}
	index.collect()
	return index.report()
}
func (index *variantIndex) collect() {
	for _, d := range index.engine.declarations {
		if d.file.included && d.fn.Body != nil {
			collectVariantSites(d, index.domains)
		}
	}
}
func (index *variantIndex) report() error {
	for _, domain := range sortedVariantDomains(index.domains) {
		if err := index.reportDomain(domain); err != nil {
			return err
		}
	}
	return nil
}
func (index *variantIndex) reportDomain(domain string) error {
	config := index.engine.config
	group := repeatedVariantGroup(index.domains[domain], int(config.Counts["repeated-variant-sites"]))
	if len(group.repeated) < int(config.Counts["repeated-variant-variants"]) {
		return nil
	}
	return index.engine.reportVariants(domain, group)
}

func sortedVariantDomains(domains map[string][]variantSite) []string {
	keys := make([]string, 0, len(domains))
	for key := range domains {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// A scanner owns declaration attribution, chain membership and its local sites.
type variantScanner struct {
	owner *declaration
	info  *types.Info
	tails map[*ast.IfStmt]bool
	sites []variantSite
}

func collectVariantSites(d *declaration, domains map[string][]variantSite) {
	scanner := variantScanner{owner: d, info: d.file.pkg.TypesInfo}
	scanner.scan(d.fn.Body)
	for _, site := range scanner.sites {
		domains[site.domain] = append(domains[site.domain], site)
	}
}
func (scanner *variantScanner) scan(body *ast.BlockStmt) {
	scanner.tails = variantChainTails(body)
	ast.Inspect(body, scanner.visit)
}
func (scanner *variantScanner) visit(node ast.Node) bool {
	site := variantSite{owner: scanner.owner, info: scanner.info, facts: variantTypeFacts{scanner.info}, arms: map[string][]Source{}}
	if site.interpret(node, scanner.tails) && len(site.arms) >= 2 {
		scanner.record(site, node)
	}
	return true
}
func (scanner *variantScanner) record(site variantSite, node ast.Node) {
	site.node = node
	site.decision = scanner.owner.source("variant-decision", node.Pos(), node.End(), Detail{Subject: site.domain})
	scanner.sites = append(scanner.sites, site)
}

func variantChainTails(body *ast.BlockStmt) map[*ast.IfStmt]bool {
	tails := map[*ast.IfStmt]bool{}
	ast.Inspect(body, func(node ast.Node) bool {
		if branch, ok := node.(*ast.IfStmt); ok {
			if next, ok := branch.Else.(*ast.IfStmt); ok {
				tails[next] = true
			}
		}
		return true
	})
	return tails
}

func (s *variantSite) interpret(node ast.Node, tails map[*ast.IfStmt]bool) bool {
	switch n := node.(type) {
	case *ast.SwitchStmt:
		return s.valueSwitch(n)
	case *ast.TypeSwitchStmt:
		return s.typeSwitch(n)
	case *ast.IfStmt:
		if !tails[n] {
			return s.ifChain(n)
		}
	}
	return false
}

func valueVariantDomain(t types.Type) string {
	if t == nil {
		return ""
	}
	named, ok := types.Unalias(t).(*types.Named)
	if !ok {
		return ""
	}
	basic, ok := named.Underlying().(*types.Basic)
	if !ok || basic.Info()&(types.IsInteger|types.IsString) == 0 {
		return ""
	}
	return "value:" + canonicalType(named, nil)
}

func (s *variantSite) valueSwitch(n *ast.SwitchStmt) bool {
	if n.Tag == nil {
		return false
	}
	s.domain = valueVariantDomain(s.info.TypeOf(n.Tag))
	return s.domain != "" && s.switchArms(n.Body, s.valueArm)
}
func (s *variantSite) switchArms(body *ast.BlockStmt, add func(ast.Expr) bool) bool {
	for _, clause := range body.List {
		if !addVariantArmList(clause.(*ast.CaseClause).List, add) {
			return false
		}
	}
	return true
}
func addVariantArmList(expressions []ast.Expr, add func(ast.Expr) bool) bool {
	for _, expr := range expressions {
		if !add(expr) {
			return false
		}
	}
	return true
}

func (s *variantSite) valueArm(expr ast.Expr) bool {
	info := s.info.Types[expr]
	if info.Value == nil {
		return false
	}
	// Successful Go type checking establishes representability at every supported
	// switch/equality. Typed constants must match; untyped constants are permitted.
	if domain := valueVariantDomain(info.Type); domain != "" && domain != s.domain {
		return false
	}
	key := s.domain + "=" + info.Value.ExactString()
	s.addArm(key, expr)
	return true
}

func (s *variantSite) addArm(key string, expr ast.Expr) {
	receipt := s.owner.source("variant-arm", expr.Pos(), expr.End(), Detail{Subject: key})
	receipt.Spelling = string(s.owner.file.data[s.owner.file.tf.Offset(expr.Pos()):s.owner.file.tf.Offset(expr.End())])
	s.arms[key] = append(s.arms[key], receipt)
}

func (s *variantSite) typeSwitch(n *ast.TypeSwitchStmt) bool {
	assertion := switchedTypeAssertion(n.Assign)
	if assertion == nil {
		return false
	}
	s.domain = s.facts.role(assertion.X)
	return s.domain != "" && s.switchArms(n.Body, s.typeArm)
}
func typeVariantDomain(t types.Type) string {
	named, ok := types.Unalias(t).(*types.Named)
	if !ok {
		return ""
	}
	if _, ok := named.Underlying().(*types.Interface); !ok {
		return ""
	}
	return "type:" + canonicalType(named, nil)
}
func (s *variantSite) typeArm(expr ast.Expr) bool {
	key, valid := s.facts.concreteKey(expr)
	if !valid {
		return false
	}
	if key != "" {
		s.addArm(s.domain+"="+key, expr)
	}
	return true
}

func switchedTypeAssertion(stmt ast.Stmt) *ast.TypeAssertExpr {
	var expr ast.Expr
	switch n := stmt.(type) {
	case *ast.AssignStmt:
		if len(n.Rhs) == 1 {
			expr = n.Rhs[0]
		}
	case *ast.ExprStmt:
		expr = n.X
	}
	assertion, _ := unparen(expr).(*ast.TypeAssertExpr)
	return assertion
}

func concreteVariantType(t types.Type) bool {
	t = types.Unalias(t)
	if pointer, ok := t.(*types.Pointer); ok {
		t = types.Unalias(pointer.Elem())
	}
	named, ok := t.(*types.Named)
	if !ok {
		return false
	}
	_, iface := named.Underlying().(*types.Interface)
	return !iface
}

type variantGroup struct {
	sites    []variantSite
	repeated []string
	support  map[string]int
}

func repeatedVariantGroup(sites []variantSite, minimum int) variantGroup {
	group := variantGroup{support: map[string]int{}}
	group.countSupport(sites)
	group.findRepeated(minimum)
	group.supportingSites(sites)
	return group
}
func (g *variantGroup) countSupport(sites []variantSite) {
	for _, site := range sites {
		for key := range site.arms {
			g.support[key]++
		}
	}
}
func (g *variantGroup) findRepeated(minimum int) {
	for key, count := range g.support {
		if count >= minimum {
			g.repeated = append(g.repeated, key)
		}
	}
	sort.Strings(g.repeated)
}
func (g *variantGroup) supportingSites(sites []variantSite) {
	for _, site := range sites {
		if site.supports(g.repeated) {
			g.sites = append(g.sites, site)
		}
	}
	sort.Slice(g.sites, func(i, j int) bool { return variantDecisionLess(g.sites[i].decision, g.sites[j].decision) })
}
func variantDecisionLess(x, y Source) bool {
	if x.File != y.File {
		return x.File < y.File
	}
	return x.StartOffset < y.StartOffset
}

func (s variantSite) supports(keys []string) bool {
	for _, key := range keys {
		if len(s.arms[key]) > 0 {
			return true
		}
	}
	return false
}

func (a *engine) reportVariants(domain string, group variantGroup) error {
	id, raw := identity(variantSmell, "", "", domain)
	if err := a.claimIdentity(id, raw); err != nil {
		return err
	}
	c := caseFromSource(variantSmell, a.config.Severity[variantSmell], group.sites[0].owner.declReceipt())
	c.ID = id
	c.Receipts = []any{}
	group.evidence(c, domain, a.config)
	a.variantRoles(c, group)
	a.report.Cases = append(a.report.Cases, *c)
	return nil
}

type variantEvidence struct {
	finding   *Case
	group     variantGroup
	decisions []Source
	arms      map[string][]Source
}

func (g variantGroup) evidence(c *Case, domain string, config Config) {
	evidence := variantEvidence{finding: c, group: g, arms: map[string][]Source{}}
	evidence.sites()
	evidence.domainClues(domain, config)
	evidence.supportClues(config.Counts["repeated-variant-sites"])
}
func (e *variantEvidence) sites() {
	for _, site := range e.group.sites {
		e.decisions = append(e.decisions, site.decision)
		e.finding.Receipts = append(e.finding.Receipts, site.decision)
		e.finding.Clues = append(e.finding.Clues, site.variantSetClue())
		e.repeatedArms(site)
	}
}
func (s variantSite) variantSetClue() Clue {
	subject := fmt.Sprintf("%s:%d", s.decision.File, s.decision.StartOffset)
	return metric("variant-set", subject, sortedVariantArms(s.arms)).forDeclaration(s.decision.Declaration).supportedBy([]Source{s.decision})
}
func (e *variantEvidence) repeatedArms(site variantSite) {
	for _, key := range e.group.repeated {
		e.arms[key] = append(e.arms[key], site.arms[key]...)
		appendSources(e.finding, site.arms[key])
	}
}
func (e *variantEvidence) domainClues(domain string, config Config) {
	e.finding.Clues = append(e.finding.Clues,
		metric("variant-decision-sites", domain, len(e.group.sites)).compare(config.Counts["repeated-variant-sites"], ">=").supportedBy(e.decisions),
		metric("repeated-variant-count", domain, len(e.group.repeated)).compare(config.Counts["repeated-variant-variants"], ">=").supportedBy(e.decisions),
		metric("repeated-variant-set", domain, e.group.repeated).supportedBy(e.decisions))
}
func (e *variantEvidence) supportClues(minimum int64) {
	for _, key := range e.group.repeated {
		e.finding.Clues = append(e.finding.Clues, metric("variant-support", key, e.group.support[key]).compare(minimum, ">=").supportedBy(e.arms[key]))
	}
}

func sortedVariantArms(arms map[string][]Source) []string {
	keys := make([]string, 0, len(arms))
	for key := range arms {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
