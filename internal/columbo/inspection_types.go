package columbo

import (
	"go/ast"
	"go/token"
	"go/types"
)

type inspectionField struct {
	inspectionSpan
	Names    []string `json:"names"`
	Embedded bool     `json:"embedded"`
	TypeText string   `json:"type_text"`
	TypeKey  string   `json:"type_key,omitempty"`
}
type inspectionType struct {
	inspectionSpan
	Key         string            `json:"key"`
	ResolvedKey string            `json:"resolved_key,omitempty"`
	Alias       bool              `json:"alias"`
	Fields      []inspectionField `json:"fields"`
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
