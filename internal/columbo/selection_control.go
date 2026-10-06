package columbo

import (
	"go/ast"
	"go/token"
	"go/types"
	"maps"
	"reflect"
	"slices"
	"strings"
)

type selectionPath struct {
	env  selectionEnv
	exit token.Token
}

func (s *selectionScan) block(list []ast.Stmt, env selectionEnv) []selectionPath {
	paths := []selectionPath{{env: env}}
	for _, stmt := range list {
		paths = s.advance(stmt, paths)
	}
	return paths
}
func (s *selectionScan) advance(stmt ast.Stmt, paths []selectionPath) []selectionPath {
	var next []selectionPath
	for _, path := range paths {
		if path.exit != token.ILLEGAL {
			next = append(next, path)
			continue
		}
		next = append(next, s.statement(stmt, path.env)...)
	}
	return next
}

type selectionStep func(*selectionScan, ast.Stmt, selectionEnv) []selectionPath

// One dispatch table owns statement classification. Each handler owns the
// transfer semantics of its syntax, rather than repeating a variant switch.
var selectionSteps = map[reflect.Type]selectionStep{
	reflect.TypeOf((*ast.IfStmt)(nil)): func(s *selectionScan, n ast.Stmt, e selectionEnv) []selectionPath {
		return s.ifStatement(n.(*ast.IfStmt), e)
	},
	reflect.TypeOf((*ast.SwitchStmt)(nil)): func(s *selectionScan, n ast.Stmt, e selectionEnv) []selectionPath {
		return s.switchStatement(n.(*ast.SwitchStmt), e)
	},
	reflect.TypeOf((*ast.BlockStmt)(nil)): func(s *selectionScan, n ast.Stmt, e selectionEnv) []selectionPath {
		return s.block(n.(*ast.BlockStmt).List, e)
	},
	reflect.TypeOf((*ast.ReturnStmt)(nil)): func(s *selectionScan, n ast.Stmt, e selectionEnv) []selectionPath {
		s.returns(n.(*ast.ReturnStmt), e)
		return nil
	},
	reflect.TypeOf((*ast.BranchStmt)(nil)): func(s *selectionScan, n ast.Stmt, e selectionEnv) []selectionPath {
		return []selectionPath{{env: e, exit: n.(*ast.BranchStmt).Tok}}
	},
	reflect.TypeOf((*ast.ExprStmt)(nil)):       (*selectionScan).expressionStatement,
	reflect.TypeOf((*ast.AssignStmt)(nil)):     (*selectionScan).assignmentStatement,
	reflect.TypeOf((*ast.DeclStmt)(nil)):       (*selectionScan).declarationStatement,
	reflect.TypeOf((*ast.ForStmt)(nil)):        (*selectionScan).forStatement,
	reflect.TypeOf((*ast.RangeStmt)(nil)):      (*selectionScan).rangeStatement,
	reflect.TypeOf((*ast.TypeSwitchStmt)(nil)): (*selectionScan).unsupportedStatement,
	reflect.TypeOf((*ast.SelectStmt)(nil)):     (*selectionScan).unsupportedStatement,
}

