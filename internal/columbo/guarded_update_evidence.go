package columbo

import (
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"sort"
	"strconv"
)

func (scan *guardedUpdateScan) site(node *ast.IfStmt, mutation *guardedMutation, match *guardedMatch) advisorySite {
	site := scan.owner.guardedSite(node.Cond, mutation.node, match.guard)
	site.receipts = append(scan.surroundingConditions(), site.receipts...)
	site.receipts = append(site.receipts, scan.owner.guardedContext(node, match.inputs)...)
	return site
}
func (owner *declaration) guardedContext(node *ast.IfStmt, inputs []guardedInput) []Source {
	receipts := owner.guardedInputs(inputs)
	if len(node.Body.List) > 1 {
		receipts = append(receipts, owner.guardedBody(node.Body))
	}
	return receipts
}
func (owner *declaration) guardedBody(body *ast.BlockStmt) Source {
	context := Detail{Subject: "guard also controls other statements; preserve complete body and order, not a whole-if replacement"}
	return owner.categoryReceipt("guarded-body", body, context)
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
	id, _ := identity(guardedUpdateKind, fieldID, key.check, key.write)
	return advisoryGroup{id: id, kind: guardedUpdateKind, subject: key.subject(), lead: guardedUpdateLead, limits: guardedUpdateLimits, values: key.values(fieldID), sites: sites}
}
func (key guardedUpdateKey) values(fieldID string) []advisoryValue {
	return []advisoryValue{{"resolved-field", fieldID, types.TypeString(key.field.Type(), nil)}, {"normalized-check", "", key.check}, {"normalized-write", "", key.write}}
}
func (key guardedUpdateKey) subject() string { return key.field.Name() + ": repeated guarded mutation" }
func (owner *declaration) guardedInputs(inputs []guardedInput) []Source {
	receipts := []Source{}
	for _, input := range inputs {
		receipts = append(receipts, owner.categoryReceipt("mutation-input", input.node, Detail{Subject: input.role}))
	}
	return receipts
}
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
	for _, receipt := range site.receipts {
		if receipt.Kind == "guarded-update" {
			return receipt.StartOffset
		}
	}
	return 0
}
