package columbo

import (
	"go/ast"
	"go/token"
)

type subjectSyntax struct {
	doc  *ast.CommentGroup
	name *ast.Ident
	recv *ast.FieldList
	typ  *ast.FuncType
	body *ast.BlockStmt
	span ast.Node
}

func funcSubject(fn *ast.FuncDecl) *subjectSyntax {
	return &subjectSyntax{doc: fn.Doc, name: fn.Name, recv: fn.Recv, typ: fn.Type, body: fn.Body, span: fn}
}

func literalSubject(doc *ast.CommentGroup, name *ast.Ident, lit *ast.FuncLit) *subjectSyntax {
	return &subjectSyntax{doc: doc, name: name, typ: lit.Type, body: lit.Body, span: lit}
}

func declarationSubjects(node ast.Decl) []*subjectSyntax {
	switch decl := node.(type) {
	case *ast.FuncDecl:
		return []*subjectSyntax{funcSubject(decl)}
	case *ast.GenDecl:
		return variableSubjects(decl)
	}
	return nil
}
func variableSubjects(decl *ast.GenDecl) []*subjectSyntax {
	out := []*subjectSyntax{}
	if decl.Tok != token.VAR {
		return out
	}
	for _, spec := range decl.Specs {
		out = append(out, specSubjects(decl, spec.(*ast.ValueSpec))...)
	}
	return out
}
func specSubjects(decl *ast.GenDecl, spec *ast.ValueSpec) []*subjectSyntax {
	out := []*subjectSyntax{}
	if len(spec.Values) != len(spec.Names) {
		return out
	}
	for i, name := range spec.Names {
		if lit, ok := ast.Unparen(spec.Values[i]).(*ast.FuncLit); ok {
			out = append(out, literalSubject(specDoc(decl, spec), name, lit))
		}
	}
	return out
}
func specDoc(decl *ast.GenDecl, spec *ast.ValueSpec) *ast.CommentGroup {
	if len(spec.Names) != 1 {
		return nil
	}
	if decl.Lparen.IsValid() {
		return spec.Doc
	}
	return decl.Doc
}

func (s *subjectSyntax) Pos() token.Pos { return s.span.Pos() }
func (s *subjectSyntax) End() token.Pos { return s.span.End() }

func (s *subjectSyntax) nodes() []ast.Node {
	out := []ast.Node{}
	if s.recv != nil {
		out = append(out, s.recv)
	}
	out = append(out, s.typ)
	if s.body != nil {
		out = append(out, s.body)
	}
	return out
}
func (s *subjectSyntax) inspect(visit func(ast.Node) bool) {
	for _, n := range s.nodes() {
		ast.Inspect(n, visit)
	}
}
func (s *subjectSyntax) walk(v ast.Visitor) {
	for _, n := range s.nodes() {
		ast.Walk(v, n)
	}
}
