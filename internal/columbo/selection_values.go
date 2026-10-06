package columbo

import (
	"go/ast"
	"go/types"
	"maps"
	"slices"
	"strings"
)

const selectionSmell = "selection-use-coupling"

// Values are immutable snapshots. Copying a local preserves the selected value,
// even when the original variable is subsequently overwritten.
type selectedValue struct {
	origins map[ast.Expr]string
	traces  map[*selectionSite][]ast.Node
	unknown bool
}
type selectionEnv map[types.Object]selectedValue

type selectionSite struct {
	observations []selectedUse
	owner        *declaration
	decision     ast.Node
	role         string
	origins      map[ast.Expr]string
	usedOrigins  map[ast.Expr]string
	messages     map[*ast.CallExpr]string
	flows        map[ast.Node]bool
	returned     bool
}

func emptySelectedValue() selectedValue {
	return selectedValue{origins: map[ast.Expr]string{}, traces: map[*selectionSite][]ast.Node{}}
}
func unknownSelectedValue() selectedValue {
	v := emptySelectedValue()
	v.unknown = true
	return v
}
func (v selectedValue) withFlow(node ast.Node) selectedValue {
	v.traces = maps.Clone(v.traces)
	for site, flow := range v.traces {
		if !slices.Contains(flow, node) {
			v.traces[site] = append(slices.Clone(flow), node)
		}
	}
	return v
}
func (v selectedValue) implementations() []string {
	set := map[string]bool{}
	for _, name := range v.origins {
		set[name] = true
	}
	return slices.Sorted(maps.Keys(set))
}
func mergeSelectedValues(values []selectedValue) selectedValue {
	result := emptySelectedValue()
	for _, value := range values {
		result.include(value)
	}
	return result
}
func (v *selectedValue) include(other selectedValue) {
	maps.Copy(v.origins, other.origins)
	v.unknown = v.unknown || other.unknown
	for site, flow := range other.traces {
		v.traces[site] = mergeSelectionFlow(v.traces[site], flow)
	}
}
func selectedRole(t types.Type) string {
	if t == nil {
		return ""
	}
	t = types.Unalias(t)
	if _, ok := t.(*types.TypeParam); ok {
		return ""
	}
	iface, ok := t.Underlying().(*types.Interface)
	if !ok || iface.NumMethods() == 0 {
		return ""
	}
	return canonicalRole(t, iface)
}
func structuralRole(iface *types.Interface) string {
	methods := make([]string, iface.NumMethods())
	for i := range methods {
		methods[i] = selectedMessage(iface.Method(i))
	}
	slices.Sort(methods)
	return "interface{" + strings.Join(methods, ";") + "}"
}
func selectedMessage(method *types.Func) string {
	name := method.Name()
	if !method.Exported() && method.Pkg() != nil {
		name = method.Pkg().Path() + "." + name
	}
	return name + strings.TrimPrefix(canonicalType(method.Type(), nil), "func")
}
func selectedImplementation(t types.Type) string {
	if t == nil {
		return ""
	}
	t = types.Unalias(t)
	if ptr, ok := t.(*types.Pointer); ok {
		t = types.Unalias(ptr.Elem())
	}
	named, ok := t.(*types.Named)
	if !ok || !concreteVariantType(named) || hasUnfixedArguments(named) {
		return ""
	}
	return canonicalType(named, nil)
}
func hasUnfixedArguments(named *types.Named) bool {
	for i := 0; i < named.TypeArgs().Len(); i++ {
		if containsTypeParameter(named.TypeArgs().At(i)) {
			return true
		}
	}
	return false
}
func containsTypeParameter(t types.Type) bool {
	t = types.Unalias(t)
	if _, ok := t.(*types.TypeParam); ok {
		return true
	}
	if named, ok := t.(*types.Named); ok {
		return hasUnfixedArguments(named)
	}
	for _, part := range typeComponents(t) {
		if containsTypeParameter(part) {
			return true
		}
	}
	return false
}

func canonicalRole(t types.Type, iface *types.Interface) string {
	if _, named := t.(*types.Named); named {
		return canonicalType(t, nil)
	}
	return structuralRole(iface)
}

func mergeSelectionFlow(left, right []ast.Node) []ast.Node {
	result := slices.Clone(left)
	for _, node := range right {
		if !slices.Contains(result, node) {
			result = append(result, node)
		}
	}
	return result
}

func isInterfaceValue(t types.Type) bool {
	if t == nil {
		return false
	}
	t = types.Unalias(t)
	if _, parameter := t.(*types.TypeParam); parameter {
		return false
	}
	_, ok := t.Underlying().(*types.Interface)
	return ok
}
