package columbo

import (
	"go/ast"
	"go/types"
)

// typeReferenceIndex distinguishes genuinely file-local types from unexported
// collaborators referenced by other physical files, including test variants.
type typeReferenceIndex struct {
	engine *engine
	cross  map[string]bool
}
type privateReferences struct {
	index *typeReferenceIndex
	path  string
	info  *types.Info
}

func (a *engine) findPrivate() {
	index := &typeReferenceIndex{a, map[string]bool{}}
	for _, f := range a.files {
		index.references(f)
	}
	for _, f := range a.files {
		index.definitions(f.typeInfo())
	}
}
func (i *typeReferenceIndex) references(f *file) {
	scan := &privateReferences{i, f.path, f.typeInfo()}
	ast.Inspect(f.ast, scan.visit)
}
func (s *privateReferences) visit(n ast.Node) bool {
	if id, ok := n.(*ast.Ident); ok {
		s.index.reference(s.typeName(id), s.path)
	}
	return true
}
func (s *privateReferences) typeName(id *ast.Ident) *types.TypeName {
	obj, _ := s.info.Uses[id].(*types.TypeName)
	return obj
}
func (i *typeReferenceIndex) reference(obj *types.TypeName, path string) {
	if obj != nil && i.engine.typePosition(obj).Filename != path {
		i.cross[i.engine.typeKey(obj)] = true
	}
}
func (i *typeReferenceIndex) definitions(info *types.Info) {
	for _, obj := range info.Defs {
		if t, ok := obj.(*types.TypeName); ok && !t.Exported() {
			i.engine.private[t] = !i.cross[i.engine.typeKey(t)]
		}
	}
}
