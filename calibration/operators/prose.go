package operators

import (
	"errors"
	"fmt"
	"go/ast"
	"strconv"
	"strings"

	"github.com/KellyBennett/Columbo/calibration/harness"
)

const proseSmell = "prose-comment"

// packageClause is the symbol Columbo anchors a file-wide prose-comment case to.
func packageClause(u unit) string { return u.pkg.PkgPath + ".<package-clause>" }

func (u unit) site() harness.Site {
	return harness.Site{Package: u.pkg.PkgPath, Symbol: packageClause(u), Offset: u.offset(u.file.Package), File: u.rel}
}

func findUnit(c *harness.Corpus, rel string) (unit, bool) {
	for _, u := range units(c) {
		if u.rel == rel {
			return u, true
		}
	}
	return unit{}, false
}

func baselineProse(c *harness.Corpus, rel string) bool {
	for _, f := range c.Baseline.Findings {
		if f.Smell == proseSmell && f.File == rel {
			return true
		}
	}
	return false
}

// Directives are written //word with no space; prose is "// text". Treating every such
// comment as machine input is broader than Columbo's allowlist, so a cure never deletes one.
func machineInput(text string) bool {
	if strings.HasPrefix(text, "/*line ") || strings.HasPrefix(text, "// +") {
		return true
	}
	return len(text) > 2 && strings.HasPrefix(text, "//") && text[2] != ' ' && text[2] != '\t'
}

func cgoPreamble(f *ast.File, g *ast.CommentGroup) bool {
	for _, imp := range f.Imports {
		if imp.Path.Value == `"C"` && (imp.Doc == g || declDocOf(f, imp) == g) {
			return true
		}
	}
	return false
}

func declDocOf(f *ast.File, imp *ast.ImportSpec) *ast.CommentGroup {
	for _, d := range f.Decls {
		if gd, ok := d.(*ast.GenDecl); ok && len(gd.Specs) == 1 && gd.Specs[0] == imp {
			return gd.Doc
		}
	}
	return nil
}

type addProseComment struct{}

func (addProseComment) ID() string           { return "add-prose-comment" }
func (addProseComment) Class() harness.Class { return harness.Inject }
func (addProseComment) Target() string       { return proseSmell }
func (addProseComment) Doses(*harness.Corpus, harness.Site) []int {
	return []int{1}
}

func (o addProseComment) Sites(c *harness.Corpus) []harness.Site {
	var out []harness.Site
	for _, u := range units(c) {
		if !baselineProse(c, u.rel) && undocumented(u) != nil {
			out = append(out, u.site())
		}
	}
	return out
}

func undocumented(u unit) ast.Decl {
	for _, d := range u.file.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			if d.Doc == nil {
				return d
			}
		case *ast.GenDecl:
			if d.Doc == nil && d.Tok.String() != "import" {
				return d
			}
		}
	}
	return nil
}

func declName(d ast.Decl) string {
	switch d := d.(type) {
	case *ast.FuncDecl:
		return d.Name.Name
	case *ast.GenDecl:
		for _, s := range d.Specs {
			switch s := s.(type) {
			case *ast.TypeSpec:
				return s.Name.Name
			case *ast.ValueSpec:
				return s.Names[0].Name
			}
		}
	}
	return "This declaration"
}

func (o addProseComment) Apply(c *harness.Corpus, site harness.Site, _ int) (harness.Applied, error) {
	u, ok := findUnit(c, site.File)
	if !ok {
		return harness.Applied{}, errors.New("site vanished")
	}
	d := undocumented(u)
	if d == nil {
		return harness.Applied{}, errors.New("every declaration is documented")
	}
	at := u.offset(d.Pos())
	text := fmt.Sprintf("// %s keeps the calibration example readable for the next maintainer.\n", declName(d))
	out, err := finish(u.path, applySplices(u.src, []splice{{at, at, text}}))
	if err != nil {
		return harness.Applied{}, err
	}
	kill := true
	return harness.Applied{
		Edit: harness.Edit{u.rel: out},
		Expectation: harness.Expectation{
			Touched:    []string{site.Symbol},
			Prediction: harness.Prediction{Kill: &kill, Reason: "prose-comment has a zero limit and the added line is not machine input"},
		},
	}, nil
}

type proseToStringConstant struct{}

