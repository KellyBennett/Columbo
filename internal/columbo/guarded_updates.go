package columbo

import (
	"go/ast"
	"go/token"
	"go/types"
	"sort"
)

const guardedUpdateKind = "repeated-guarded-update"
const guardedUpdateLead = "These locations repeat the same guarded update schema. Consider a shared operation while preserving surrounding conditions, evaluation order and effects. Distinct parameters need not denote the same runtime object."
const guardedUpdateLimits = "Lexical evidence only, not a universal domain invariant or proof that extraction or unconditional clamping is safe. Direct resolved integer fields, strict constant bound, ++/--, single-statement body, no init/else; pure conjunctions only. No alias, promoted/nested field, call, OR, closure or interprocedural inference. No behavior-family membership inferred from names or historical reports."

type guardedUpdateKey struct {
	field            *types.Var
	bound, boundType string
	operation        token.Token
}
type guardedUpdateScan struct {
	owner     *declaration
	groups    map[guardedUpdateKey][]advisorySite
	ancestors []ast.Node
}

func (a *engine) guardedUpdateAdvisories() []advisoryGroup {
	sites := map[guardedUpdateKey][]advisorySite{}
	for _, owner := range a.declarations {
		if owner.file.included {
			scan := guardedUpdateScan{owner: owner, groups: sites}
			owner.inspectBody(scan.visit)
		}
	}
	return guardedUpdateGroups(a.fset, sites)
}
func guardedUpdateGroups(fset *token.FileSet, sites map[guardedUpdateKey][]advisorySite) []advisoryGroup {
	groups := []advisoryGroup{}
	for key, occurrences := range sites {
		if len(occurrences) >= 2 {
			groups = append(groups, key.group(fset, occurrences))
		}
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].id < groups[j].id })
	return groups
}
func (scan *guardedUpdateScan) visit(node ast.Node) bool {
	if node == nil {
		scan.ancestors = scan.ancestors[:len(scan.ancestors)-1]
		return false
	}
	if _, closure := node.(*ast.FuncLit); closure {
		return false
	}
	if decision, ok := node.(*ast.IfStmt); ok {
		scan.collect(decision)
	}
	scan.ancestors = append(scan.ancestors, node)
	return true
}
func (scan *guardedUpdateScan) collect(node *ast.IfStmt) {
	update := guardedStatement(node)
	if update == nil {
		return
	}
	facts := guardedFacts{scan.owner.file.typeInfo()}
	key, guard := facts.match(node.Cond, update)
	if guard != nil {
		scan.groups[key] = append(scan.groups[key], scan.site(node, update, guard))
	}
}
func guardedStatement(node *ast.IfStmt) *ast.IncDecStmt {
	if node.Init != nil || node.Else != nil || len(node.Body.List) != 1 {
		return nil
	}
	update, _ := node.Body.List[0].(*ast.IncDecStmt)
	return update
}
