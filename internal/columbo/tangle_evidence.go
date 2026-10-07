package columbo

import (
	"go/ast"
	"go/token"
	"sort"
)

type TangleReport struct {
	Version      int           `json:"version"`
	Files        int           `json:"files_analyzed"`
	Declarations int           `json:"declarations_analyzed"`
	Groups       []TangleGroup `json:"groups"`
}
type TangleGroup struct {
	ID            string       `json:"id"`
	Symbol        string       `json:"symbol"`
	Field         string       `json:"discriminant_field"`
	Values        []string     `json:"repeated_values"`
	WrittenFields []string     `json:"written_fields"`
	Sites         []TangleSite `json:"sites"`
	Lead          string       `json:"lead"`
	Limits        string       `json:"limits"`
}
type TangleSite struct {
	Condition   Source   `json:"condition"`
	Comparisons []Source `json:"comparisons"`
	Writes      []Source `json:"writes"`
}

const tangleLead = "Repeated field decisions are nested and lexically guard updates to multiple fields on the same receiver expression. Gather each behavioral category into coherent paths or methods. Preserve ordering and boundary behavior, tolerate temporary duplication, and rerun for fresh evidence before assigning ownership or deduplicating."
const tangleLimits = "Syntactic evidence, not path feasibility or proof of entanglement. Receiver paths resolve variables, direct fields and simple indexes; aliases, mutation between checks, runtime index identity and call effects are not analyzed. Writes may be nested in either branch and are not necessarily executed. Return-only validation, switches, helper-mediated updates and single-output behavior are outside this collector's scope."

func (a *engine) tangles() TangleReport {
	report := TangleReport{Version: 1, Groups: []TangleGroup{}}
	for _, f := range a.files {
		if f.included {
			report.Files++
		}
	}
	for _, d := range a.declarations {
		report.collect(d)
	}
	sort.Slice(report.Groups, func(i, j int) bool { return report.Groups[i].ID < report.Groups[j].ID })
	return report
}
func (r *TangleReport) collect(d *declaration) {
	if !d.file.included || !d.hasBody() {
		return
	}
	r.Declarations++
	r.Groups = append(r.Groups, d.tangles()...)
}

type tangleScan struct {
	owner  *declaration
	facts  tangleFacts
	groups map[string]*tangleCandidate
}
type tangleCandidate struct {
	group    TangleGroup
	receiver string
	nodes    []*ast.IfStmt
	values   map[string]int
	fields   map[string]bool
}

func (d *declaration) tangles() []TangleGroup {
	scan := tangleScan{d, tangleFacts{d.file.typeInfo()}, map[string]*tangleCandidate{}}
	ast.Inspect(d.fn.Body, scan.visit)
	return scan.finish()
}
func (s *tangleScan) visit(node ast.Node) bool {
	if _, closure := node.(*ast.FuncLit); closure {
		return false
	}
	if branch, ok := node.(*ast.IfStmt); ok {
		s.branch(branch)
	}
	return true
}
func (s *tangleScan) branch(branch *ast.IfStmt) {
	grouped := map[string][]tangleComparison{}
	for _, c := range s.facts.comparisons(branch.Cond) {
		key := canonical([]string{c.field, c.receiver})
		grouped[key] = append(grouped[key], c)
	}
	for key, comparisons := range grouped {
		s.add(key, branch, comparisons)
	}
}
func (s *tangleScan) candidate(key string, first tangleComparison) *tangleCandidate {
	if s.groups[key] == nil {
		s.groups[key] = newTangleCandidate(s.owner.symbol, first)
	}
	return s.groups[key]
}
func newTangleCandidate(symbol string, first tangleComparison) *tangleCandidate {
	group := TangleGroup{Symbol: symbol, Field: first.field, Lead: tangleLead, Limits: tangleLimits}
	return &tangleCandidate{group: group, receiver: first.receiver, values: map[string]int{}, fields: map[string]bool{}}
}
func (s *tangleScan) add(key string, branch *ast.IfStmt, comparisons []tangleComparison) {
	c := s.candidate(key, comparisons[0])
	site := s.site(branch.Cond, comparisons)
	c.count(comparisons)
	s.writes(branch, c, &site)
	c.nodes = append(c.nodes, branch)
	c.group.Sites = append(c.group.Sites, site)
}
func (c *tangleCandidate) count(comparisons []tangleComparison) {
	values := map[string]bool{}
	for _, comparison := range comparisons {
		values[comparison.value] = true
	}
	for value := range values {
		c.values[value]++
	}
}
func (s *tangleScan) site(condition ast.Expr, comparisons []tangleComparison) TangleSite {
	site := TangleSite{Condition: s.receipt("variant-condition", condition), Comparisons: []Source{}, Writes: []Source{}}
	for _, comparison := range comparisons {
		site.Comparisons = append(site.Comparisons, s.receipt("field-comparison", comparison.node))
	}
	return site
}
func (s *tangleScan) receipt(kind string, node ast.Node) Source {
	receipt := s.owner.source(kind, node.Pos(), node.End(), Detail{Subject: s.owner.symbol})
	receipt.Spelling = string(s.owner.file.data[receipt.StartOffset:receipt.EndOffset])
	return receipt
}

