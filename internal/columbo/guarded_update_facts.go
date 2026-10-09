package columbo

import (
	"go/ast"
	"go/token"
	"go/types"
	"slices"
)

type guardedFacts struct{ info *types.Info }
type guardedMatch struct {
	key    guardedUpdateKey
	guard  ast.Expr
	inputs []guardedInput
}
type guardedPredicate struct {
	facts   guardedFacts
	subject resolvedValuePath
}

func (facts guardedFacts) match(condition ast.Expr, mutation *guardedMutation, normalizer *guardedNormalizer) *guardedMatch {
	subject := facts.path(mutation.target)
	if !subject.valid() || !facts.pure(condition) {
		return nil
	}
	predicate := guardedPredicate{facts, subject}
	guard := predicate.comparison(condition)
	if guard == nil {
		return nil
	}
	normalizer.subject = subject
	return normalizer.match(guard, mutation)
}
func (facts guardedFacts) path(expr ast.Expr) resolvedValuePath {
	field, ok := unparen(expr).(*ast.SelectorExpr)
	if !ok || !directGuardedField(field) || !facts.scalarField(field) {
		return resolvedValuePath{}
	}
	return categoryPath(facts.info, field)
}
func directGuardedField(field *ast.SelectorExpr) bool {
	_, ok := unparen(field.X).(*ast.Ident)
	return ok
}
func (facts guardedFacts) scalarField(field *ast.SelectorExpr) bool {
	selection := facts.info.Selections[field]
	if selection == nil || len(selection.Index()) != 1 {
		return false
	}
	return guardedScalar(facts.info.TypeOf(field))
}
func guardedScalar(typ types.Type) bool {
	if typ == nil {
		return false
	}
	basic, ok := typ.Underlying().(*types.Basic)
	return ok && basic.Info()&(types.IsInteger|types.IsString|types.IsBoolean) != 0
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
	return facts.info.Types[expr].Value != nil || categoryPath(facts.info, expr).valid() || facts.pureUnary(expr)
}
func guardedPureOperator(op token.Token) bool {
	return op == token.LAND || guardedComparison(op) || guardedArithmetic(op)
}
func guardedComparison(op token.Token) bool {
	return slices.Contains([]token.Token{token.EQL, token.NEQ, token.LSS, token.GTR, token.LEQ, token.GEQ}, op)
}
func guardedArithmetic(op token.Token) bool {
	return slices.Contains([]token.Token{token.ADD, token.SUB, token.MUL, token.QUO, token.REM, token.AND, token.OR, token.XOR, token.SHL, token.SHR, token.AND_NOT}, op)
}
func (predicate guardedPredicate) comparison(expr ast.Expr) *ast.BinaryExpr {
	binary, ok := unparen(expr).(*ast.BinaryExpr)
	if !ok {
		return nil
	}
	if binary.Op == token.LAND {
		return predicate.conjunction(binary)
	}
	if guardedComparison(binary.Op) && predicate.readsSubject(binary) {
		return binary
	}
	return nil
}
func (predicate guardedPredicate) conjunction(binary *ast.BinaryExpr) *ast.BinaryExpr {
	if comparison := predicate.comparison(binary.X); comparison != nil {
		return comparison
	}
	return predicate.comparison(binary.Y)
}
func (predicate guardedPredicate) readsSubject(binary *ast.BinaryExpr) bool {
	return predicate.reads(binary.X) || predicate.reads(binary.Y)
}

func (facts guardedFacts) pureUnary(expr ast.Expr) bool {
	node, ok := unparen(expr).(*ast.UnaryExpr)
	return ok && guardedUnary(node.Op) && facts.pure(node.X)
}
func (predicate guardedPredicate) reads(expr ast.Expr) bool {
	if predicate.subject.same(predicate.facts.path(expr)) {
		return true
	}
	if binary, ok := unparen(expr).(*ast.BinaryExpr); ok {
		return predicate.readsSubject(binary)
	}
	if unary, ok := unparen(expr).(*ast.UnaryExpr); ok {
		return predicate.reads(unary.X)
	}
	return false
}
