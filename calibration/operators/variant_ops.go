package operators

import (
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"strings"

	"github.com/KellyBennett/Columbo/calibration/harness"
)

const variantSmell = "repeated-variant-decision"

// A one-variant decision is not a site at all, so dose 1 never counts toward the domain.
type repeatVariantDecision struct{}

func (repeatVariantDecision) ID() string           { return "repeat-variant-decision" }
func (repeatVariantDecision) Class() harness.Class { return harness.Inject }
func (repeatVariantDecision) Target() string       { return variantSmell }

func (repeatVariantDecision) Sites(c *harness.Corpus) []harness.Site {
	var out []harness.Site
	domains := variantDomains(c)
	for _, s := range variantSites(c) {
		if _, fires := repeatedVariants(c, domains[s.domain]); fires {
			continue
		}
		if _, ok := domainTypeText(s.fn, s.named); ok {
			out = append(out, s.site())
		}
	}
	return out
}

func (s variantSite) site() harness.Site {
	site := s.fn.site()
	site.Offset = s.fn.offset(s.node.Pos())
	return site
}

func findVariantSite(c *harness.Corpus, site harness.Site) (variantSite, error) {
	f, ok := findFn(c, site.Symbol)
	if !ok {
		return variantSite{}, errors.New("site vanished")
	}
	for _, s := range variantSitesIn(f) {
		if f.offset(s.node.Pos()) == site.Offset {
			return s, nil
		}
	}
	return variantSite{}, errors.New("decision vanished")
}

func (repeatVariantDecision) Doses(c *harness.Corpus, site harness.Site) []int {
	s, err := findVariantSite(c, site)
	if err != nil {
		return nil
	}
	var doses []int
	for k := 1; k <= len(s.arms) && k <= 3; k++ {
		doses = append(doses, k)
	}
	return doses
}

func (repeatVariantDecision) Apply(c *harness.Corpus, site harness.Site, dose int) (harness.Applied, error) {
	s, err := findVariantSite(c, site)
	if err != nil {
		return harness.Applied{}, err
	}
	typ, _ := domainTypeText(s.fn, s.named)
	name := uniqueFuncName(s.fn, lowerFirst(s.fn.decl.Name.Name)+"Variant")
	tag := "v"
	if strings.HasPrefix(s.domain, "type:") {
		tag = "v.(type)"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "\n\nfunc %s(v %s) int {\nswitch %s {\n", name, typ, tag)
	for i, a := range s.arms[:dose] {
		fmt.Fprintf(&b, "case %s:\nreturn %d\n", s.fn.text(a.expr), i)
	}
	b.WriteString("}\nreturn -1\n}")
	end := s.fn.offset(s.fn.decl.End())
	out, err := finish(s.fn.path, applySplices(s.fn.src, []splice{{end, end, b.String()}}))
	if err != nil {
		return harness.Applied{}, err
	}
	before := variantDomains(c)[s.domain]
	sites := before
	if dose >= 2 {
		sites = append(sites[:len(sites):len(sites)], variantSite{arms: s.arms[:dose]})
	}
	repeated, fires := repeatedVariants(c, sites)
	symbol := symbolOf(s.fn.pkg.PkgPath, &ast.FuncDecl{Name: ast.NewIdent(name)})
	return harness.Applied{
		Edit: harness.Edit{s.fn.rel: out},
		Expectation: harness.Expectation{
			Touched: append(siteSymbols(before), symbol),
			Prediction: harness.Prediction{Kill: &fires, Reason: fmt.Sprintf("%d repeated variants across %d decisions (limits %d sites, %d variants)",
				len(repeated), len(sites), int(threshold(c, "repeated-variant-sites", 2)), int(threshold(c, "repeated-variant-variants", 2)))},
		},
	}, nil
}

// The variant knowledge is still written out at the second site; only its syntax changed.
type decisionToMapLookup struct{}

