package operators

import (
	"errors"
	"fmt"
	"go/ast"
	"go/types"
	"sort"
	"strings"

	"github.com/KellyBennett/Columbo/calibration/harness"
)

// Only type-switch domains qualify: a value enum has no types to carry the behavior, and replacing it with a hierarchy is not mechanical.
type replaceConditionalWithPolymorphism struct{}

func (replaceConditionalWithPolymorphism) ID() string           { return "replace-conditional-with-polymorphism" }
func (replaceConditionalWithPolymorphism) Class() harness.Class { return harness.Cure }
func (replaceConditionalWithPolymorphism) Target() string       { return variantSmell }

func curableDomains(c *harness.Corpus) map[string][]variantSite {
	out := map[string][]variantSite{}
	for domain, sites := range variantDomains(c) {
		if _, fires := repeatedVariants(c, sites); !fires || !baselineAny(c, variantSmell, siteSymbols(sites)) {
			continue
		}
		for _, s := range sites {
			if _, err := s.polymorphicPlan(); err == nil {
				out[domain] = append(out[domain], s)
			}
		}
	}
	return out
}

func baselineAny(c *harness.Corpus, smell string, symbols []string) bool {
	for _, f := range c.Baseline.Findings {
		for _, s := range symbols {
			if f.Smell == smell && f.Symbol == s {
				return true
			}
		}
	}
	return false
}

func (replaceConditionalWithPolymorphism) Sites(c *harness.Corpus) []harness.Site {
	var out []harness.Site
	for _, sites := range curableDomains(c) {
		out = append(out, sites[0].site())
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Symbol+out[i].File < out[j].Symbol+out[j].File })
	return out
}

func (replaceConditionalWithPolymorphism) Doses(c *harness.Corpus, site harness.Site) []int {
	s, err := findVariantSite(c, site)
	if err != nil {
		return nil
	}
	var doses []int
	for k := range curableDomains(c)[s.domain] {
		doses = append(doses, k+1)
	}
	return doses
}

func (o replaceConditionalWithPolymorphism) Apply(c *harness.Corpus, site harness.Site, dose int) (harness.Applied, error) {
	s, err := findVariantSite(c, site)
	if err != nil {
		return harness.Applied{}, err
	}
	all := variantDomains(c)[s.domain]
	curable := curableDomains(c)[s.domain]
	if dose > len(curable) {
		return harness.Applied{}, errors.New("not enough curable decisions")
	}
	srcs := map[string][]byte{}
	paths := map[string]string{}
	splices := map[string][]splice{}
	touched := siteSymbols(all)
	cured := map[ast.Node]bool{}
	for _, cs := range curable[:dose] {
		p, err := cs.polymorphicPlan()
		if err != nil {
			return harness.Applied{}, err
		}
		srcs[cs.fn.rel], paths[cs.fn.rel] = cs.fn.src, cs.fn.path
		splices[cs.fn.rel] = append(splices[cs.fn.rel], p.splices...)
		touched = append(touched, p.methods...)
		cured[cs.node] = true
	}
	edit := harness.Edit{}
	for rel, ss := range splices {
		out, err := finish(paths[rel], applySplices(srcs[rel], ss))
		if err != nil {
			return harness.Applied{}, err
		}
		edit[rel] = out
	}
	var left []variantSite
	for _, cs := range all {
		if !cured[cs.node] {
			left = append(left, cs)
		}
	}
	repeated, fires := repeatedVariants(c, left)
	kill := !fires
	return harness.Applied{
		Edit: edit,
		Expectation: harness.Expectation{
			Touched: touched,
			Prediction: harness.Prediction{Kill: &kill, Reason: fmt.Sprintf("%d decisions left with %d repeated variants (limits %d sites, %d variants)",
				len(left), len(repeated), int(threshold(c, "repeated-variant-sites", 2)), int(threshold(c, "repeated-variant-variants", 2)))},
		},
	}, nil
}

