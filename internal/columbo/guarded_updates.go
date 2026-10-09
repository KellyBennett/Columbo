package columbo

import (
	"go/ast"
	"go/token"
	"go/types"
	"sort"
)

const guardedUpdateKind = "repeated-guarded-update"
const guardedUpdateLead = "These locations repeat the same guarded mutation schema. Consider a shared operation while preserving surrounding conditions, evaluation order and effects. Caller/helper receipts can expose the same boundary at both locations; the outer guard is not proven redundant. Distinct parameters need not denote the same runtime object."
const guardedUpdateLimits = "Lexical guarded-mutation evidence, not a universal invariant or proof of safe extraction. Direct resolved integer/string/bool fields; scalar comparisons, assignment/compound/++/-- in a multi-statement body with an unchanged check and inputs, no init/else; conjunction context retained. Consistent typed parameter roles across check and write; named constants retain identity. Earlier unknown effects/control flow or potentially relevant writes block a candidate; later statements remain context. No whole-if replacement claim. One-hop direct free-function calls with one pointer subject parameter and no results can repeat a matching helper guard/mutation; safe prefixes required, no scalar input remapping. No alias, promoted/nested field, effectful expression, OR clause, closure or transitive inference. No behavior-family inference."

type guardedUpdateKey struct {
	field        *types.Var
	check, write string
}
type guardedUpdateScan struct {
	helpers   map[string]*declaration
	engine    *engine
	owner     *declaration
	groups    map[guardedUpdateKey][]advisorySite
	ancestors []ast.Node
	fset      *token.FileSet
}

func (a *engine) guardedUpdateAdvisories() []advisoryGroup {
	sites := map[guardedUpdateKey][]advisorySite{}
	helpers := map[string]*declaration{}
	for _, owner := range a.declarations {
		if owner.file.included {
			scan := guardedUpdateScan{helpers: helpers, engine: a, owner: owner, groups: sites, fset: a.fset}
			owner.inspectBody(scan.visit)
		}
	}
	guardedHelperDeclarations(sites, helpers)
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
	if node.Init != nil || node.Else != nil {
		return
	}
	for index, statement := range node.Body.List {
		scan.collectHelper(node, statement, index)
		mutation := guardedMutationOf(statement)
		if mutation != nil && scan.owner.guardedPrefix(node, mutation.node, index) {
			scan.collectMutation(node, mutation)
		}
	}
}
func (scan *guardedUpdateScan) collectMutation(node *ast.IfStmt, mutation *guardedMutation) {
	facts := guardedFacts{scan.owner.file.typeInfo()}
	match := facts.match(node.Cond, mutation, scan.normalizer())
	if match != nil {
		scan.groups[match.key] = append(scan.groups[match.key], scan.site(node, mutation.node, match))
	}
}
func (scan *guardedUpdateScan) normalizer() *guardedNormalizer {
	return &guardedNormalizer{owner: scan.owner, fset: scan.fset}
}
