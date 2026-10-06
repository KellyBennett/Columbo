package columbo

import (
	"go/ast"
	"go/types"
)

// variantTypeFacts owns eligibility and canonical identities derived from Go's
// semantic information; syntax scanners never choose a representation of types.
type variantTypeFacts struct{ info *types.Info }

func (f variantTypeFacts) role(expr ast.Expr) string { return typeVariantDomain(f.info.TypeOf(expr)) }
func (f variantTypeFacts) concreteKey(expr ast.Expr) (string, bool) {
	t := f.info.TypeOf(expr)
	if nilVariantType(t) {
		return "", true
	}
	if !concreteVariantType(t) {
		return "", false
	}
	return canonicalType(t, nil), true
}
func nilVariantType(t types.Type) bool {
	basic, ok := t.(*types.Basic)
	return ok && basic.Kind() == types.UntypedNil
}
func fieldVariantSelection(selection *types.Selection) bool {
	return selection != nil && selection.Kind() == types.FieldVal
}
