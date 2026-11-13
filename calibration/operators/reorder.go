package operators

import (
	"errors"
	"go/ast"
	"go/token"

	"github.com/KellyBennett/Columbo/calibration/harness"
)

// reorderDecls reverses the order of a file's top-level function, type, and constant
// declarations. Variable declarations keep their slots because their initialization order
// is observable, and a file with more than one init or blank function is skipped because
// init#N and _#N number in declaration order and init order is observable.
type reorderDecls struct{}

func (reorderDecls) ID() string                                { return "reorder-decls" }
func (reorderDecls) Class() harness.Class                      { return harness.Neutral }
func (reorderDecls) Target() string                            { return "" }
func (reorderDecls) Doses(*harness.Corpus, harness.Site) []int { return []int{1} }

func (reorderDecls) Sites(c *harness.Corpus) []harness.Site {
	var out []harness.Site
	for _, u := range units(c) {
		if len(reorderSpans(u)) >= 2 {
			out = append(out, harness.Site{Package: u.pkg.PkgPath, Symbol: u.rel, File: u.rel})
		}
	}
	return out
}

func (reorderDecls) Apply(c *harness.Corpus, site harness.Site, _ int) (harness.Applied, error) {
	for _, u := range units(c) {
		if u.rel != site.File {
			continue
		}
		spans := reorderSpans(u)
		if len(spans) < 2 {
			return harness.Applied{}, errors.New("fewer than two declMovable declarations")
		}
		ss := make([]splice, len(spans))
		for i, s := range spans {
			from := spans[len(spans)-1-i]
			ss[i] = splice{s.start, s.end, string(u.src[from.start:from.end])}
		}
		out, err := finish(u.path, applySplices(u.src, ss))
		if err != nil {
			return harness.Applied{}, err
		}
		return harness.Applied{Edit: harness.Edit{u.rel: out}}, nil
	}
	return harness.Applied{}, errors.New("site vanished")
}

type span struct{ start, end int }

func reorderSpans(u unit) []span {
	repeated := map[string]int{}
	var spans []span
	for _, d := range u.file.Decls {
		switch d := d.(type) {
		case *ast.GenDecl:
			if d.Tok == token.IMPORT || d.Tok == token.VAR {
				continue
			}
			spans = append(spans, u.declSpan(d, d.Doc))
		case *ast.FuncDecl:
			if d.Recv == nil && (d.Name.Name == "init" || d.Name.Name == "_") {
				repeated[d.Name.Name]++
			}
			spans = append(spans, u.declSpan(d, d.Doc))
		}
	}
	if repeated["init"] > 1 || repeated["_"] > 1 {
		return nil
	}
	return spans
}

// declSpan covers a declaration with its doc comment and any comment trailing its last line.
func (u unit) declSpan(d ast.Decl, doc *ast.CommentGroup) span {
	start, end := d.Pos(), d.End()
	if doc != nil {
		start = doc.Pos()
	}
	fset := u.pkg.Fset
	for _, g := range u.file.Comments {
		if g.Pos() >= end && fset.Position(g.Pos()).Line == fset.Position(end).Line {
			end = g.End()
			break
		}
	}
	return span{u.offset(start), u.offset(end)}
}
