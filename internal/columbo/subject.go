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
