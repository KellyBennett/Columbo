package columbo

import (
	"go/ast"
	"go/token"
	"strings"
)

const commentSmell = "prose-comment"

func (a *engine) comments() error {
	for _, f := range a.files {
		if f.included {
			if err := a.commentCase(f.proseComments()); err != nil {
				return err
			}
		}
	}
	return nil
}
func (a *engine) commentCase(receipts []Source) error {
	if len(receipts) == 0 {
		return nil
	}
	c, err := a.identifiedCase(a.sourceCase(commentSmell, receipts[0]), "")
	if err != nil || c == nil {
		return err
	}
	c.commentEvidence(receipts)
	a.report.Cases = append(a.report.Cases, *c)
	return nil
}
func (c *Case) commentEvidence(receipts []Source) {
	appendSources(c, receipts[1:])
	c.Clues = append(c.Clues, c.metric("prose-comments", len(receipts)).compare(0, ">").supportedBy(receipts))
}
func (f *file) proseComments() []Source {
	preambles := f.cgoPreambles()
	sources := []Source{}
	for _, group := range f.ast.Comments {
		if !preambles[group] {
			sources = append(sources, f.commentSources(group)...)
		}
	}
	return sources
}
func (f *file) commentSources(group *ast.CommentGroup) []Source {
	sources := []Source{}
	for _, comment := range group.List {
		if !machineComment(comment.Text) {
			sources = append(sources, f.commentReceipt(comment))
		}
	}
	return sources
}
func (f *file) commentReceipt(comment *ast.Comment) Source {
	source := f.receipt("prose-comment", comment.Pos(), f.commentEnd(comment), Detail{Subject: f.packageClauseRef().Symbol, Value: 1})
	return source.forDeclaration(f.packageClauseRef())
}
func (f *file) commentEnd(comment *ast.Comment) token.Pos {
	start := f.tf.Offset(comment.Pos())
	raw := string(f.data[start:])
	if strings.HasPrefix(raw, "/*") {
		return f.tf.Pos(start + strings.Index(raw, "*/") + 2)
	}
	if newline := strings.IndexByte(raw, '\n'); newline >= 0 {
		return f.tf.Pos(start + newline)
	}
	return f.tf.Pos(len(f.data))
}
func (f *file) packageClauseRef() DeclarationRef {
	return DeclarationRef{File: f.rel, Symbol: f.packagePath() + ".<package-clause>"}
}
func (f *file) packageClauseEvidence() DeclarationEvidence {
	return DeclarationEvidence{Ref: f.packageClauseRef(), Source: f.packageClauseReceipt(), Dependencies: []DependencyEvidence{}}
}
func (f *file) packageClauseReceipt() Source {
	ref := f.packageClauseRef()
	start, end := f.packageClauseRange()
	return f.receipt("package-clause", start, end, Detail{Subject: ref.Symbol}).forDeclaration(ref)
}
func (f *file) cgoPreambles() map[*ast.CommentGroup]bool {
	groups := map[*ast.CommentGroup]bool{}
	for _, node := range f.ast.Decls {
		if decl, ok := node.(*ast.GenDecl); ok && decl.Tok == token.IMPORT {
			addCgoPreambles(groups, decl)
		}
	}
	return groups
}
func addCgoPreambles(groups map[*ast.CommentGroup]bool, decl *ast.GenDecl) {
	for _, spec := range decl.Specs {
		imp := spec.(*ast.ImportSpec)
		if imp.Path.Value == `"C"` {
			groups[imp.Doc] = true
			if len(decl.Specs) == 1 {
				groups[decl.Doc] = true
			}
		}
	}
}

func (f *file) packageClauseRange() (token.Pos, token.Pos) { return f.ast.Package, f.ast.Name.End() }
