package columbo

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"sort"
)

const overwriteKind = "witnessed-conditional-overwrite"
const overwriteLead = "For the displayed input witness, both sequential guards execute and the later complete scalar assignment overwrites a different earlier value at the same resolved local destination. Review the ordering and intended policy before untangling behavior. Deliberate override policies are legitimate and can have identical evidence; this is not a design-defect diagnosis or a stage blocker."
const overwriteLimits = "Existential local-path witness, not public-API reachability, exhaustive overlap analysis, or proof of disjointness when absent. Search: at most two resolved signed-integer parameters/direct receiver fields, each -2 through 8; at most 32 direct constant assignments inside top-level ifs and 256 statements. Guards: constants, integer comparisons, conjunctions and checked subtraction; no overflow witnesses (int is conservatively bounded to signed 32-bit on every architecture). Receiver assumed nonnil. Executed prefixes must evaluate; unsupported paths are skipped. Unknown calls, aliases/escapes, input mutation and unsupported control flow invalidate the function. Complete scalar constant writes to local destinations only. No assertion about final return value, safe refactoring or intended precedence."

type overwriteInput struct {
	root  types.Object
	field types.Object
}
type overwriteCandidate struct {
	guard  *ast.IfStmt
	write  *ast.AssignStmt
	target types.Object
	value  constant.Value
}
type overwriteScan struct {
	owner      *declaration
	info       *types.Info
	inputs     []overwriteInput
	labels     map[overwriteInput]string
	candidates []overwriteCandidate
}

func (a *engine) overwriteAdvisories() []advisoryGroup {
	groups := []advisoryGroup{}
	for _, d := range a.declarations {
		if !d.file.included || d.fn.Body == nil {
			continue
		}
		scan := overwriteScan{owner: d, info: d.file.typeInfo(), labels: map[overwriteInput]string{}}
		if scan.prepare() {
			groups = append(groups, scan.groups()...)
		}
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].id < groups[j].id })
	return groups
}
func (s *overwriteScan) prepare() bool {
	if !s.supportedBody() || len(s.inputs) == 0 || len(s.inputs) > 2 || !s.stable() {
		return false
	}
	for _, stmt := range s.owner.fn.Body.List {
		s.collectCandidates(stmt)
	}
	return len(s.candidates) >= 2 && len(s.candidates) <= 32
}
func (s *overwriteScan) supportedBody() bool {
	valid, count := true, 0
	ast.Inspect(s.owner.fn.Body, func(n ast.Node) bool {
		if _, ok := n.(ast.Stmt); ok {
			count++
		}
		valid = valid && count <= 256 && s.supportedNode(n)
		return valid
	})
	return valid
}
func (s *overwriteScan) supportedNode(n ast.Node) bool {
	switch n := n.(type) {
	case *ast.CallExpr, *ast.FuncLit, *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt, *ast.GoStmt, *ast.DeferStmt, *ast.BranchStmt, *ast.LabeledStmt, *ast.SendStmt, *ast.IncDecStmt:
		return false
	case *ast.UnaryExpr:
		return n.Op != token.AND && n.Op != token.ARROW
	case *ast.IfStmt:
		s.collectInputs(n.Cond)
		return n.Init == nil && n.Else == nil
	}
	return true
}
func (s *overwriteScan) collectInputs(expr ast.Expr) {
	ast.Inspect(expr, func(n ast.Node) bool {
		e, ok := n.(ast.Expr)
		if !ok {
			return true
		}
		key, ok := s.input(e)
		if !ok {
			return true
		}
		s.addInput(key)
		return false
	})
}
func (s *overwriteScan) addInput(key overwriteInput) {
	if _, seen := s.labels[key]; seen {
		return
	}
	s.labels[key] = key.label()
	s.inputs = append(s.inputs, key)
}
func (key overwriteInput) label() string {
	if key.field != nil {
		return key.root.Name() + "." + key.field.Name()
	}
	return key.root.Name()
}
func (key overwriteInput) typ() types.Type {
	if key.field != nil {
		return key.field.Type()
	}
	return key.root.Type()
}
func (s *overwriteScan) input(e ast.Expr) (overwriteInput, bool) {
	if !overwriteInteger(s.info.TypeOf(e)) {
		return overwriteInput{}, false
	}
	key := visitReference(e, s)
	return key, key.root != nil
}
func (s *overwriteScan) identifier(id *ast.Ident) overwriteInput {
	obj := s.info.ObjectOf(id)
	if s.owner.hasParameter(obj) {
		return overwriteInput{root: obj}
	}
	return overwriteInput{}
}
func (s *overwriteScan) selector(e *ast.SelectorExpr) overwriteInput {
	root, ok := e.X.(*ast.Ident)
	sel := s.info.Selections[e]
	if !ok || sel == nil || sel.Kind() != types.FieldVal || len(sel.Index()) != 1 {
		return overwriteInput{}
	}
	if s.info.ObjectOf(root) != s.owner.signature.Recv() {
		return overwriteInput{}
	}
	return overwriteInput{s.info.ObjectOf(root), sel.Obj()}
}
func overwriteInteger(t types.Type) bool {
	if t == nil {
		return false
	}
	b, ok := t.Underlying().(*types.Basic)
	return ok && b.Info()&types.IsInteger != 0 && b.Info()&types.IsUnsigned == 0
}
func (s *overwriteScan) collectCandidates(stmt ast.Stmt) {
	guard, ok := stmt.(*ast.IfStmt)
	if !ok || guard.Init != nil || guard.Else != nil {
		return
	}
	for _, child := range guard.Body.List {
		if candidate := s.candidate(guard, child); candidate != nil {
			s.candidates = append(s.candidates, *candidate)
		}
	}
}
func (s *overwriteScan) candidate(guard *ast.IfStmt, stmt ast.Stmt) *overwriteCandidate {
	write, ok := stmt.(*ast.AssignStmt)
	if !ok || write.Tok != token.ASSIGN {
		return nil
	}
	assignment := s.decodeAssignment(write)
	if assignment == nil || !s.local(assignment.target) {
		return nil
	}
	value := s.constant(assignment.value)
	if value == nil {
		return nil
	}
	return &overwriteCandidate{guard, write, assignment.target, value}
}

func overwriteScalar(v constant.Value) bool {
	return v.Kind() == constant.String || v.Kind() == constant.Int || v.Kind() == constant.Bool
}
func (s *overwriteScan) local(obj types.Object) bool {
	if _, ok := obj.(*types.Var); !ok {
		return false
	}
	return obj.Pos() > s.owner.fn.Body.Pos() && obj.Pos() < s.owner.fn.Body.End()
}
func (s *overwriteScan) stable() bool {
	valid := true
	ast.Inspect(s.owner.fn.Body, func(n ast.Node) bool {
		if write, ok := n.(*ast.AssignStmt); ok {
			valid = valid && s.safeAssignment(write)
		}
		return valid
	})
	return valid
}
func (s *overwriteScan) safeAssignment(write *ast.AssignStmt) bool {
	for _, lhs := range write.Lhs {
		if !s.safeTarget(lhs) {
			return false
		}
	}
	return true
}
func (s *overwriteScan) safeTarget(e ast.Expr) bool {
	if id, ok := e.(*ast.Ident); ok {
		return id.Name == "_" || s.local(s.info.ObjectOf(id))
	}
	sel, ok := e.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	key := s.selector(sel)
	if key.root == nil {
		return false
	}
	_, input := s.labels[key]
	return !input
}
