package operators

import (
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"github.com/KellyBennett/Columbo/calibration/harness"
)

// A nested block ends a cluster sequence, so the helper calls stop forming one cluster though the design is unchanged.
type isolateHelperCalls struct {
	id, follows string
}

func (o isolateHelperCalls) ID() string                                { return o.id }
func (o isolateHelperCalls) Class() harness.Class                      { return harness.Evade }
func (o isolateHelperCalls) Target() string                            { return "cosmetic-extraction" }
func (o isolateHelperCalls) Follows() string                           { return o.follows }
func (o isolateHelperCalls) Doses(*harness.Corpus, harness.Site) []int { return []int{1} }

func (o isolateHelperCalls) Sites(c *harness.Corpus) []harness.Site {
	var out []harness.Site
	for _, f := range funcs(c) {
		if f.decl.Body != nil && helperCallStatements(c, f) >= 2 {
			out = append(out, f.site())
		}
	}
	return out
}

func (o isolateHelperCalls) Apply(c *harness.Corpus, site harness.Site, _ int) (harness.Applied, error) {
	f, ok := findFn(c, site.Symbol)
	if !ok {
		return harness.Applied{}, errors.New("site vanished")
	}
	qual := qualifier(f.unit)
	var ss []splice
	for _, s := range f.decl.Body.List {
		text, ok := isolate(f, s, qual)
		if ok {
			ss = append(ss, splice{f.offset(s.Pos()), f.offset(s.End()), text})
		}
	}
	if len(ss) < 2 {
		return harness.Applied{}, errors.New("fewer than two isolatable statements")
	}
	out, err := finish(f.path, applySplices(f.src, ss))
	if err != nil {
		return harness.Applied{}, err
	}
	return harness.Applied{
		Edit:        harness.Edit{f.rel: out},
		Expectation: harness.Expectation{Touched: helperTouched(c, f), Catches: []string{"cosmetic-extraction", "long-function"}},
	}, nil
}

func isolate(f fn, s ast.Stmt, qual types.Qualifier) (string, bool) {
	switch s := s.(type) {
	case *ast.ExprStmt:
		if _, ok := s.X.(*ast.CallExpr); ok {
			return "{\n" + f.text(s) + "\n}", true
		}
	case *ast.AssignStmt:
		if len(s.Rhs) != 1 {
			return "", false
		}
		if _, ok := s.Rhs[0].(*ast.CallExpr); !ok {
			return "", false
		}
		if s.Tok == token.ASSIGN {
			return "{\n" + f.text(s) + "\n}", true
		}
		if s.Tok != token.DEFINE {
			return "", false
		}
		var decls []string
		lhs := make([]string, len(s.Lhs))
		for i, l := range s.Lhs {
			lhs[i] = f.text(l)
			id, ok := l.(*ast.Ident)
			if !ok {
				return "", false
			}
			if obj := f.pkg.TypesInfo.Defs[id]; obj != nil {
				decls = append(decls, fmt.Sprintf("var %s %s", id.Name, types.TypeString(obj.Type(), qual)))
			}
		}
		return strings.Join(decls, "\n") + "\n{\n" + strings.Join(lhs, ", ") + " = " + f.text(s.Rhs[0]) + "\n}", true
	}
	return "", false
}

// Helpers count as touched so a cosmetic or long-function case that lands on one still counts.
func helperTouched(c *harness.Corpus, f fn) []string {
	touched := []string{f.symbol}
	ast.Inspect(f.decl.Body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if callee, ok := resolveCallee(c, f, call); ok {
				touched = append(touched, callee.symbol)
			}
		}
		return true
	})
	return touched
}

// helperCallStatements counts top-level statements that call a same-package function, so the
// evade targets the cosmetic parent and not the helpers it called.
func helperCallStatements(c *harness.Corpus, f fn) int {
	n := 0
	for _, s := range f.decl.Body.List {
		found := false
		ast.Inspect(s, func(x ast.Node) bool {
			if call, ok := x.(*ast.CallExpr); ok && !found {
				if _, ok := resolveCallee(c, f, call); ok {
					found = true
				}
			}
			return !found
		})
		if found {
			n++
		}
	}
	return n
}
