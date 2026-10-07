package columbo

import (
	"go/ast"
	"go/token"
	"go/types"
	"slices"
)

type guardedFacts struct{ info *types.Info }
type guardedPredicate struct {
	facts      guardedFacts
	subject    resolvedValuePath
	comparison token.Token
}

func (facts guardedFacts) match(condition ast.Expr, update *ast.IncDecStmt) (guardedUpdateKey, ast.Expr) {
	subject := facts.path(update.X)
	if !subject.valid() || !facts.pure(condition) {
		return guardedUpdateKey{}, nil
	}
	predicate := guardedPredicate{facts, subject, guardedComparison(update.Tok)}
	guard, bound := predicate.bound(condition)
	if guard == nil {
		return guardedUpdateKey{}, nil
	}
	return facts.key(subject, update.Tok, bound), guard
}
func (facts guardedFacts) key(subject resolvedValuePath, operation token.Token, bound ast.Expr) guardedUpdateKey {
	value, typ := guardedConstant(facts.info.Types[bound])
	return guardedUpdateKey{subject.fields[0], value, typ, operation}
}
func guardedConstant(value types.TypeAndValue) (string, string) {
	return value.Value.ExactString(), types.TypeString(value.Type, nil)
}
func (facts guardedFacts) path(expr ast.Expr) resolvedValuePath {
	field, ok := unparen(expr).(*ast.SelectorExpr)
	if !ok || !directGuardedField(field) || !facts.integerField(field) {
		return resolvedValuePath{}
	}
	return categoryPath(facts.info, field)
}
func directGuardedField(field *ast.SelectorExpr) bool {
	_, ok := unparen(field.X).(*ast.Ident)
	return ok
}
func (facts guardedFacts) integerField(field *ast.SelectorExpr) bool {
	selection := facts.info.Selections[field]
	if selection == nil || len(selection.Index()) != 1 {
		return false
	}
	return guardedInteger(facts.info.TypeOf(field))
}
func guardedInteger(typ types.Type) bool {
	basic, ok := typ.Underlying().(*types.Basic)
	return ok && basic.Info()&types.IsInteger != 0
}
func (facts guardedFacts) pure(expr ast.Expr) bool {
	expr = unparen(expr)
	binary, ok := expr.(*ast.BinaryExpr)
	if !ok {
		return facts.pureValue(expr)
	}
	if binary.Op == token.LOR {
		return false
	}
	if facts.pureValue(expr) {
		return true
	}
	return guardedPureOperator(binary.Op) && facts.pure(binary.X) && facts.pure(binary.Y)
}
func (facts guardedFacts) pureValue(expr ast.Expr) bool {
	return facts.info.Types[expr].Value != nil || categoryPath(facts.info, expr).valid()
}
func guardedPureOperator(op token.Token) bool {
	return slices.Contains([]token.Token{token.LAND, token.EQL, token.NEQ, token.LSS, token.GTR, token.LEQ, token.GEQ}, op)
}
func (predicate guardedPredicate) bound(expr ast.Expr) (ast.Expr, ast.Expr) {
	binary, ok := unparen(expr).(*ast.BinaryExpr)
	if !ok {
		return nil, nil
	}
	if binary.Op == token.LAND {
		return predicate.conjunction(binary)
	}
	return predicate.comparisonBound(binary)
}
func (predicate guardedPredicate) conjunction(binary *ast.BinaryExpr) (ast.Expr, ast.Expr) {
	if guard, bound := predicate.bound(binary.X); guard != nil {
		return guard, bound
	}
	return predicate.bound(binary.Y)
}
func (predicate guardedPredicate) comparisonBound(binary *ast.BinaryExpr) (ast.Expr, ast.Expr) {
	left, right, op := binary.X, binary.Y, binary.Op
	if op == predicate.comparison && predicate.operands(left, right) {
		return binary, right
	}
	if op == reversedGuard(predicate.comparison) && predicate.operands(right, left) {
		return binary, left
	}
	return nil, nil
}
func (predicate guardedPredicate) operands(subject, bound ast.Expr) bool {
	return predicate.subject.same(predicate.facts.path(subject)) && predicate.facts.info.Types[bound].Value != nil
}
func guardedComparison(operation token.Token) token.Token {
	if operation == token.DEC {
		return token.GTR
	}
	return token.LSS
}
func reversedGuard(op token.Token) token.Token {
	if op == token.LSS {
		return token.GTR
	}
	return token.LSS
}
