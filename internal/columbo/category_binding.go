package columbo

import (
	"go/ast"
	"go/token"
	"go/types"
)

type categoryInvocation struct {
	call    *ast.CallExpr
	binding ast.Node
	target  *types.Func
}

func (invocation categoryInvocation) declaration(engine *engine) *declaration {
	if invocation.binding != nil {
		return engine.target(invocation.target)
	}
	return engine.calls[invocation.call]
}

func (branch categoryBranch) invocation(info *types.Info) (categoryInvocation, bool) {
	if len(branch.body) != 2 {
		call, ok := branch.action()
		return categoryInvocation{call: call}, ok
	}
	binding := categoryBinding(branch.body[0])
	return binding.invocation(info, branch.body[1])
}

type categoryFunctionBinding struct {
	name  *ast.Ident
	value ast.Expr
	node  ast.Node
}

func (binding categoryFunctionBinding) invocation(info *types.Info, statement ast.Stmt) (categoryInvocation, bool) {
	call, ok := categoryStatementAction(statement)
	if !ok || call == nil || !binding.invokedBy(info, call) {
		return categoryInvocation{}, false
	}
	target := binding.function(info)
	return categoryInvocation{call, binding.node, target}, target != nil
}

func (binding categoryFunctionBinding) invokedBy(info *types.Info, call *ast.CallExpr) bool {
	if binding.name == nil || info.Defs[binding.name] == nil {
		return false
	}
	callee, ok := ast.Unparen(call.Fun).(*ast.Ident)
	return ok && info.Uses[callee] == info.Defs[binding.name]
}

func categoryBinding(stmt ast.Stmt) categoryFunctionBinding {
	if assignment, ok := stmt.(*ast.AssignStmt); ok {
		return categoryAssignmentBinding(assignment)
	}
	if declaration, ok := stmt.(*ast.DeclStmt); ok {
		return categoryDeclaredBinding(declaration)
	}
	return categoryFunctionBinding{}
}

func categoryAssignmentBinding(assignment *ast.AssignStmt) categoryFunctionBinding {
	if assignment.Tok != token.DEFINE || len(assignment.Lhs) != 1 || len(assignment.Rhs) != 1 {
		return categoryFunctionBinding{}
	}
	name, _ := assignment.Lhs[0].(*ast.Ident)
	return categoryFunctionBinding{name, assignment.Rhs[0], assignment}
}

func categoryDeclaredBinding(declaration *ast.DeclStmt) categoryFunctionBinding {
	group, ok := declaration.Decl.(*ast.GenDecl)
	if !ok || len(group.Specs) != 1 {
		return categoryFunctionBinding{}
	}
	return categoryValueBinding(group.Specs[0], declaration)
}

func categoryValueBinding(spec ast.Spec, node ast.Node) categoryFunctionBinding {
	value, ok := spec.(*ast.ValueSpec)
	if !ok || len(value.Names) != 1 || len(value.Values) != 1 {
		return categoryFunctionBinding{}
	}
	return categoryFunctionBinding{value.Names[0], value.Values[0], node}
}

func (binding categoryFunctionBinding) function(info *types.Info) *types.Func {
	name := calleeIdentifier(binding.value)
	if name == nil {
		return nil
	}
	return (&callResolver{info}).identifier(name)
}
