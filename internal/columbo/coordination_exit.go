package columbo

import (
	"go/ast"
	"go/token"
	"go/types"
)

func (p coordinationPair) excludesContinuation(key string) bool {
	left, ok := p.left.node.(*ast.IfStmt)
	if !ok || left.Else != nil || len(left.Body.List) == 0 {
		return false
	}
	body := p.scan.owner.fn.Body
	variable := coordinationIsolatedSlot(body, p.scan.info, key)
	if variable == nil {
		return false
	}
	last := left.Body.List[len(left.Body.List)-1]
	return coordinationTerminalExcludes(body, last, p.right.node, variable) &&
		coordinationNoBypass(body, left.Body, last)
}

func coordinationIsolatedSlot(body *ast.BlockStmt, info *types.Info, key string) *types.Var {
	var variable *types.Var
	for _, obj := range info.Defs {
		v, ok := obj.(*types.Var)
		if ok && coordinationKey(resolvedValuePath{root: v}) == key {
			variable = v
			break
		}
	}
	if variable == nil || variable.Pos() <= body.Pos() || variable.Pos() >= body.End() {
		return nil
	}
	if coordinationExitUnknown(body, info, variable) {
		return nil
	}
	return variable
}

func coordinationTerminalExcludes(body *ast.BlockStmt, last ast.Stmt, later ast.Node, variable *types.Var) bool {
	switch exit := last.(type) {
	case *ast.ReturnStmt:
	case *ast.BranchStmt:
		if exit.Tok != token.BREAK {
			return false
		}
		target := coordinationBreakTarget(body, exit)
		if target == nil || !coordinationContains(target, later) {
			return false
		}
		if (variable.Pos() <= target.Pos() || variable.Pos() >= target.End()) && coordinationOuterLoop(body, target) {
			return false
		}
	default:
		return false
	}
	return true
}

func coordinationNoBypass(body, branchBody *ast.BlockStmt, last ast.Stmt) bool {
	safe := true
	ast.Inspect(branchBody, func(node ast.Node) bool {
		if coordinationClosure(node) {
			return false
		}
		if branch, ok := node.(*ast.BranchStmt); ok && branch != last {
			target := coordinationBreakTarget(body, branch)
			if branch.Tok != token.BREAK || target == nil || !coordinationContains(branchBody, target) {
				safe = false
			}
		}
		return safe
	})
	return safe
}

func coordinationExitUnknown(body *ast.BlockStmt, info *types.Info, variable *types.Var) bool {
	unknown := false
	ast.Inspect(body, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.FuncLit:
			unknown = unknown || coordinationReferences(n, info, variable)
			return false
		case *ast.SelectorExpr:
			unknown = unknown || coordinationPointerReceiverExposes(n, info, variable)
		case *ast.UnaryExpr:
			unknown = unknown || (n.Op == token.AND && coordinationAddressedSlot(n.X, info, variable))
		case *ast.BranchStmt:
			unknown = unknown || n.Tok == token.GOTO || n.Tok == token.CONTINUE || n.Tok == token.FALLTHROUGH
		}
		return !unknown
	})
	return unknown
}

func coordinationPointerReceiverExposes(expr *ast.SelectorExpr, info *types.Info, variable *types.Var) bool {
	selection := info.Selections[expr]
	if selection == nil || selection.Kind() == types.FieldVal {
		return false
	}
	signature, ok := selection.Obj().Type().(*types.Signature)
	if !ok || signature.Recv() == nil {
		return false
	}
	_, pointer := signature.Recv().Type().(*types.Pointer)
	return pointer && coordinationReferences(expr.X, info, variable)
}

func coordinationAddressedSlot(expr ast.Expr, info *types.Info, variable *types.Var) bool {
	operand := ast.Unparen(expr)
	if value, ok := info.Types[operand]; ok && !value.Addressable() {
		return false
	}
	return coordinationReferences(operand, info, variable)
}

func coordinationReferences(node ast.Node, info *types.Info, variable *types.Var) bool {
	found := false
	ast.Inspect(node, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && info.ObjectOf(id) == variable {
			found = true
		}
		return !found
	})
	return found
}

func coordinationContains(outer, inner ast.Node) bool {
	return outer.Pos() <= inner.Pos() && inner.End() <= outer.End()
}

func coordinationBreakTarget(body *ast.BlockStmt, branch *ast.BranchStmt) ast.Node {
	if branch.Tok != token.BREAK {
		return nil
	}
	var path []ast.Node
	ast.Inspect(body, func(node ast.Node) bool {
		if node == nil || !coordinationContains(node, branch) {
			return false
		}
		path = append(path, node)
		return node != branch
	})
	for i := len(path) - 1; i >= 0; i-- {
		node := path[i]
		if branch.Label != nil {
			label, ok := node.(*ast.LabeledStmt)
			if !ok || label.Label.Name != branch.Label.Name {
				continue
			}
			node = label.Stmt
		}
		switch node.(type) {
		case *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
			return node
		}
		if branch.Label != nil {
			return nil
		}
	}
	return nil
}

func coordinationOuterLoop(body *ast.BlockStmt, target ast.Node) bool {
	found := false
	ast.Inspect(body, func(node ast.Node) bool {
		if node == nil || node == target || !coordinationContains(node, target) {
			return false
		}
		switch node.(type) {
		case *ast.ForStmt, *ast.RangeStmt:
			found = true
		}
		return !found
	})
	return found
}
