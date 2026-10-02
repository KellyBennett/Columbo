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
func (a *engine) envy(d *declaration) error {
	if d.signature.Recv() == nil || d.fn.Body == nil {
		return nil
	}
	own := []Source{}
	groups := map[string]*accessGroup{}
	ast.Inspect(d.fn.Body, func(n ast.Node) bool {
		if _, ok := n.(*ast.FuncLit); ok {
			return false
		}
		s, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		sel := d.file.pkg.TypesInfo.Selections[s]
		if sel == nil || sel.Kind() == types.MethodExpr {
			return true
		}
		v, key, ok := stableValue(d, s.X)
		if !ok {
			return true
		}
		if v == d.signature.Recv() && key == d.variable(v) {
			own = append(own, d.file.receipt("own-access", s.Pos(), s.End(), key, nil, nil))
			return true
		}
		if _, ok := stripPointer(d.file.pkg.TypesInfo.TypeOf(s.X)).(*types.Named); !ok {
			return true
		}
		g := groups[key]
		if g == nil {
			g = &accessGroup{key: key, first: d.file.tf.Offset(s.Pos())}
			groups[key] = g
		}
		g.receipts = append(g.receipts, d.file.receipt("foreign-access", s.Pos(), s.End(), key, nil, nil))
		return true
	})
	qual := []*accessGroup{}
	for _, g := range groups {
		if int64(len(g.receipts)) >= a.config.Counts["feature-envy-foreign-accesses"] && (len(own) == 0 || meets(fraction(len(g.receipts), len(own)), a.config.Ratios["feature-envy-ratio"])) {
			qual = append(qual, g)
		}
	}
	if len(qual) == 0 {
		return nil
	}
	sort.Slice(qual, func(i, j int) bool {
		if qual[i].first != qual[j].first {
			return qual[i].first < qual[j].first
		}
		return qual[i].key < qual[j].key
	})
	c, e := a.newCase(d, "feature-envy", "")
	if e != nil || c == nil {
		return e
	}
	c.Clues = append(c.Clues, metric("own-accesses", d.symbol, len(own), nil, nil))
	appendSources(c, own)
	for _, g := range qual {
		c.Clues = append(c.Clues, metric("foreign-accesses", g.key, len(g.receipts), a.config.Counts["feature-envy-foreign-accesses"], ">="))
		if len(own) == 0 {
			c.Clues = append(c.Clues, metric("foreign-own-ratio", g.key, 0, nil, nil))
		} else {
			c.Clues = append(c.Clues, metric("foreign-own-ratio", g.key, rounded(fraction(len(g.receipts), len(own))), a.config.Ratios["feature-envy-ratio"], ">="))
		}
		appendSources(c, g.receipts)
	}
	a.report.Cases = append(a.report.Cases, *c)
	return nil
}
