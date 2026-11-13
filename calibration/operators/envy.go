package operators

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"sort"
	"strings"

	"github.com/KellyBennett/Columbo/calibration/harness"
)

// accessGroup keys a receiver value as Columbo's feature-envy rule does.
type accessGroup struct {
	root   *types.Var
	path   string
	count  int
	first  token.Pos
	sample *ast.SelectorExpr
}

func (g *accessGroup) bare() bool { return g.path == "" }

// envyProfile reimplements Columbo's feature-envy counting (SPEC "Feature Envy").
type envyProfile struct {
	own    int
	groups []*accessGroup
}

func profileOf(info *types.Info, body *ast.BlockStmt, own *types.Var) envyProfile {
	var p envyProfile
	byKey := map[string]*accessGroup{}
	ast.Inspect(body, func(n ast.Node) bool {
		if _, lit := n.(*ast.FuncLit); lit {
			return false
		}
		s, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		sel := info.Selections[s]
		if sel == nil || sel.Kind() == types.MethodExpr {
			return true
		}
		root, path, ok := valuePath(info, s.X)
		if !ok {
			return true
		}
		if root == own && path == "" {
			p.own++
			return true
		}
		if !named(info.TypeOf(s.X)) {
			return true
		}
		key := fmt.Sprintf("%p%s", root, path)
		g := byKey[key]
		if g == nil {
			g = &accessGroup{root: root, path: path, first: s.Pos(), sample: s}
			byKey[key] = g
			p.groups = append(p.groups, g)
		}
		g.count++
		return true
	})
	return p
}

func valuePath(info *types.Info, e ast.Expr) (*types.Var, string, bool) {
	for {
		switch x := e.(type) {
		case *ast.ParenExpr:
			e = x.X
			continue
		case *ast.StarExpr:
			e = x.X
			continue
		case *ast.UnaryExpr:
			if x.Op == token.AND {
				e = x.X
				continue
			}
		}
		break
	}
	switch x := e.(type) {
	case *ast.Ident:
		v, ok := info.ObjectOf(x).(*types.Var)
		return v, "", ok && v != nil
	case *ast.SelectorExpr:
		sel := info.Selections[x]
		if sel == nil || sel.Kind() != types.FieldVal {
			return nil, "", false
		}
		root, path, ok := valuePath(info, x.X)
		return root, path + "." + sel.Obj().Name(), ok
	}
	return nil, "", false
}

func named(t types.Type) bool {
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	_, ok := types.Unalias(t).(*types.Named)
	return ok
}

type envyLimits struct {
	foreign int
	ratio   float64
}

func limitsOf(c *harness.Corpus) envyLimits {
	return envyLimits{
		foreign: int(threshold(c, "feature-envy-foreign-accesses", 5)),
		ratio:   threshold(c, "feature-envy-ratio", 2.0),
	}
}

func (l envyLimits) qualifies(g *accessGroup, own int) bool {
	return g.count >= l.foreign && (own == 0 || float64(g.count) >= l.ratio*float64(own))
}

func (p envyProfile) qualifying(l envyLimits) []*accessGroup {
	var out []*accessGroup
	for _, g := range p.groups {
		if l.qualifies(g, p.own) {
			out = append(out, g)
		}
	}
	return out
}

func (p envyProfile) group(root *types.Var) *accessGroup {
	for _, g := range p.groups {
		if g.root == root && g.bare() {
			return g
		}
	}
	return nil
}

func (p envyProfile) describe(l envyLimits) string {
	parts := []string{fmt.Sprintf("own %d", p.own)}
	for _, g := range p.groups {
		if g.count >= l.foreign-1 {
			parts = append(parts, fmt.Sprintf("%s%s %d", g.root.Name(), g.path, g.count))
		}
	}
	return strings.Join(parts, ", ") + fmt.Sprintf(" (limit %d, ratio %.1f)", l.foreign, l.ratio)
}

func predictEnvy(p envyProfile, l envyLimits, what string) harness.Prediction {
	kill := len(p.qualifying(l)) > 0
	return harness.Prediction{Kill: &kill, Reason: what + ": " + p.describe(l)}
}

func recvVarOf(f fn) *types.Var {
	if f.decl.Recv == nil {
		return nil
	}
	return recvVar(f)
}

func localNamed(t types.Type, pkg *types.Package) (*types.Named, bool) {
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	n, ok := t.(*types.Named)
	if !ok || n.Obj().Pkg() != pkg || n.TypeParams().Len() > 0 {
		return nil, false
	}
	switch n.Underlying().(type) {
	case *types.Interface, *types.Pointer:
		return nil, false
	}
	return n, true
}

func sortedGroups(gs []*accessGroup) {
	sort.Slice(gs, func(i, j int) bool { return gs[i].first < gs[j].first })
}