func (s *selectionScan) statement(stmt ast.Stmt, env selectionEnv) []selectionPath {
	if step := s.steps[reflect.TypeOf(stmt)]; step != nil {
		return step(s, stmt, env)
	}
	s.calls(stmt, env)
	return []selectionPath{{env: env}}
}
func (s *selectionScan) expressionStatement(stmt ast.Stmt, env selectionEnv) []selectionPath {
	expr := stmt.(*ast.ExprStmt).X
	s.calls(expr, env)
	if s.isPanic(expr) {
		return nil
	}
	return []selectionPath{{env: env}}
}
func (s *selectionScan) assignmentStatement(stmt ast.Stmt, env selectionEnv) []selectionPath {
	n := stmt.(*ast.AssignStmt)
	s.expressions(n.Rhs, env)
	s.assign(n.Lhs, n.Rhs, n, env)
	return []selectionPath{{env: env}}
}
func (s *selectionScan) declarationStatement(stmt ast.Stmt, env selectionEnv) []selectionPath {
	s.declaration(stmt.(*ast.DeclStmt), env)
	return []selectionPath{{env: env}}
}
func (s *selectionScan) unsupportedStatement(stmt ast.Stmt, env selectionEnv) []selectionPath {
	s.invalidateWrites(stmt, env)
	return []selectionPath{{env: env}}
}
func (s *selectionScan) forStatement(stmt ast.Stmt, env selectionEnv) []selectionPath {
	n := stmt.(*ast.ForStmt)
	loop := newSelectionLoop(n)
	return loop.run(s, env)
}
func (s *selectionScan) rangeStatement(stmt ast.Stmt, env selectionEnv) []selectionPath {
	n := stmt.(*ast.RangeStmt)
	s.calls(n.X, env)
	loop := selectionLoop{body: n.Body, mayExit: true}
	return loop.run(s, env)
}
func (s *selectionScan) ifStatement(n *ast.IfStmt, env selectionEnv) []selectionPath {
	s.statement(n.Init, env)
	paths := s.ifArms(n, env)
	return s.merge(n, paths)
}
func (s *selectionScan) ifArms(n *ast.IfStmt, env selectionEnv) []selectionPath {
	s.calls(n.Cond, env)
	paths := s.block(n.Body.List, env.clone())
	other := env.clone()
	if tail, ok := n.Else.(*ast.IfStmt); ok {
		s.statement(tail.Init, other)
		return append(paths, s.ifArms(tail, other)...)
	}
	if n.Else == nil {
		return append(paths, selectionPath{env: other})
	}
	return append(paths, s.statement(n.Else, other)...)
}
func hasSelectionBranch(node ast.Node, kind token.Token) bool {
	found := false
	ast.Inspect(node, func(n ast.Node) bool {
		if _, ok := n.(*ast.FuncLit); ok {
			return false
		}
		if branch, ok := n.(*ast.BranchStmt); ok && (branch.Tok == kind || branch.Label != nil) {
			found = true
		}
		return true
	})
	return found
}
func (s *selectionScan) switchStatement(n *ast.SwitchStmt, env selectionEnv) []selectionPath {
	s.statement(n.Init, env)
	s.calls(n.Tag, env)
	if hasSelectionBranch(n, token.FALLTHROUGH) {
		s.invalidateWrites(n, env)
		return []selectionPath{{env: env}}
	}
	paths := s.switchArms(n, env)
	for i := range paths {
		if paths[i].exit == token.BREAK {
			paths[i].exit = token.ILLEGAL
		}
	}
	return s.merge(n, paths)
}
func (s *selectionScan) switchArms(n *ast.SwitchStmt, env selectionEnv) []selectionPath {
	var paths []selectionPath
	hasDefault := false
	for _, stmt := range n.Body.List {
		arm := stmt.(*ast.CaseClause)
		hasDefault = hasDefault || arm.List == nil
		s.expressions(arm.List, env)
		paths = append(paths, s.block(arm.Body, env.clone())...)
	}
	if !hasDefault {
		paths = append(paths, selectionPath{env: env.clone()})
	}
	return paths
}
func (s *selectionScan) merge(node ast.Node, paths []selectionPath) []selectionPath {
	var continuing []selectionEnv
	var exits []selectionPath
	for _, path := range paths {
		if path.exit == token.ILLEGAL {
			continuing = append(continuing, path.env)
		} else {
			exits = append(exits, path)
		}
	}
	if len(continuing) > 0 {
		exits = append(exits, selectionPath{env: s.mergeEnvs(node, continuing)})
	}
	return exits
}
func (s *selectionScan) mergeEnvs(node ast.Node, paths []selectionEnv) selectionEnv {
	objects := map[types.Object]bool{}
	for _, env := range paths {
		for obj := range env {
			objects[obj] = true
		}
	}
	merged := selectionEnv{}
	for obj := range objects {
		merged[obj] = s.mergeLocal(node, obj, paths)
	}
	return merged
}
func (s *selectionScan) mergeLocal(node ast.Node, obj types.Object, paths []selectionEnv) selectedValue {
	values := reachingSelectedValues(obj, paths)
	value := mergeSelectedValues(values)
	role := selectedRole(obj.Type())
	if !s.probing && role != "" && !value.unknown && len(value.implementations()) >= s.minimum && differentSelectedPaths(values) {
		site := s.newSite(selectionKey{node: node, object: obj}, role, value)
		value.traces[site] = nil
	}
	return value
}
func differentSelectedPaths(values []selectedValue) bool {
	sets := map[string]bool{}
	for _, value := range values {
		implementations := value.implementations()
		if len(implementations) > 0 {
			sets[strings.Join(implementations, "\x00")] = true
		}
	}
	return len(sets) >= 2
}

type selectionKey struct {
	node   ast.Node
	object types.Object
}

func (s *selectionScan) newSite(key selectionKey, role string, value selectedValue) *selectionSite {
	if site := s.siteIndex[key]; site != nil {
		maps.Copy(site.origins, value.origins)
		return site
	}
	site := &selectionSite{owner: s.owner, decision: key.node, role: role, origins: maps.Clone(value.origins), usedOrigins: map[ast.Expr]string{}, messages: map[*ast.CallExpr]string{}, flows: map[ast.Node]bool{}}
	s.siteIndex[key] = site
	s.sites = append(s.sites, site)
	return site
}

// Write traversal is shared by unsupported-region invalidation and capture
// analysis; each caller supplies the meaning of an indirect assignment.
type selectionWrites struct {
	nested     bool
	assignment func([]ast.Expr)
}

func (w selectionWrites) visit(n ast.Node) bool {
	if _, ok := n.(*ast.FuncLit); ok && !w.nested {
		return false
	}
	if assignment, ok := n.(*ast.AssignStmt); ok {
		w.assignment(assignment.Lhs)
	}
	return true
}
func (s *selectionScan) invalidateWrites(node ast.Node, env selectionEnv) {
	writes := selectionWrites{assignment: func(left []ast.Expr) { s.invalidateLocals(left, env) }}
	ast.Inspect(node, writes.visit)
}
func (s *selectionScan) invalidateLocals(left []ast.Expr, env selectionEnv) {
	for _, expr := range left {
		if obj := s.local(expr); obj != nil {
			env[obj] = unknownSelectedValue()
		}
	}
}

func (s *selectionScan) qualifyingSites() []*selectionSite {
	sites := slices.DeleteFunc(s.sites, func(site *selectionSite) bool { return site.returned || len(site.messages) == 0 })
	for _, site := range sites {
		site.origins = site.usedOrigins
	}
	slices.SortFunc(sites, compareSelectionSites)
	return sites
}

func (s *selectionScan) isPanic(expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	return ok && s.isUniverse(call.Fun, "panic")
}

func (env selectionEnv) clone() selectionEnv { return maps.Clone(env) }
func reachingSelectedValues(obj types.Object, paths []selectionEnv) []selectedValue {
	values := make([]selectedValue, len(paths))
	for i, env := range paths {
		var ok bool
		values[i], ok = env[obj]
		if !ok {
			values[i] = unknownSelectedValue()
		}
	}
	return values
}

func compareSelectionSites(a, b *selectionSite) int {
	if a.decision.Pos() != b.decision.Pos() {
		return int(a.decision.Pos() - b.decision.Pos())
	}
	return strings.Compare(a.role, b.role)
}