type polymorphicPlan struct {
	splices []splice
	methods []string
}

// Pointer cases keep dispatch exact: a value-receiver method would also match the pointer type the switch sent to its default.
func (s variantSite) polymorphicPlan() (polymorphicPlan, error) {
	f := s.fn
	n, ok := s.node.(*ast.TypeSwitchStmt)
	d := f.decl
	if !ok || len(d.Body.List) == 0 || d.Body.List[0] != n || n.Init != nil || f.obj == nil || d.Type.TypeParams != nil || namedResults(d) {
		return polymorphicPlan{}, errors.New("not an opening type switch")
	}
	sig := f.obj.Type().(*types.Signature)
	if sig.Recv() != nil && isGenericRecv(sig) {
		return polymorphicPlan{}, errors.New("generic receiver")
	}
	bound, x := typeSwitchParts(n)
	if _, isIdent := x.(*ast.Ident); !isIdent {
		return polymorphicPlan{}, errors.New("switch over an expression")
	}
	info := f.pkg.TypesInfo
	trailing := len(d.Body.List) > 1
	mustReturn := trailing || sig.Results().Len() > 0
	var arms []*ast.CaseClause
	var deflt *ast.CaseClause
	for _, st := range n.Body.List {
		cc := st.(*ast.CaseClause)
		if cc.List == nil {
			deflt = cc
			continue
		}
		if len(cc.List) != 1 || len(cc.Body) == 0 {
			return polymorphicPlan{}, errors.New("multi-type or empty case")
		}
		if !localPointerType(f, info.TypeOf(cc.List[0])) {
			return polymorphicPlan{}, errors.New("case is not a pointer to a type of this package")
		}
		if _, ends := cc.Body[len(cc.Body)-1].(*ast.ReturnStmt); mustReturn && !ends {
			return polymorphicPlan{}, errors.New("case falls through to code after the switch")
		}
		if !movableArm(cc.Body) {
			return polymorphicPlan{}, errors.New("case jumps or defers")
		}
		arms = append(arms, cc)
	}
	free := armFreeVars(f, arms)
	qual := qualifier(f.unit)
	method := lowerFirst(d.Name.Name)
	for n := 2; methodTaken(f, arms, method); n++ {
		method = fmt.Sprintf("%s%d", lowerFirst(d.Name.Name), n)
	}
	var params, args []string
	for _, v := range free {
		params = append(params, v.Name()+" "+types.TypeString(v.Type(), qual))
		args = append(args, v.Name())
	}
	results := ""
	if d.Type.Results != nil {
		results = " " + f.text(d.Type.Results)
	}
	recv := "_"
	if bound != nil {
		recv = bound.Name
	}
	ifaceSig := fmt.Sprintf("%s(%s)%s", method, strings.Join(params, ", "), results)
	var methods strings.Builder
	var symbols []string
	for _, cc := range arms {
		typ := f.text(cc.List[0])
		fmt.Fprintf(&methods, "\n\nfunc (%s %s) %s(%s)%s {\n%s\n}", recv, typ, method, strings.Join(params, ", "), results,
			string(f.src[f.offset(cc.Body[0].Pos()):f.offset(cc.Body[len(cc.Body)-1].End())]))
		recvField := &ast.FieldList{List: []*ast.Field{{Type: cc.List[0]}}}
		symbols = append(symbols, symbolOf(f.pkg.PkgPath, &ast.FuncDecl{Recv: recvField, Name: ast.NewIdent(method)}))
	}
	p, ok2 := freshName(f, "variant"), freshName(f, "isVariant")
	call := fmt.Sprintf("%s.%s(%s)", p, method, strings.Join(args, ", "))
	var body strings.Builder
	fmt.Fprintf(&body, "if %s, %s := %s.(interface{ %s }); %s {\n", p, ok2, f.text(x), ifaceSig, ok2)
	switch {
	case sig.Results().Len() > 0:
		body.WriteString("return " + call + "\n")
	case trailing:
		body.WriteString(call + "\nreturn\n")
	default:
		body.WriteString(call + "\n")
	}
	body.WriteString("}")
	if deflt != nil && len(deflt.Body) > 0 {
		body.WriteString(" else {\n")
		if bound != nil && usesImplicit(f, deflt) {
			fmt.Fprintf(&body, "%s := %s\n", bound.Name, f.text(x))
		}
		body.WriteString(string(f.src[f.offset(deflt.Body[0].Pos()):f.offset(deflt.Body[len(deflt.Body)-1].End())]) + "\n}")
	}
	end := f.offset(d.End())
	return polymorphicPlan{
		splices: []splice{
			{f.offset(n.Pos()), f.offset(n.End()), body.String()},
			{end, end, methods.String()},
		},
		methods: symbols,
	}, nil
}

