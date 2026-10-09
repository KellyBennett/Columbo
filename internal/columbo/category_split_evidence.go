package columbo

import (
	"fmt"
	"go/ast"
	"go/token"
)

type categorySplitCandidate struct {
	pair          categorySplitPair
	first, second categorySplitArm
	inputs        []string
}
type splitReceipts struct {
	owner *declaration
	site  advisorySite
}

func (c categorySplitCandidate) evidence(label string) advisoryGroup {
	group := c.group(label)
	group.values = c.values(label)
	group.sites = []advisorySite{c.first.site(c.inputs), c.second.site(c.inputs)}
	context := c.pair.context()
	if len(context.receipts) > 0 {
		group.sites = append(group.sites, context)
	}
	return group
}
func (c categorySplitCandidate) group(label string) advisoryGroup {
	owner := c.pair.scan.owner
	path := c.pair.left.path
	anchor := c.anchor(label)
	id, _ := identity(categorySplitKind, owner.file.rel, owner.symbol, anchor)
	return advisoryGroup{id: id, kind: categorySplitKind, subject: path.key(path.root.Name()), lead: categorySplitLead, limits: categorySplitLimits}
}
func (c categorySplitCandidate) anchor(label string) string {
	file := c.pair.scan.owner.file.tf
	return fmt.Sprintf("%d:%d:%d:%d:%s", file.Offset(c.pair.left.node.Pos()), file.Offset(c.pair.right.node.Pos()), file.Offset(c.first.branch.node.Pos()), file.Offset(c.second.branch.node.Pos()), label)
}
func (c categorySplitCandidate) values(label string) []advisoryValue {
	values := []advisoryValue{{"explicit-category", "", label}, {"selector-identity", "", "resolved field path only; runtime value stability unknown"}, {"policy-relationship", "", "unknown: one category behavior or independent policies"}}
	for _, input := range c.inputs {
		values = append(values, advisoryValue{"shared-lexical-input", "", c.first.effects.reads[input].Detail.Subject})
	}
	return values
}
func (a categorySplitArm) site(inputs []string) advisorySite {
	receipts := splitReceipts{owner: a.owner, site: advisorySite{symbol: a.owner.symbol, representation: "category-update-site"}}
	a.roots(&receipts)
	for _, update := range a.updates {
		receipts.add("split-update-context", update, Detail{Subject: "complete numeric update; nested conditions remain in decision receipt"})
	}
	for _, key := range splitSortedKeys(a.effects.writes) {
		receipts.access(a.effects.writes[key], "split-scalar-write")
	}
	for _, key := range inputs {
		receipts.access(a.effects.reads[key], "split-shared-input-read")
	}
	return receipts.site
}
func (a categorySplitArm) roots(r *splitReceipts) {
	r.add("split-decision", a.decision.node, Detail{Subject: "complete decision; default and control transfers retained"})
	r.add("split-selector", a.decision.selection.selector, Detail{Subject: "resolved selector field"})
	r.add("split-category-arm", a.branch.node, Detail{Subject: categoryLabels(a.branch, a.owner.file.typeInfo())})
}
func (r *splitReceipts) add(kind string, node ast.Node, detail Detail) {
	r.site.receipts = append(r.site.receipts, r.owner.nodeSource(kind, node, detail))
}
func (r *splitReceipts) access(source Source, kind string) {
	source.Kind = kind
	source.Spelling = string(r.owner.file.data[source.StartOffset:source.EndOffset])
	r.site.receipts = append(r.site.receipts, source)
}
func (p categorySplitPair) context() advisorySite {
	receipts := splitReceipts{owner: p.scan.owner, site: advisorySite{symbol: p.scan.owner.symbol, representation: "unresolved-execution-context"}}
	p.intervening(&receipts)
	p.span(receipts.uncertainty)
	return receipts.site
}
func (p categorySplitPair) intervening(r *splitReceipts) {
	for _, stmt := range p.block.List {
		if stmt.Pos() >= p.left.node.End() && stmt.End() <= p.right.node.Pos() {
			r.add("split-intervening-statement", stmt, Detail{Subject: "intervening execution; effects not resolved"})
		}
	}
}
func (r *splitReceipts) uncertainty(node ast.Node) {
	if call, ok := node.(*ast.CallExpr); ok {
		r.add("split-call-or-conversion", call, Detail{Subject: "effects not resolved; purity not inferred"})
	}
	if unary, ok := node.(*ast.UnaryExpr); ok && unary.Op == token.AND {
		r.add("split-address-exposure", unary, Detail{Subject: "alias effects not resolved"})
	}
	r.indirectWrites(node)
}
func (r *splitReceipts) indirectWrites(node ast.Node) {
	mutation, _ := mutationOf(node)
	for _, target := range mutation.targets {
		if !categoryPath(r.owner.file.typeInfo(), target).valid() {
			r.add("split-indirect-write", target, Detail{Subject: "target identity not resolved"})
		}
	}
}
