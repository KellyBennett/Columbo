package operators

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"sort"

	"github.com/KellyBennett/Columbo/calibration/harness"
)

// variantSite reimplements Columbo's repeated-variant-decision trigger from the outside.
type variantSite struct {
	fn     fn
	node   ast.Node
	domain string
	named  *types.Named
	arms   []variantArm
}

type variantArm struct {
	key  string
	expr ast.Expr
}

func (s *variantSite) add(key string, e ast.Expr) {
	for _, a := range s.arms {
		if a.key == key {
			return
		}
	}
	s.arms = append(s.arms, variantArm{key, e})
}

func (s variantSite) has(key string) bool {
	for _, a := range s.arms {
		if a.key == key {
			return true
		}
	}
	return false
}

func variantSites(c *harness.Corpus) []variantSite {
	var out []variantSite
	for _, f := range funcs(c) {
		out = append(out, variantSitesIn(f)...)
	}
	return out
}

func variantSitesIn(f fn) []variantSite {
	if f.decl.Body == nil {
		return nil
	}
	tails := map[*ast.IfStmt]bool{}
	ast.Inspect(f.decl.Body, func(n ast.Node) bool {
		if i, ok := n.(*ast.IfStmt); ok {
			if next, ok := i.Else.(*ast.IfStmt); ok {
				tails[next] = true
			}
		}
		return true
	})
	var out []variantSite
	ast.Inspect(f.decl.Body, func(n ast.Node) bool {
		s := variantSite{fn: f, node: n}
		ok := false
		switch n := n.(type) {
		case *ast.SwitchStmt:
			ok = s.valueSwitch(n)
		case *ast.TypeSwitchStmt:
			ok = s.typeSwitch(n)
		case *ast.IfStmt:
			ok = !tails[n] && s.ifChain(n)
		}
		if ok && len(s.arms) >= 2 {
			out = append(out, s)
		}
		return true
	})
	return out
}

func valueDomain(t types.Type) *types.Named {
	named, ok := types.Unalias(t).(*types.Named)
	if !ok {
		return nil
	}
	basic, ok := named.Underlying().(*types.Basic)
	if !ok || basic.Info()&(types.IsInteger|types.IsString) == 0 {
		return nil
	}
	return named
}

func (s *variantSite) setDomain(prefix string, named *types.Named) {
	s.named, s.domain = named, prefix+types.TypeString(named, nil)
}

func (s *variantSite) valueSwitch(n *ast.SwitchStmt) bool {
	info := s.fn.pkg.TypesInfo
	if n.Tag == nil {
		return false
	}
	named := valueDomain(info.TypeOf(n.Tag))
	if named == nil {
		return false
	}
	s.setDomain("value:", named)
	for _, cl := range n.Body.List {
		for _, e := range cl.(*ast.CaseClause).List {
			if !s.valueArm(e) {
				return false
			}
		}
	}
	return true
}

func (s *variantSite) valueArm(e ast.Expr) bool {
	tv := s.fn.pkg.TypesInfo.Types[e]
	if tv.Value == nil {
		return false
	}
	if d := valueDomain(tv.Type); d != nil && "value:"+types.TypeString(d, nil) != s.domain {
		return false
	}
	s.add(s.domain+"="+tv.Value.ExactString(), e)
	return true
}

func (s *variantSite) typeSwitch(n *ast.TypeSwitchStmt) bool {
	info := s.fn.pkg.TypesInfo
	var x ast.Expr
	switch a := n.Assign.(type) {
	case *ast.AssignStmt:
		if len(a.Rhs) == 1 {
			x = a.Rhs[0]
		}
	case *ast.ExprStmt:
		x = a.X
	}
	ta, ok := ast.Unparen(x).(*ast.TypeAssertExpr)
	if !ok {
		return false
	}
	named, ok := types.Unalias(info.TypeOf(ta.X)).(*types.Named)
	if !ok {
		return false
	}
	if _, iface := named.Underlying().(*types.Interface); !iface {
		return false
	}
	s.setDomain("type:", named)
	for _, cl := range n.Body.List {
		for _, e := range cl.(*ast.CaseClause).List {
			t := info.TypeOf(e)
			if b, ok := t.(*types.Basic); ok && b.Kind() == types.UntypedNil {
				continue
			}
			if !concreteNamed(t) {
				return false
			}
			s.add(s.domain+"="+types.TypeString(t, nil), e)
		}
	}
	return true
}

