package columbo

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"
)

func (owner *declaration) categoryReceipt(kind string, node ast.Node, detail Detail) Source {
	receipt := owner.source(kind, node.Pos(), node.End(), detail)
	receipt.Spelling = string(owner.file.data[receipt.StartOffset:receipt.EndOffset])
	return receipt
}
func (scan *categoryBehaviorScan) selectionSite(selection categorySelection, actions []categoryAction) advisorySite {
	site := advisorySite{symbol: scan.owner.symbol, representation: "direct-execution"}
	site.receipts = append(site.receipts, scan.owner.categoryReceipt("category-selector", selection.selector, Detail{Subject: "selector"}))
	for _, branch := range selection.branches {
		label := categoryLabels(branch, scan.owner.file.typeInfo())
		site.receipts = append(site.receipts, scan.owner.categoryReceipt("category-branch", branch.node, Detail{Subject: label}))
		site.receipts = append(site.receipts, scan.branchReceipt(branch, label, actions))
	}
	return site
}
func (scan *categoryBehaviorScan) branchReceipt(branch categoryBranch, label string, actions []categoryAction) Source {
	for _, action := range actions {
		if action.branch.node == branch.node {
			return scan.owner.categoryReceipt("category-call", action.call, Detail{Subject: label + " -> " + action.target.symbol})
		}
	}
	return scan.owner.categoryReceipt("category-no-op", branch.noOpNode(), Detail{Subject: label + " -> explicit no-op/control transfer"})
}
func categoryLabels(branch categoryBranch, info *types.Info) string {
	if len(branch.labels) == 0 {
		return "default/else"
	}
	labels := []string{}
	for _, label := range branch.labels {
		labels = append(labels, info.Types[label].Value.ExactString())
	}
	return strings.Join(labels, ", ")
}
func (d *declaration) categoryCalleeSite(position int) advisorySite {
	scan := newCategoryStateScan(d, position)
	d.inspectBody(scan.markWrites)
	d.inspectBody(scan.access)
	return advisorySite{symbol: d.symbol, representation: "selected-callee", receipts: append([]Source{d.declReceipt()}, scan.receipts...)}
}

type categoryStateScan struct {
	owner     *declaration
	parameter *types.Var
	writes    map[*ast.SelectorExpr]bool
	receipts  []Source
}

func (scan *categoryStateScan) markWrites(node ast.Node) bool {
	if _, closure := node.(*ast.FuncLit); closure {
		return false
	}
	if assignment, ok := node.(*ast.AssignStmt); ok {
		for _, lhs := range assignment.Lhs {
			scan.markWrite(lhs)
		}
	}
	if increment, ok := node.(*ast.IncDecStmt); ok {
		scan.markWrite(increment.X)
	}
	return true
}
func (scan *categoryStateScan) markWrite(expr ast.Expr) {
	if field, ok := ast.Unparen(expr).(*ast.SelectorExpr); ok {
		scan.writes[field] = true
	}
}
func (scan *categoryStateScan) access(node ast.Node) bool {
	if _, closure := node.(*ast.FuncLit); closure {
		return false
	}
	if field, ok := node.(*ast.SelectorExpr); ok {
		scan.fieldAccess(field)
	}
	return true
}
func (scan *categoryStateScan) fieldAccess(field *ast.SelectorExpr) {
	path := scan.subjectPath(field)
	if !path.valid() {
		return
	}
	receipt := scan.owner.categoryReceipt(scan.accessKind(field), field, Detail{Subject: scan.subjectKey(path)})
	scan.receipts = append(scan.receipts, receipt)
}
func (scan *categoryStateScan) subjectPath(field *ast.SelectorExpr) resolvedValuePath {
	path := categoryPath(scan.owner.file.typeInfo(), field)
	if path.root != scan.parameter || len(path.fields) == 0 {
		return resolvedValuePath{}
	}
	return path
}
func (scan *categoryStateScan) accessKind(field *ast.SelectorExpr) string {
	if scan.writes[field] {
		return "subject-state-write"
	}
	return "subject-state-access"
}
func (scan *categoryStateScan) subjectKey(path resolvedValuePath) string {
	return path.key(types.TypeString(scan.parameter.Type(), nil))
}
func (branch categoryBranch) noOpNode() ast.Node {
	if len(branch.body) != 0 {
		return branch.body[0]
	}
	return branch.node
}

type categoryStateSupport map[string]map[string]bool

func (group *advisoryGroup) addSharedCategoryState() {
	counts := categoryStateSupport{}
	for _, site := range group.sites {
		counts.addSite(site)
	}
	for _, key := range counts.shared() {
		group.values = append(group.values, advisoryValue{"shared-subject-state", key, "supporting evidence only"})
	}
}
func (counts categoryStateSupport) addSite(site advisorySite) {
	for _, receipt := range site.receipts {
		if receipt.Kind == "subject-state-access" || receipt.Kind == "subject-state-write" {
			counts.add(receipt.Detail.Subject, site.symbol)
		}
	}
}
func (counts categoryStateSupport) add(key, symbol string) {
	if counts[key] == nil {
		counts[key] = map[string]bool{}
	}
	counts[key][symbol] = true
}
func (counts categoryStateSupport) shared() []string {
	shared := map[string]bool{}
	for key, callees := range counts {
		if len(callees) >= 2 {
			shared[key] = true
		}
	}
	return sortedSet(shared)
}

func categoryActionResults(signature *types.Signature) bool {
	for index := 0; index < signature.Results().Len(); index++ {
		if _, ok := signature.Results().At(index).Type().Underlying().(*types.Basic); !ok {
			return false
		}
	}
	return true
}
func categoryArguments(info *types.Info, call *ast.CallExpr) bool {
	if call.Ellipsis != token.NoPos {
		return false
	}
	for _, argument := range call.Args {
		if !categoryArgument(info, argument) {
			return false
		}
	}
	return true
}

func newCategoryStateScan(owner *declaration, position int) *categoryStateScan {
	return &categoryStateScan{owner: owner, parameter: owner.signature.Params().At(position), writes: map[*ast.SelectorExpr]bool{}}
}

func categoryArgument(info *types.Info, argument ast.Expr) bool {
	return categoryPath(info, argument).valid() || info.Types[argument].Value != nil
}