func typeSwitchParts(n *ast.TypeSwitchStmt) (*ast.Ident, ast.Expr) {
	switch a := n.Assign.(type) {
	case *ast.AssignStmt:
		return a.Lhs[0].(*ast.Ident), ast.Unparen(a.Rhs[0].(*ast.TypeAssertExpr).X)
	case *ast.ExprStmt:
		return nil, ast.Unparen(ast.Unparen(a.X).(*ast.TypeAssertExpr).X)
	}
	return nil, nil
}

func localPointerType(f fn, t types.Type) bool {
	p, ok := types.Unalias(t).(*types.Pointer)
	if !ok {
		return false
	}
	named, ok := types.Unalias(p.Elem()).(*types.Named)
	return ok && named.Obj().Pkg() == f.pkg.Types && named.TypeParams() == nil
}

func movableArm(body []ast.Stmt) bool {
	ok := true
	for _, st := range body {
		ast.Inspect(st, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.FuncLit:
				return false
			case *ast.BranchStmt, *ast.LabeledStmt, *ast.DeferStmt:
				ok = false
			case *ast.CallExpr:
				if id, isIdent := n.Fun.(*ast.Ident); isIdent && id.Name == "recover" {
					ok = false
				}
			}
			return ok
		})
	}
	return ok
}

// The switch opens the function, so only parameters and the receiver can be in scope.
func armFreeVars(f fn, arms []*ast.CaseClause) []*types.Var {
	seen := map[*types.Var]bool{}
	var out []*types.Var
	for _, cc := range arms {
		implicit := f.pkg.TypesInfo.Implicits[cc]
		for _, st := range cc.Body {
			ast.Inspect(st, func(n ast.Node) bool {
				id, ok := n.(*ast.Ident)
				if !ok {
					return true
				}
				v, ok := f.pkg.TypesInfo.Uses[id].(*types.Var)
				if ok && v != implicit && !seen[v] && local(f, v) && v.Pos() < f.decl.Body.Lbrace {
					seen[v] = true
					out = append(out, v)
				}
				return true
			})
		}
	}
	sortVars(out)
	return out
}

func usesImplicit(f fn, cc *ast.CaseClause) bool {
	implicit := f.pkg.TypesInfo.Implicits[cc]
	used := false
	for _, st := range cc.Body {
		ast.Inspect(st, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && f.pkg.TypesInfo.Uses[id] == implicit {
				used = true
			}
			return !used
		})
	}
	return used
}

func methodTaken(f fn, arms []*ast.CaseClause, name string) bool {
	for _, cc := range arms {
		if taken(f, f.pkg.TypesInfo.TypeOf(cc.List[0]), name) {
			return true
		}
	}
	return false
}

func freshName(f fn, base string) string {
	used := map[string]bool{}
	ast.Inspect(f.decl, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok {
			used[id.Name] = true
		}
		return true
	})
	name := base
	for n := 2; used[name]; n++ {
		name = fmt.Sprintf("%s%d", base, n)
	}
	return name
}