type tangleWrites struct {
	scan      *tangleScan
	candidate *tangleCandidate
	site      *TangleSite
}

func (s *tangleScan) writes(branch *ast.IfStmt, candidate *tangleCandidate, site *TangleSite) {
	collector := tangleWrites{s, candidate, site}
	ast.Inspect(branch.Body, collector.visit)
	if branch.Else != nil {
		ast.Inspect(branch.Else, collector.visit)
	}
}
func (w *tangleWrites) visit(node ast.Node) bool {
	if _, closure := node.(*ast.FuncLit); closure {
		return false
	}
	for _, target := range tangleTargets(node) {
		w.write(target, node)
	}
	return true
}
func tangleTargets(node ast.Node) []ast.Expr {
	if n, ok := node.(*ast.AssignStmt); ok && n.Tok != token.DEFINE {
		return n.Lhs
	}
	if n, ok := node.(*ast.IncDecStmt); ok {
		return []ast.Expr{n.X}
	}
	return nil
}
func (w *tangleWrites) write(target ast.Expr, node ast.Node) {
	field, receiver := w.scan.facts.field(target)
	if !w.candidate.accepts(field, receiver) {
		return
	}
	w.candidate.fields[field] = true
	receipt := w.scan.receipt("guarded-field-write", node)
	receipt.Detail.Subject = field
	w.site.Writes = append(w.site.Writes, receipt)
}
func (c *tangleCandidate) accepts(field, receiver string) bool {
	return field != "" && field != c.group.Field && receiver == c.receiver
}
func (s *tangleScan) finish() []TangleGroup {
	groups := []TangleGroup{}
	for _, c := range s.groups {
		if c.qualifies() {
			groups = append(groups, c.finish())
		}
	}
	return groups
}
func (c *tangleCandidate) qualifies() bool {
	repeated, guarded := 0, 0
	for _, count := range c.values {
		if count >= 2 {
			repeated++
		}
	}
	for _, site := range c.group.Sites {
		if len(site.Writes) > 0 {
			guarded++
		}
	}
	return len(c.nodes) >= 3 && repeated >= 2 && guarded >= 2 && len(c.fields) >= 2 && c.nested()
}
func (c *tangleCandidate) nested() bool {
	for i, outer := range c.nodes {
		for _, inner := range c.nodes[i+1:] {
			if tangleWithin(inner, outer.Body) || tangleElseNested(inner, outer.Else) {
				return true
			}
		}
	}
	return false
}
func (c *tangleCandidate) finish() TangleGroup {
	for value, count := range c.values {
		if count >= 2 {
			c.group.Values = append(c.group.Values, value)
		}
	}
	for field := range c.fields {
		c.group.WrittenFields = append(c.group.WrittenFields, field)
	}
	sort.Strings(c.group.Values)
	sort.Strings(c.group.WrittenFields)
	key := canonical([]any{c.group.Symbol, c.group.Field, c.group.Sites[0].Condition.File, c.group.Sites[0].Condition.StartOffset})
	c.group.ID = "TC-" + choiceSetID(key)[3:]
	return c.group
}

func tangleWithin(inner *ast.IfStmt, block *ast.BlockStmt) bool {
	return inner.Pos() > block.Pos() && inner.End() < block.End()
}
func tangleElseNested(inner *ast.IfStmt, alternative ast.Stmt) bool {
	block, ok := alternative.(*ast.BlockStmt)
	return ok && tangleWithin(inner, block)
}
