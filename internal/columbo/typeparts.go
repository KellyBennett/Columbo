package columbo

import "go/types"

// Structural components are shared by dependency collection and interface discovery.
// Named types and interfaces remain policy decisions for those investigations.
type elementType interface{ Elem() types.Type }
type mapType interface {
	Key() types.Type
	Elem() types.Type
}
type fieldType interface {
	NumFields() int
	Field(int) *types.Var
}
type signatureType interface {
	Params() *types.Tuple
	Results() *types.Tuple
}

func typeComponents(t types.Type) []types.Type {
	switch t := t.(type) {
	case mapType:
		return []types.Type{t.Key(), t.Elem()}
	case elementType:
		return []types.Type{t.Elem()}
	case fieldType:
		return fieldComponents(t)
	case signatureType:
		return append(tupleComponents(t.Params()), tupleComponents(t.Results())...)
	}
	return nil
}
func fieldComponents(t fieldType) []types.Type {
	out := []types.Type{}
	for i := 0; i < t.NumFields(); i++ {
		out = append(out, t.Field(i).Type())
	}
	return out
}

// callResultComponents expands multi-value expressions. Single results are
// already recorded directly by the dependency collector.
func callResultComponents(t types.Type) []types.Type {
	if tuple, ok := t.(*types.Tuple); ok {
		return tupleComponents(tuple)
	}
	return nil
}

func tupleComponents(t *types.Tuple) []types.Type {
	out := []types.Type{}
	for i := 0; i < t.Len(); i++ {
		out = append(out, t.At(i).Type())
	}
	return out
}
