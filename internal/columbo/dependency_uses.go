package columbo

import (
	"go/ast"
	"go/types"
	"slices"
)

type dependencyUses struct {
	identities map[string]map[string]bool
	receipts   map[string]map[string]Source
}

func newDependencyUses() *dependencyUses {
	return &dependencyUses{identities: map[string]map[string]bool{}, receipts: map[string]map[string]Source{}}
}

func (u *dependencyUses) record(origin, identity string, receipt Source) {
	if u.identities[origin] == nil {
		u.identities[origin] = map[string]bool{}
		u.receipts[origin] = map[string]Source{}
	}
	u.identities[origin][identity] = true
	u.receipts[origin][canonical(receipt)] = receipt
}

func (u *dependencyUses) origins() []string {
	origins := map[string]bool{}
	for origin := range u.identities {
		origins[origin] = true
	}
	return sortedSet(origins)
}

func (d *declaration) dependencyUseClues() []Clue {
	out := []Clue{}
	for _, origin := range d.depUses.origins() {
		identities := d.scoredUses(origin)
		if len(identities) > 0 {
			out = append(out, d.useClue(origin, identities))
		}
	}
	if identities := d.signatureOnlyUses(); len(identities) > 0 {
		out = append(out, d.useClue("signature-only", identities))
	}
	return out
}

func (d *declaration) scoredUses(origin string) []string {
	identities := map[string]bool{}
	for identity := range d.depUses.identities[origin] {
		if d.deps[identity] {
			identities[identity] = true
		}
	}
	return sortedSet(identities)
}

func (d *declaration) signatureOnlyUses() []string {
	identities := map[string]bool{}
	for _, identity := range d.scoredUses("signature") {
		if d.depUses.onlySignature(identity) {
			identities[identity] = true
		}
	}
	return sortedSet(identities)
}

func (u *dependencyUses) onlySignature(identity string) bool {
	for origin, identities := range u.identities {
		if origin != "signature" && identities[identity] {
			return false
		}
	}
	return true
}

func (d *declaration) useClue(origin string, identities []string) Clue {
	ref := d.ref()
	receipts := d.useReceipts(origin, identities)
	return metric("dependency-use-"+origin, d.symbol, identities).forDeclaration(&ref).supportedBy(receipts)
}

func (d *declaration) useReceipts(origin string, identities []string) []Source {
	if origin == "signature-only" {
		origin = "signature"
	}
	out := []Source{}
	for _, receipt := range d.depUses.receipts[origin] {
		if slices.Contains(identities, receipt.DependencyIdentity) {
			receipt.DependencyScored = true
			out = append(out, receipt)
		}
	}
	return out
}

func (s *dependencyScan) selectedType(node *ast.SelectorExpr, typ types.Type) {
	origin := "value"
	if _, signature := typ.(*types.Signature); signature {
		origin = "signature"
	}
	s.add(node, typ, origin)
}

func (s *dependencyScan) callResults(call *ast.CallExpr) {
	typ := s.info.TypeOf(call)
	s.add(call, typ, s.resultOrigin(call, 0))
	for index, result := range callResultComponents(typ) {
		s.annotateResult(call, result, s.resultOrigin(call, index))
	}
}

func (s *dependencyScan) annotateResult(call *ast.CallExpr, typ types.Type, origin string) {
	collector := *s.collector
	collector.identities = map[string]bool{}
	collector.packages = map[string]bool{}
	collector.walk(typ)
	for identity := range collector.identities {
		if s.declaration.deps[identity] {
			s.record(call, identity, origin)
		}
	}
}

func (s *dependencyScan) resultOrigin(call *ast.CallExpr, index int) string {
	if len(s.path) < 2 {
		return "consumed"
	}
	parent, expression := s.resultContext(call)
	if resultDiscarded(parent, expression, index) {
		return "discarded"
	}
	return "consumed"
}

func resultDiscarded(parent ast.Node, call ast.Expr, index int) bool {
	if discardedInvocation(parent) {
		return true
	}
	switch parent := parent.(type) {
	case *ast.AssignStmt:
		return discardedAssignment(parent, call, index)
	case *ast.ValueSpec:
		return discardedDeclaration(parent, call, index)
	}
	return false
}

func discardedAssignment(parent *ast.AssignStmt, call ast.Expr, index int) bool {
	index = resultPosition(parent.Rhs, call, index)
	if index >= len(parent.Lhs) {
		return false
	}
	name, ok := parent.Lhs[index].(*ast.Ident)
	return ok && name.Name == "_"
}

func discardedDeclaration(parent *ast.ValueSpec, call ast.Expr, index int) bool {
	index = resultPosition(parent.Values, call, index)
	return index < len(parent.Names) && parent.Names[index].Name == "_"
}

func resultPosition(expressions []ast.Expr, call ast.Expr, index int) int {
	if len(expressions) == 1 {
		return index
	}
	for position, expression := range expressions {
		if expression == call {
			return position
		}
	}
	return len(expressions)
}

func discardedInvocation(parent ast.Node) bool {
	switch parent.(type) {
	case *ast.ExprStmt, *ast.GoStmt, *ast.DeferStmt:
		return true
	}
	return false
}

func (s *dependencyScan) resultContext(call ast.Expr) (ast.Node, ast.Expr) {
	for position := len(s.path) - 2; position >= 0; position-- {
		parent := s.path[position]
		if wrapper, ok := parent.(*ast.ParenExpr); ok {
			call = wrapper
			continue
		}
		return parent, call
	}
	return nil, call
}
