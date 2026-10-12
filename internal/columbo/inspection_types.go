package columbo

import (
	"go/ast"
	"go/token"
	"go/types"
	"strconv"
)

type inspectionField struct {
	inspectionSpan
	Names    []string
	Embedded bool
	TypeText string
	TypeKey  string
}
type inspectionType struct {
	inspectionSpan
	Key         string
	ResolvedKey string
	Alias       bool
	Fields      []inspectionField
}

func (f *file) inspectionTypes() []inspectionType {
	result := []inspectionType{}
	for _, node := range f.ast.Decls {
		group, ok := node.(*ast.GenDecl)
		if !ok || group.Tok != token.TYPE {
			continue
		}
		for _, spec := range group.Specs {
			t := spec.(*ast.TypeSpec)
			result = append(result, f.inspectionType(t))
		}
	}
	return result
}
func (f *file) inspectionType(t *ast.TypeSpec) inspectionType {
	obj, _ := f.typeInfo().Defs[t.Name].(*types.TypeName)
	result := inspectionType{inspectionSpan: f.inspectionSpan(t), Key: f.packagePath() + "." + t.Name.Name, Alias: t.Assign.IsValid(), Fields: []inspectionField{}}
	if obj != nil {
		result.ResolvedKey = inspectionTypeKey(obj.Type())
	}
	result.identifyBlank(t.Name.Name)
	st, ok := t.Type.(*ast.StructType)
	if !ok {
		return result
	}
	for _, field := range st.Fields.List {
		s := f.inspectionSpan(field.Type)
		item := inspectionField{inspectionSpan: f.inspectionSpan(field), Names: []string{}, Embedded: len(field.Names) == 0, TypeText: string(f.data[s.StartOffset:s.EndOffset])}
		typ := f.typeInfo().TypeOf(field.Type)
		if typ != nil {
			item.TypeKey = inspectionTypeKey(typ)
		}
		for _, name := range field.Names {
			item.Names = append(item.Names, name.Name)
		}
		result.Fields = append(result.Fields, item)
	}
	return result
}

func (typ *inspectionType) identifyBlank(name string) {
	if name != "_" {
		return
	}
	typ.Key += "@" + typ.Path + ":" + strconv.Itoa(typ.StartOffset)
	if !typ.Alias {
		typ.ResolvedKey = typ.Key
	}
}