func (proseToStringConstant) ID() string           { return "prose-to-string-constant" }
func (proseToStringConstant) Class() harness.Class { return harness.Evade }
func (proseToStringConstant) Target() string       { return proseSmell }
func (proseToStringConstant) Follows() string      { return "add-prose-comment" }
func (proseToStringConstant) Doses(*harness.Corpus, harness.Site) []int {
	return []int{1}
}

func (o proseToStringConstant) Sites(c *harness.Corpus) []harness.Site {
	var out []harness.Site
	for _, u := range units(c) {
		if !baselineProse(c, u.rel) && len(proseDocs(u)) > 0 {
			out = append(out, u.site())
		}
	}
	return out
}

type proseDoc struct {
	decl ast.Decl
	doc  *ast.CommentGroup
}

func proseDocs(u unit) []proseDoc {
	var out []proseDoc
	for _, d := range u.file.Decls {
		var doc *ast.CommentGroup
		switch d := d.(type) {
		case *ast.FuncDecl:
			doc = d.Doc
		case *ast.GenDecl:
			doc = d.Doc
		}
		if doc != nil && !cgoPreamble(u.file, doc) && allProse(doc) {
			out = append(out, proseDoc{d, doc})
		}
	}
	return out
}

func allProse(g *ast.CommentGroup) bool {
	for _, c := range g.List {
		if machineInput(c.Text) {
			return false
		}
	}
	return true
}

func (o proseToStringConstant) Apply(c *harness.Corpus, site harness.Site, _ int) (harness.Applied, error) {
	u, ok := findUnit(c, site.File)
	if !ok {
		return harness.Applied{}, errors.New("site vanished")
	}
	docs := proseDocs(u)
	if len(docs) == 0 {
		return harness.Applied{}, errors.New("no prose doc comment")
	}
	var ss []splice
	taken := map[string]bool{}
	for _, pd := range docs {
		name := freeUnitName(u, collabLowerFirst(declName(pd.decl))+"Doc", taken)
		ss = append(ss,
			splice{u.offset(pd.doc.Pos()), u.offset(pd.decl.Pos()), ""},
			splice{u.offset(pd.decl.End()), u.offset(pd.decl.End()), fmt.Sprintf("\n\nconst %s = %s", name, strconv.Quote(strings.TrimSpace(pd.doc.Text())))})
	}
	out, err := finish(u.path, applySplices(u.src, ss))
	if err != nil {
		return harness.Applied{}, err
	}
	return harness.Applied{
		Edit:        harness.Edit{u.rel: out},
		Expectation: harness.Expectation{Touched: []string{site.Symbol}, Catches: []string{proseSmell}},
	}, nil
}

func freeUnitName(u unit, base string, taken map[string]bool) string {
	for n := 0; ; n++ {
		name := base
		if n > 0 {
			name = fmt.Sprintf("%s%d", base, n)
		}
		if !taken[name] && u.pkg.Types.Scope().Lookup(name) == nil {
			taken[name] = true
			return name
		}
	}
}

type deleteProseComments struct{}

func (deleteProseComments) ID() string           { return "delete-prose-comments" }
func (deleteProseComments) Class() harness.Class { return harness.Cure }
func (deleteProseComments) Target() string       { return proseSmell }
func (deleteProseComments) Doses(*harness.Corpus, harness.Site) []int {
	return []int{1}
}

func (o deleteProseComments) Sites(c *harness.Corpus) []harness.Site {
	var out []harness.Site
	for _, u := range units(c) {
		if baselineProse(c, u.rel) {
			out = append(out, u.site())
		}
	}
	return out
}

func (o deleteProseComments) Apply(c *harness.Corpus, site harness.Site, _ int) (harness.Applied, error) {
	u, ok := findUnit(c, site.File)
	if !ok {
		return harness.Applied{}, errors.New("site vanished")
	}
	var ss []splice
	for _, g := range u.file.Comments {
		if cgoPreamble(u.file, g) {
			continue
		}
		for _, cm := range g.List {
			if !machineInput(cm.Text) {
				ss = append(ss, lineAware(u.src, u.offset(cm.Pos()), u.offset(cm.End())))
			}
		}
	}
	if len(ss) == 0 {
		return harness.Applied{}, errors.New("no prohibited comment found")
	}
	out, err := finish(u.path, applySplices(u.src, ss))
	if err != nil {
		return harness.Applied{}, err
	}
	return harness.Applied{
		Edit:        harness.Edit{u.rel: out},
		Expectation: harness.Expectation{Touched: []string{site.Symbol}},
	}, nil
}
