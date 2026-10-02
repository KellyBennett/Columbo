package columbo

import (
	"fmt"
	"go/ast"
	"regexp"
	"strings"
	"unicode"
)

var directiveRE = regexp.MustCompile(`^columbo:ignore[\t ]+([^\t ]+)[\t ]+--[\t ]+(.+)$`)

func (a *engine) suppressions() error {
	attached := map[*ast.Comment]*declaration{}
	for _, d := range a.declarations {
		if d.file.included && d.fn.Doc != nil {
			for _, c := range d.fn.Doc.List {
				attached[c] = d
			}
		}
	}
	seen := map[string]bool{}
	for _, f := range a.files {
		if !f.included {
			continue
		}
		for _, g := range f.ast.Comments {
			for _, comment := range g.List {
				if !strings.HasPrefix(comment.Text, "//") {
					continue
				}
				text := strings.TrimLeft(strings.TrimPrefix(comment.Text, "//"), " \t")
				if !strings.HasPrefix(text, "columbo:ignore") {
					continue
				}
				fail := func() error { return fmt.Errorf("%s:%d: invalid columbo suppression", f.rel, f.tf.Line(comment.Pos())) }
				m := directiveRE.FindStringSubmatch(text)
				if m == nil {
					return fail()
				}
				smell := m[1]
				if _, ok := a.config.Severity[smell]; !ok || smell == "data-clump" {
					return fail()
				}
				just := strings.TrimSpace(m[2])
				count := 0
				for _, r := range just {
					if !unicode.IsSpace(r) {
						count++
					}
				}
				if count < 10 {
					return fail()
				}
				d := attached[comment]
				if d == nil {
					return fail()
				}
				key := fmt.Sprintf("%s:%d:%s", f.rel, f.tf.Offset(d.fn.Pos()), smell)
				if seen[key] {
					return fail()
				}
				seen[key] = true
				s := Suppression{Smell: smell, Symbol: d.symbol, File: f.rel, Line: f.tf.Line(comment.Pos()), Justification: just}
				for i := range a.report.Cases {
					c := &a.report.Cases[i]
					if c.File == f.rel && c.Symbol == d.symbol && c.Smell == smell {
						c.Suppressed = true
						s.Applied = true
						s.CaseID = c.ID
						break
					}
				}
				if !s.Applied {
					a.report.Warnings = append(a.report.Warnings, Warning{"unused-suppression", s.File, s.Line, fmt.Sprintf("Unused suppression for %s on %s.", s.Smell, s.Symbol)})
				}
				a.report.Suppressions = append(a.report.Suppressions, s)
			}
		}
	}
	return nil
}
