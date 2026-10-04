package columbo

import (
	"go/ast"
	"go/token"
	"go/types"
	"sort"
)

type accessGroup struct {
	key      string
	receipts []Source
	first    int
}

// A stable value is a variable plus a field path. Calls, indexes, package
// selectors and method expressions never acquire a stable variable identity.
type valueResolver struct {
	declaration *declaration
	info        *types.Info
}

func stableValue(d *declaration, expr ast.Expr) (*types.Var, string, bool) {
	resolver := &valueResolver{d, d.file.typeInfo()}
	return resolver.resolve(expr)
}
func valueBase(expr ast.Expr) ast.Expr {
	expr = ast.Unparen(expr)
	switch node := expr.(type) {
	case *ast.StarExpr:
		return valueBase(node.X)
	case *ast.UnaryExpr:
		if node.Op == token.AND {
			return valueBase(node.X)
		}
	}
	return expr
}
func (r *valueResolver) resolve(expr ast.Expr) (*types.Var, string, bool) {
	switch node := valueBase(expr).(type) {
	case *ast.Ident:
		return r.identifier(node)
	case *ast.SelectorExpr:
		return r.field(node)
	}
	return nil, "", false
}
func (r *valueResolver) identifier(id *ast.Ident) (*types.Var, string, bool) {
	variable, ok := r.info.Uses[id].(*types.Var)
	if !ok {
		return nil, "", false
	}
	return variable, r.declaration.variable(variable), true
}
func (r *valueResolver) fieldSelection(expr *ast.SelectorExpr) bool {
	selection := r.info.Selections[expr]
	return selection != nil && selection.Kind() == types.FieldVal
}
func (r *valueResolver) field(expr *ast.SelectorExpr) (*types.Var, string, bool) {
	if !r.fieldSelection(expr) {
		return nil, "", false
	}
	variable, key, ok := r.resolve(expr.X)
	return variable, key + "." + expr.Sel.Name, ok
}

// valueAccess binds a selected member to the stable value that owns it.
type valueAccess struct {
	declaration *declaration
	selector    *ast.SelectorExpr
	root        *types.Var
	key         string
}

func (d *declaration) selection(expr *ast.SelectorExpr) *types.Selection {
	return d.file.typeInfo().Selections[expr]
}
func (d *declaration) expressionType(expr ast.Expr) types.Type {
	return d.file.typeInfo().TypeOf(expr)
}
func (d *declaration) valueAccess(expr *ast.SelectorExpr) (valueAccess, bool) {
	selection := d.selection(expr)
	if selection == nil || selection.Kind() == types.MethodExpr {
		return valueAccess{}, false
	}
	root, key, ok := stableValue(d, expr.X)
	if !ok {
		return valueAccess{}, false
	}
	return valueAccess{d, expr, root, key}, true
}
func (v valueAccess) own() bool { return v.declaration.isOwnValue(v.root, v.key) }
func (v valueAccess) namedReceiver() bool {
	return isNamedType(v.declaration.expressionType(v.selector.X))
}
func (v valueAccess) receipt(kind string) Source {
	return v.declaration.accessReceipt(kind, v.selector, v.key)
}

type accessCollection struct {
	own         []Source
	groups      map[string]*accessGroup
	declaration *declaration
}

func (a *engine) envy(d *declaration) error {
	if !d.hasReceiver() || !d.hasBody() {
		return nil
	}
	c, err := d.accesses().caseFor(a)
	if err != nil || c == nil {
		return err
	}
	a.report.Cases = append(a.report.Cases, *c)
	return nil
}
func (c *accessCollection) caseFor(a *engine) (*Case, error) {
	qualifying := c.qualifying(a.config)
	if len(qualifying) == 0 {
		return nil, nil
	}
	report, err := a.newCase(c.declaration, "feature-envy", "")
	if err != nil || report == nil {
		return report, err
	}
	c.evidence(report, qualifying, a.config)
	return report, nil
}
func (c *accessCollection) evidence(report *Case, groups []*accessGroup, config Config) {
	own := report.metric("own-accesses", len(c.own)).supportedBy(c.own)
	report.Clues = append(report.Clues, own)
	appendSources(report, c.own)
	for _, group := range groups {
		group.clues(report, c.own, config)
	}
}
func (d *declaration) accesses() *accessCollection {
	c := &accessCollection{declaration: d, own: []Source{}, groups: map[string]*accessGroup{}}
	d.inspectBody(c.visit)
	return c
}
func (c *accessCollection) visit(n ast.Node) bool {
	if _, ok := n.(*ast.FuncLit); ok {
		return false
	}
	if s, ok := n.(*ast.SelectorExpr); ok {
		c.selector(s)
	}
	return true
}
func (c *accessCollection) selector(selector *ast.SelectorExpr) {
	access, ok := c.declaration.valueAccess(selector)
	if !ok {
		return
	}
	if access.own() {
		c.own = append(c.own, access.receipt("own-access"))
		return
	}
	if access.namedReceiver() {
		c.foreign(access)
	}
}
func isNamedType(t types.Type) bool { _, ok := stripPointer(t).(*types.Named); return ok }
func (d *declaration) isOwnValue(v *types.Var, key string) bool {
	return v == d.signature.Recv() && key == d.variable(v)
}
func (d *declaration) accessReceipt(kind string, s *ast.SelectorExpr, key string) Source {
	return d.source(kind, s.Pos(), s.End(), Detail{Subject: key})
}
func (c *accessCollection) foreign(access valueAccess) {
	receipt := access.receipt("foreign-access")
	group := c.groups[access.key]
	if group == nil {
		group = &accessGroup{key: access.key, first: receipt.StartOffset}
		c.groups[access.key] = group
	}
	group.receipts = append(group.receipts, receipt)
}
func (c *accessCollection) qualifying(config Config) []*accessGroup {
	out := []*accessGroup{}
	for _, g := range c.groups {
		if g.qualifies(len(c.own), config) {
			out = append(out, g)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].first != out[j].first {
			return out[i].first < out[j].first
		}
		return out[i].key < out[j].key
	})
	return out
}
func (g *accessGroup) qualifies(own int, c Config) bool {
	return int64(len(g.receipts)) >= c.Counts["feature-envy-foreign-accesses"] && (own == 0 || meets(fraction(len(g.receipts), own), c.Ratios["feature-envy-ratio"]))
}
func (g *accessGroup) clues(c *Case, own []Source, config Config) {
	foreign := metric("foreign-accesses", g.key, len(g.receipts)).compare(config.Counts["feature-envy-foreign-accesses"], ">=").forDeclaration(c.PrimaryDeclaration).supportedBy(g.receipts)
	sources := append(append([]Source{}, g.receipts...), own...)
	ratio := g.ratioClue(len(own), config.Ratios["feature-envy-ratio"]).forDeclaration(c.PrimaryDeclaration).supportedBy(sources)
	c.Clues = append(c.Clues, foreign, ratio)
	appendSources(c, g.receipts)
}
func (g *accessGroup) ratioClue(own int, limit float64) Clue {
	if own == 0 {
		return metric("foreign-own-ratio", g.key, 0)
	}
	return metric("foreign-own-ratio", g.key, rounded(fraction(len(g.receipts), own))).compare(limit, ">=")
}
