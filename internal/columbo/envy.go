package columbo

import (
	"go/ast"
	"go/types"
	"sort"
)

type accessGroup struct {
	key      string
	receipts []Source
	first    int
}

func stableValue(d *declaration, e ast.Expr) (*types.Var, string, bool) {
	e = unparen(e)
	switch e := e.(type) {
	case *ast.StarExpr:
		return stableValue(d, e.X)
	case *ast.UnaryExpr:
		if e.Op.String() == "&" {
			return stableValue(d, e.X)
		}
	case *ast.Ident:
		v, ok := d.file.pkg.TypesInfo.Uses[e].(*types.Var)
		if ok {
			return v, d.variable(v), true
		}
	case *ast.SelectorExpr:
		sel := d.file.pkg.TypesInfo.Selections[e]
		if sel == nil || sel.Kind() != types.FieldVal {
			return nil, "", false
		}
		v, k, ok := stableValue(d, e.X)
		return v, k + "." + e.Sel.Name, ok
	}
	return nil, "", false
}

type accessCollection struct {
	own         []Source
	groups      map[string]*accessGroup
	declaration *declaration
}

func (a *engine) envy(d *declaration) error {
	if d.signature.Recv() == nil || d.fn.Body == nil {
		return nil
	}
	accesses := d.accesses()
	qualifying := accesses.qualifying(a.config)
	if len(qualifying) == 0 {
		return nil
	}
	c, e := a.newCase(d, "feature-envy", "")
	if e != nil || c == nil {
		return e
	}
	c.value("own-accesses", len(accesses.own))
	appendSources(c, accesses.own)
	for _, g := range qualifying {
		g.clues(c, len(accesses.own), a.config)
	}
	a.report.Cases = append(a.report.Cases, *c)
	return nil
}
func (d *declaration) accesses() *accessCollection {
	c := &accessCollection{declaration: d, own: []Source{}, groups: map[string]*accessGroup{}}
	ast.Inspect(d.fn.Body, c.visit)
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
func (c *accessCollection) selector(s *ast.SelectorExpr) {
	sel := c.declaration.file.pkg.TypesInfo.Selections[s]
	if sel == nil || sel.Kind() == types.MethodExpr {
		return
	}
	v, key, ok := stableValue(c.declaration, s.X)
	if !ok {
		return
	}
	if c.declaration.isOwnValue(v, key) {
		c.own = append(c.own, c.declaration.accessReceipt("own-access", s, key))
		return
	}
	if isNamedType(c.declaration.file.pkg.TypesInfo.TypeOf(s.X)) {
		c.foreign(s, key)
	}
}
func isNamedType(t types.Type) bool { _, ok := stripPointer(t).(*types.Named); return ok }
func (d *declaration) isOwnValue(v *types.Var, key string) bool {
	return v == d.signature.Recv() && key == d.variable(v)
}
func (d *declaration) accessReceipt(kind string, s *ast.SelectorExpr, key string) Source {
	return d.file.receipt(kind, s.Pos(), s.End(), Detail{Subject: key})
}
func (c *accessCollection) foreign(s *ast.SelectorExpr, key string) {
	g := c.groups[key]
	if g == nil {
		g = &accessGroup{key: key, first: c.declaration.file.tf.Offset(s.Pos())}
		c.groups[key] = g
	}
	g.receipts = append(g.receipts, c.declaration.accessReceipt("foreign-access", s, key))
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
func (g *accessGroup) clues(c *Case, own int, config Config) {
	c.Clues = append(c.Clues, metric("foreign-accesses", g.key, len(g.receipts)).compare(config.Counts["feature-envy-foreign-accesses"], ">="))
	g.ratioClue(c, own, config.Ratios["feature-envy-ratio"])
	appendSources(c, g.receipts)
}
func (g *accessGroup) ratioClue(c *Case, own int, limit float64) {
	if own == 0 {
		c.Clues = append(c.Clues, metric("foreign-own-ratio", g.key, 0))
		return
	}
	c.Clues = append(c.Clues, metric("foreign-own-ratio", g.key, rounded(fraction(len(g.receipts), own))).compare(limit, ">="))
}
