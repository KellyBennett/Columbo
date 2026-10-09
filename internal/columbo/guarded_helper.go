package columbo

import (
	"go/ast"
	"go/token"
	"go/types"
)

type guardedHelper struct {
	scan    *guardedUpdateScan
	caller  *ast.IfStmt
	target  *declaration
	call    *ast.CallExpr
	subject resolvedValuePath
}
type guardedHelperMatch struct {
	helper *guardedHelper
	match  *guardedMatch
}

func (scan *guardedUpdateScan) collectHelper(node *ast.IfStmt, statement ast.Stmt, index int) {
	call := guardedHelperCall(statement)
	if call == nil || !scan.owner.guardedPrefix(node, call, index) {
		return
	}
	helper := scan.helper(call, node)
	if helper != nil {
		helper.collect()
	}
}
func guardedHelperCall(statement ast.Stmt) *ast.CallExpr {
	expression, ok := statement.(*ast.ExprStmt)
	if !ok {
		return nil
	}
	call, ok := unparen(expression.X).(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return nil
	}
	return call
}
func (scan *guardedUpdateScan) helper(call *ast.CallExpr, caller *ast.IfStmt) *guardedHelper {
	target := scan.directHelper(call)
	if !guardedHelperTarget(target) || target == scan.owner {
		return nil
	}
	subject := categoryPath(scan.owner.file.typeInfo(), call.Args[0])
	if !subject.valid() || len(subject.fields) != 0 {
		return nil
	}
	return &guardedHelper{scan, caller, target, call, subject}
}
func (scan *guardedUpdateScan) directHelper(call *ast.CallExpr) *declaration {
	if _, ok := unparen(call.Fun).(*ast.Ident); !ok {
		return nil
	}
	return scan.engine.calls[call]
}
func guardedHelperTarget(target *declaration) bool {
	return target != nil && target.file.included && target.hasBody() && !target.hasReceiver() && guardedHelperSignature(target.signature)
}
func guardedHelperSignature(signature *types.Signature) bool {
	if signature.Variadic() || signature.TypeParams().Len() != 0 || signature.Results().Len() != 0 {
		return false
	}
	return guardedSubjectParameter(signature.Params())
}
func guardedSubjectParameter(params *types.Tuple) bool {
	if params.Len() != 1 {
		return false
	}
	_, pointer := params.At(0).Type().(*types.Pointer)
	return pointer
}
func (helper *guardedHelper) collect() {
	prefix := helper.target.helperPrefix()
	for _, statement := range helper.target.fn.Body.List {
		if guard, ok := statement.(*ast.IfStmt); ok {
			helper.collectGuard(guard)
		}
		if !prefix.preserves(statement) {
			return
		}
	}
}
func (owner *declaration) helperPrefix() guardedPrefix {
	prefix := newGuardedPrefix(owner.file.typeInfo())
	prefix.reads[owner.signature.Params().At(0)] = true
	return prefix
}
func (helper *guardedHelper) collectGuard(guard *ast.IfStmt) {
	if guard.Init != nil || guard.Else != nil {
		return
	}
	for index, statement := range guard.Body.List {
		mutation := guardedMutationOf(statement)
		if mutation == nil || !helper.target.guardedPrefix(guard, mutation.node, index) {
			continue
		}
		helper.collectMutation(guard, mutation)
	}
}
func (helper *guardedHelper) collectMutation(guard *ast.IfStmt, mutation *guardedMutation) {
	match := helper.match(guard, mutation)
	if match == nil {
		return
	}
	boundary := guardedHelperMatch{helper: helper, match: match}
	boundary.collect()
}
func (helper *guardedHelper) match(guard *ast.IfStmt, mutation *guardedMutation) *guardedMatch {
	if !helper.target.guardedParameterTarget(mutation.target) {
		return nil
	}
	return helper.target.matchGuard(guard, mutation, helper.scan.fset)
}
func (boundary guardedHelperMatch) collect() {
	if len(boundary.match.inputs) != 0 {
		return
	}
	guard := boundary.helper.matchCaller(boundary.match.key)
	if guard == nil {
		return
	}
	boundary.helper.scan.addHelperSite(boundary.match.key, boundary.helper.site(guard))
}
func (scan *guardedUpdateScan) addHelperSite(key guardedUpdateKey, site advisorySite) {
	scan.groups[key] = append(scan.groups[key], site)
}
func (helper *guardedHelper) matchCaller(key guardedUpdateKey) ast.Expr {
	facts := guardedFacts{helper.scan.owner.file.typeInfo()}
	if !facts.pure(helper.caller.Cond) {
		return nil
	}
	predicate := guardedPredicate{facts, helper.subject.withField(key.field)}
	guard := predicate.comparison(helper.caller.Cond)
	if guard == nil || !helper.sameCheck(guard, key) {
		return nil
	}
	return guard
}
func (helper *guardedHelper) sameCheck(guard *ast.BinaryExpr, key guardedUpdateKey) bool {
	normalizer := helper.scan.normalizer()
	normalizer.subject = helper.subject.withField(key.field)
	return normalizer.check(guard) == key.check && len(normalizer.inputs) == 0
}

func (owner *declaration) matchGuard(guard *ast.IfStmt, mutation *guardedMutation, fset *token.FileSet) *guardedMatch {
	facts := guardedFacts{owner.file.typeInfo()}
	return facts.match(guard.Cond, mutation, &guardedNormalizer{owner: owner, fset: fset})
}

func (owner *declaration) guardedParameterTarget(expr ast.Expr) bool {
	facts := guardedFacts{owner.file.typeInfo()}
	return facts.path(expr).root == owner.signature.Params().At(0)
}
