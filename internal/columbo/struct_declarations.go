package columbo

import (
	"go/ast"
	"go/token"
	"go/types"
)

type structDeclaration struct {
	file *file
	spec *ast.TypeSpec
	body *ast.StructType
}

func (f *file) structDeclarations() []structDeclaration {
	out := []structDeclaration{}
	for _, node := range f.ast.Decls {
		if decl, ok := node.(*ast.GenDecl); ok && decl.Tok == token.TYPE {
			out = append(out, f.structSpecs(decl)...)
		}
	}
	return out
}
func (f *file) structSpecs(decl *ast.GenDecl) []structDeclaration {
	out := []structDeclaration{}
	for _, spec := range decl.Specs {
		typeSpec := spec.(*ast.TypeSpec)
		body, ok := typeSpec.Type.(*ast.StructType)
		if ok && !typeSpec.Assign.IsValid() && typeSpec.Name.Name != "_" {
			out = append(out, structDeclaration{file: f, spec: typeSpec, body: body})
		}
	}
	return out
}
func (s structDeclaration) ref() DeclarationRef {
	return DeclarationRef{File: s.file.rel, Symbol: s.file.packagePath() + "." + s.spec.Name.Name}
}
func (s structDeclaration) declReceipt() Source {
	ref := s.ref()
	return s.file.receipt("declaration", s.spec.Pos(), s.spec.End(), Detail{Subject: ref.Symbol}).forDeclaration(ref)
}
func (s structDeclaration) fieldParameter(field *ast.Field, name *ast.Ident, index int) parameter {
	v := s.file.typeInfo().Defs[name].(*types.Var)
	return parameter{typ: v.Type(), field: field, index: index, obj: v}
}
func (s structDeclaration) fieldReceipt(field parameter) Source {
	start, end := field.sourceRange()
	return s.file.receipt("field", start, end, field.detail(nil)).forDeclaration(s.ref())
}
func (s structDeclaration) evidence() DeclarationEvidence {
	return DeclarationEvidence{Ref: s.ref(), Source: s.declReceipt(), Dependencies: []DependencyEvidence{}}
}

func (selection evidenceSelection) structDeclarations(files []*file) []DeclarationEvidence {
	out := []DeclarationEvidence{}
	for _, f := range files {
		for _, s := range f.structDeclarations() {
			if selection[s.ref()] {
				out = append(out, s.evidence())
			}
		}
	}
	return out
}
