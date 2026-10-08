package columbo

import (
	"encoding/json"
	"go/ast"
	"go/token"
	"go/types"
	"strconv"
)

type guardedInput struct {
	variable *types.Var
	node     ast.Expr
	role     string
}
type guardedNormalizer struct {
	owner   *declaration
	fset    *token.FileSet
	subject resolvedValuePath
	inputs  []guardedInput
}

func (normalizer *guardedNormalizer) match(guard *ast.BinaryExpr, mutation *guardedMutation) *guardedMatch {
	check := normalizer.check(guard)
	write := mutation.write(normalizer)
	if check == "" || write == "" {
		return nil
	}
	key := guardedUpdateKey{normalizer.subject.fields[0], check, write}
	return &guardedMatch{key, guard, normalizer.inputs}
}
func (normalizer *guardedNormalizer) check(guard *ast.BinaryExpr) string {
	left, right, op := guard.X, guard.Y, guard.Op
	if normalizer.isSubject(right) {
		left, right, op = right, left, guardedReverse[op]
	}
	return normalizer.binaryForm(guard, op, normalizer.expression(left), normalizer.expression(right))
}

var guardedReverse = map[token.Token]token.Token{
	token.EQL: token.EQL, token.NEQ: token.NEQ, token.LSS: token.GTR,
	token.GTR: token.LSS, token.LEQ: token.GEQ, token.GEQ: token.LEQ,
}

func (normalizer *guardedNormalizer) expression(expr ast.Expr) string {
	if expr == nil {
		return ""
	}
	expr = unparen(expr)
	if normalizer.isSubject(expr) {
		return "field"
	}
	if reference := visitReference(expr, normalizer); reference != "" {
		return reference
	}
	return normalizer.computed(expr)
}
func (normalizer *guardedNormalizer) computed(expr ast.Expr) string {
	if unary, ok := visitUnary(expr, normalizer.unary); ok {
		return unary
	}
	switch node := expr.(type) {
	case *ast.BinaryExpr:
		return normalizer.binary(node)
	case *ast.CallExpr:
		return normalizer.conversion(node)
	}
	return normalizer.constant(expr)
}
func (normalizer *guardedNormalizer) isSubject(expr ast.Expr) bool {
	return normalizer.subject.same(categoryPath(normalizer.owner.file.typeInfo(), expr))
}
func (normalizer *guardedNormalizer) identifier(node *ast.Ident) string {
	return normalizer.resolved(node, normalizer.owner.file.typeInfo().ObjectOf(node))
}
func (normalizer *guardedNormalizer) resolved(node ast.Expr, object types.Object) string {
	if constant, ok := object.(*types.Const); ok {
		return normalizer.namedConstant(node, constant)
	}
	variable, ok := object.(*types.Var)
	if !ok || !normalizer.parameter(variable) {
		return ""
	}
	return normalizer.input(node, variable)
}
func (normalizer *guardedNormalizer) parameter(variable *types.Var) bool {
	return normalizer.owner.hasParameter(variable) && guardedScalar(variable.Type())
}

func (normalizer *guardedNormalizer) input(node ast.Expr, variable *types.Var) string {
	for _, input := range normalizer.inputs {
		if input.variable == variable {
			return input.role
		}
	}
	role := guardedForm("input", normalizer.typeName(node), strconv.Itoa(len(normalizer.inputs)))
	normalizer.inputs = append(normalizer.inputs, guardedInput{variable, node, role})
	return role
}
func (normalizer *guardedNormalizer) binary(node *ast.BinaryExpr) string {
	if !guardedArithmetic(node.Op) {
		return ""
	}
	return normalizer.binaryForm(node, node.Op, normalizer.expression(node.X), normalizer.expression(node.Y))
}
func (normalizer *guardedNormalizer) binaryForm(node ast.Expr, op token.Token, left, right string) string {
	if left == "" || right == "" {
		return ""
	}
	return guardedForm(op.String(), normalizer.typeName(node), left, right)
}
func (normalizer *guardedNormalizer) unary(node *ast.UnaryExpr) string {
	if !guardedUnary(node.Op) {
		return ""
	}
	operand := normalizer.expression(node.X)
	if operand == "" {
		return ""
	}
	return guardedForm("unary "+node.Op.String(), normalizer.typeName(node), operand)
}
func guardedUnary(op token.Token) bool {
	return op == token.ADD || op == token.SUB || op == token.XOR || op == token.NOT
}
func (normalizer *guardedNormalizer) typeName(expr ast.Expr) string {
	return types.TypeString(normalizer.owner.file.typeInfo().TypeOf(expr), nil)
}
func guardedForm(operation, typ string, operands ...string) string {
	value, _ := json.Marshal(append([]string{operation, typ}, operands...))
	return string(value)
}