func concreteNamed(t types.Type) bool {
	t = types.Unalias(t)
	if p, ok := t.(*types.Pointer); ok {
		t = types.Unalias(p.Elem())
	}
	named, ok := t.(*types.Named)
	if !ok {
		return false
	}
	_, iface := named.Underlying().(*types.Interface)
	return !iface
}

func (s *variantSite) ifChain(first *ast.IfStmt) bool {
	var root types.Object
	var path string
	var parse func(e ast.Expr) bool
	parse = func(e ast.Expr) bool {
		b, ok := ast.Unparen(e).(*ast.BinaryExpr)
		if !ok {
			return false
		}
		if b.Op == token.LOR {
			return parse(b.X) && parse(b.Y)
		}
		if b.Op != token.EQL {
			return false
		}
		disc, constant := b.X, b.Y
		if s.fn.pkg.TypesInfo.Types[constant].Value == nil {
			disc, constant = constant, disc
		}
		r, p, ok := variantValuePath(s.fn.pkg.TypesInfo, disc)
		named := valueDomain(s.fn.pkg.TypesInfo.TypeOf(disc))
		if !ok || named == nil {
			return false
		}
		if root == nil {
			root, path = r, p
			s.setDomain("value:", named)
		}
		if r != root || p != path || "value:"+types.TypeString(named, nil) != s.domain {
			return false
		}
		return s.valueArm(constant)
	}
	for branch := first; branch != nil; branch, _ = branch.Else.(*ast.IfStmt) {
		if !parse(branch.Cond) {
			return false
		}
	}
	return true
}

func variantValuePath(info *types.Info, e ast.Expr) (types.Object, string, bool) {
	switch e := ast.Unparen(e).(type) {
	case *ast.Ident:
		v, ok := info.ObjectOf(e).(*types.Var)
		return v, "", ok
	case *ast.StarExpr:
		return variantValuePath(info, e.X)
	case *ast.SelectorExpr:
		sel := info.Selections[e]
		if sel == nil || sel.Kind() != types.FieldVal {
			return nil, "", false
		}
		r, p, ok := variantValuePath(info, e.X)
		return r, p + "." + e.Sel.Name, ok
	}
	return nil, "", false
}

func variantDomains(c *harness.Corpus) map[string][]variantSite {
	out := map[string][]variantSite{}
	for _, s := range variantSites(c) {
		out[s.domain] = append(out[s.domain], s)
	}
	return out
}

func repeatedVariants(c *harness.Corpus, sites []variantSite) (repeated []string, fires bool) {
	minSites := int(threshold(c, "repeated-variant-sites", 2))
	minVariants := int(threshold(c, "repeated-variant-variants", 2))
	support := map[string]int{}
	for _, s := range sites {
		for _, a := range s.arms {
			support[a.key]++
		}
	}
	for k, n := range support {
		if n >= minSites {
			repeated = append(repeated, k)
		}
	}
	sort.Strings(repeated)
	return repeated, len(repeated) >= minVariants
}

func siteSymbols(sites []variantSite) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range sites {
		if !seen[s.fn.symbol] {
			seen[s.fn.symbol] = true
			out = append(out, s.fn.symbol)
		}
	}
	return out
}

func domainTypeText(f fn, named *types.Named) (string, bool) {
	obj := named.Obj()
	if obj.Pkg() != nil && obj.Pkg() != f.pkg.Types && !obj.Exported() {
		return "", false
	}
	if named.TypeArgs() != nil || named.TypeParams() != nil {
		return "", false
	}
	return types.TypeString(named, qualifier(f.unit)), true
}

func uniqueFuncName(f fn, base string) string {
	name := base
	for n := 2; taken(f, nil, name); n++ {
		name = fmt.Sprintf("%s%d", base, n)
	}
	return name
}
