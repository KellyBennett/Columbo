package columbo

import (
	"fmt"
	"go/ast"
	"regexp"
	"strings"
	"unicode"
)

var directiveRE = regexp.MustCompile(`^columbo:ignore[\t ]+([^\t ]+)[\t ]+--[\t ]+(.+)$`)

type suppressionScanner struct {
	engine   *engine
	attached map[*ast.Comment]*declaration
	seen     map[string]bool
}
type suppressionDirective struct {
	file                 *file
	comment              *ast.Comment
	owner                *declaration
	smell, justification string
}

func (a *engine) suppressions() error {
	s := &suppressionScanner{engine: a, attached: map[*ast.Comment]*declaration{}, seen: map[string]bool{}}
	s.attachComments()
	for _, f := range a.files {
		if f.included {
			if e := s.scan(f); e != nil {
				return e
			}
		}
	}
	return nil
}
func (s *suppressionScanner) attachComments() {
	for _, d := range s.engine.declarations {
		if d.file.included && d.fn.Doc != nil {
			s.attachDeclaration(d)
		}
	}
}
func (s *suppressionScanner) attachDeclaration(d *declaration) {
	for _, c := range d.fn.Doc.List {
		s.attached[c] = d
	}
}
func (s *suppressionScanner) scan(f *file) error {
	for _, g := range f.ast.Comments {
		for _, c := range g.List {
			if e := s.comment(f, c); e != nil {
				return e
			}
		}
	}
	return nil
}
func suppressionText(c *ast.Comment) string {
	if !strings.HasPrefix(c.Text, "//") {
		return ""
	}
	text := strings.TrimLeft(strings.TrimPrefix(c.Text, "//"), " \t")
	if !strings.HasPrefix(text, "columbo:ignore") {
		return ""
	}
	return text
}
func (s *suppressionScanner) comment(f *file, c *ast.Comment) error {
	text := suppressionText(c)
	if text == "" {
		return nil
	}
	d := s.directive(f, c)
	if e := d.parse(text, s.engine.config); e != nil {
		return e
	}
	if e := s.unique(d); e != nil {
		return e
	}
	s.engine.applySuppression(d.record())
	return nil
}
func (s *suppressionScanner) directive(f *file, c *ast.Comment) *suppressionDirective {
	return &suppressionDirective{file: f, comment: c, owner: s.attached[c]}
}
func (d *suppressionDirective) invalid() error {
	return fmt.Errorf("%s:%d: invalid columbo suppression", d.file.rel, d.file.line(d.comment.Pos()))
}
func (d *suppressionDirective) parse(text string, config Config) error {
	m := directiveRE.FindStringSubmatch(text)
	if m == nil {
		return d.invalid()
	}
	d.smell = m[1]
	if _, ok := config.Severity[d.smell]; !ok || d.smell == "data-clump" {
		return d.invalid()
	}
	d.justification = strings.TrimSpace(m[2])
	if justificationLength(d.justification) < 10 || d.owner == nil {
		return d.invalid()
	}
	return nil
}
func justificationLength(s string) int {
	n := 0
	for _, r := range s {
		if !unicode.IsSpace(r) {
			n++
		}
	}
	return n
}
func (s *suppressionScanner) unique(d *suppressionDirective) error {
	key := d.key()
	if s.seen[key] {
		return d.invalid()
	}
	s.seen[key] = true
	return nil
}
func (d *suppressionDirective) key() string {
	return fmt.Sprintf("%s:%d:%s", d.file.rel, d.owner.declReceipt().StartOffset, d.smell)
}
func (d *suppressionDirective) record() Suppression {
	return Suppression{Smell: d.smell, Symbol: d.owner.symbol, File: d.file.rel, Line: d.file.line(d.comment.Pos()), Justification: d.justification}
}
func (a *engine) applySuppression(s Suppression) {
	for i := range a.report.Cases {
		if a.report.Cases[i].matchesSuppression(s) {
			s.apply(&a.report.Cases[i])
			break
		}
	}
	if !s.Applied {
		a.report.Warnings = append(a.report.Warnings, s.unusedWarning())
	}
	a.report.Suppressions = append(a.report.Suppressions, s)
}
func (c Case) matchesSuppression(s Suppression) bool {
	return c.File == s.File && c.Symbol == s.Symbol && c.Smell == s.Smell
}
func (s *Suppression) apply(c *Case) { c.Suppressed = true; s.Applied = true; s.CaseID = c.ID }
func (s Suppression) unusedWarning() Warning {
	return Warning{"unused-suppression", s.File, s.Line, fmt.Sprintf("Unused suppression for %s on %s.", s.Smell, s.Symbol)}
}