func (decisionToMapLookup) ID() string           { return "decision-to-map-lookup" }
func (decisionToMapLookup) Class() harness.Class { return harness.Evade }
func (decisionToMapLookup) Target() string       { return variantSmell }
func (decisionToMapLookup) Follows() string      { return "repeat-variant-decision" }

func (decisionToMapLookup) Sites(c *harness.Corpus) []harness.Site {
	var out []harness.Site
	for _, s := range variantSites(c) {
		if _, ok := lookupShape(s); ok {
			out = append(out, s.site())
		}
	}
	return out
}

func (decisionToMapLookup) Doses(*harness.Corpus, harness.Site) []int { return []int{1} }

func lookupShape(s variantSite) ([]string, bool) {
	body := s.fn.decl.Body.List
	if len(body) != 2 || body[0] != s.node || s.fn.decl.Type.Params.NumFields() != 1 {
		return nil, false
	}
	if r, ok := body[1].(*ast.ReturnStmt); !ok || len(r.Results) != 1 {
		return nil, false
	}
	var clauses []ast.Stmt
	switch n := s.node.(type) {
	case *ast.SwitchStmt:
		clauses = n.Body.List
	case *ast.TypeSwitchStmt:
		clauses = n.Body.List
	default:
		return nil, false
	}
	var results []string
	for _, cl := range clauses {
		cc := cl.(*ast.CaseClause)
		if len(cc.List) != 1 || len(cc.Body) != 1 {
			return nil, false
		}
		r, ok := cc.Body[0].(*ast.ReturnStmt)
		if !ok || len(r.Results) != 1 {
			return nil, false
		}
		if lit, ok := r.Results[0].(*ast.BasicLit); !ok || lit.Kind != token.INT {
			return nil, false
		}
		results = append(results, s.fn.text(r.Results[0]))
	}
	return results, true
}

func (decisionToMapLookup) Apply(c *harness.Corpus, site harness.Site, _ int) (harness.Applied, error) {
	s, err := findVariantSite(c, site)
	if err != nil {
		return harness.Applied{}, err
	}
	results, ok := lookupShape(s)
	if !ok {
		return harness.Applied{}, errors.New("not a table-shaped decision")
	}
	d := s.fn.decl
	param := d.Type.Params.List[0]
	pname, ptype := param.Names[0].Name, s.fn.text(param.Type)
	table := uniqueFuncName(s.fn, d.Name.Name+"Table")
	fallback := s.fn.text(d.Body.List[1].(*ast.ReturnStmt).Results[0])
	keyType, key := ptype, pname
	typed := strings.HasPrefix(s.domain, "type:")
	if typed {
		keyType, key = "reflect.Type", "reflect.TypeOf("+pname+")"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "var %s = map[%s]int{\n", table, keyType)
	for i, a := range s.arms {
		k := s.fn.text(a.expr)
		if typed {
			k = "reflect.TypeOf((*" + k + ")(nil)).Elem()"
		}
		fmt.Fprintf(&b, "%s: %s,\n", k, results[i])
	}
	fmt.Fprintf(&b, "}\n\nfunc %s(%s %s) int {\nif r, ok := %s[%s]; ok {\nreturn r\n}\nreturn %s\n}", d.Name.Name, pname, ptype, table, key, fallback)
	ss := []splice{{s.fn.offset(d.Pos()), s.fn.offset(d.End()), b.String()}}
	if d.Doc != nil {
		ss[0].start = s.fn.offset(d.Doc.Pos())
	}
	out, err := finish(s.fn.path, applySplices(s.fn.src, ss))
	if err != nil {
		return harness.Applied{}, err
	}
	return harness.Applied{
		Edit: harness.Edit{s.fn.rel: out},
		Expectation: harness.Expectation{
			Touched: siteSymbols(variantDomains(c)[s.domain]),
			Catches: []string{variantSmell},
		},
	}, nil
}
