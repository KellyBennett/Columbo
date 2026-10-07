package columbo

import (
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"sort"
	"strconv"
)

func (scan *guardedUpdateScan) site(node *ast.IfStmt, update *ast.IncDecStmt, guard ast.Expr) advisorySite {
	site := scan.owner.guardedSite(node.Cond, update, guard)
	site.receipts = append(scan.surroundingConditions(), site.receipts...)
	return site
}
func (owner *declaration) guardedSite(condition, update, guard ast.Node) advisorySite {
	return advisorySite{symbol: owner.symbol, representation: "guarded-update", receipts: []Source{
		owner.categoryReceipt("guarded-condition", condition, Detail{Subject: "preserve full condition and evaluation order"}),
		owner.categoryReceipt("bound-guard", guard, Detail{}),
		owner.categoryReceipt("guarded-update", update, Detail{}),
	}}
}
func (scan *guardedUpdateScan) surroundingConditions() []Source {
	receipts := []Source{}
	for _, ancestor := range scan.ancestors {
		if outer, ok := ancestor.(*ast.IfStmt); ok {
			receipts = append(receipts, scan.owner.categoryReceipt("surrounding-condition", outer.Cond, Detail{Subject: "preserve branch context"}))
		}
	}
	return receipts
}
func (key guardedUpdateKey) group(fset *token.FileSet, sites []advisorySite) advisoryGroup {
	sortGuardedSites(sites)
	fieldID := key.fieldIdentity(guardedFieldLocation(fset, key.field))
	id, _ := identity(guardedUpdateKind, fieldID, key.boundType, key.bound+key.operationText())
	return advisoryGroup{id: id, kind: guardedUpdateKind, subject: key.subject(), lead: guardedUpdateLead, limits: guardedUpdateLimits, values: key.values(fieldID), sites: sites}
}
func (key guardedUpdateKey) values(fieldID string) []advisoryValue {
	return []advisoryValue{{"resolved-field", fieldID, types.TypeString(key.field.Type(), nil)}, {"bound", key.boundType, key.bound}, {"update", "", key.operationText()}}
}
func (key guardedUpdateKey) subject() string {
	return key.field.Name() + " " + guardedComparison(key.operation).String() + " " + key.bound + "; " + key.operationText()
}
func (key guardedUpdateKey) operationText() string { return key.operation.String() + " 1" }
func (key guardedUpdateKey) fieldIdentity(location string) string {
	return key.field.Pkg().Path() + "." + key.field.Name() + "@" + location
}
func guardedFieldLocation(fset *token.FileSet, field *types.Var) string {
	return guardedLocation(fset.PositionFor(field.Pos(), false))
}
func guardedLocation(position token.Position) string {
	return filepath.Base(position.Filename) + ":" + strconv.Itoa(position.Offset)
}
func sortGuardedSites(sites []advisorySite) {
	sort.Slice(sites, func(i, j int) bool {
		left, right := sites[i].receipts[0], sites[j].receipts[0]
		if left.File != right.File {
			return left.File < right.File
		}
		return guardedUpdateOffset(sites[i]) < guardedUpdateOffset(sites[j])
	})
}
func guardedUpdateOffset(site advisorySite) int {
	return site.receipts[len(site.receipts)-1].StartOffset
}
