package columbo

import (
	"fmt"
	"go/ast"
	"sort"
)

func (s *coordinationScan) evidence(c *Case) {
	for i, left := range s.decisions {
		for _, right := range s.decisions[i+1:] {
			if left.independent(right) {
				s.pairEvidence(c, left, right)
			}
		}
	}
}
func (d coordinationDecision) independent(other coordinationDecision) bool {
	return d.selector.same(other.selector) && d.node.End() <= other.node.Pos()
}
func (d coordinationDecision) writeKeys() []string {
	keys := []string{}
	for key := range d.writes {
		if key != coordinationKey(d.selector) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}
func (s *coordinationScan) pairEvidence(c *Case, left, right coordinationDecision) {
	p := coordinationPair{scan: s, left: left, right: right}
	for _, key := range left.writeKeys() {
		if target, ok := right.writes[key]; ok {
			p.link(c, coordinationLink{"variant-shared-write", key, target})
		}
		if target, ok := right.reads[key]; ok {
			p.link(c, coordinationLink{"variant-state-overlap", key, target})
		}
	}
}

type coordinationPair struct {
	scan        *coordinationScan
	left, right coordinationDecision
}

type coordinationLink struct {
	kind, key string
	to        Source
}

func (p coordinationPair) root(d coordinationDecision) Source {
	return p.scan.owner.source("variant-coordination-root", d.node.Pos(), d.node.End(), Detail{Subject: p.scan.domain})
}
func (p coordinationPair) link(c *Case, link coordinationLink) {
	first, second := p.root(p.left), p.root(p.right)
	from := p.left.writes[link.key]
	receipts := []Source{first, second, from, link.to}
	values := p.description(link.key, from.Detail.Subject)
	if link.kind == "variant-state-overlap" {
		values = append(values, "access types: earlier write; later read", "These decisions inspect the same variant selector and access the same state. Review whether they distribute behavior that belongs together. This does not establish that a value written by one decision reaches the other.")
	}
	subject := coordinationSubject(first, second, from.Detail.Subject)
	appendSources(c, receipts)
	c.Clues = append(c.Clues, metric(link.kind, subject, values).forDeclaration(first.Declaration).supportedBy(receipts))
}

func coordinationSubject(first, second Source, location string) string {
	return fmt.Sprintf("%s:%d:%d:%s", first.File, first.StartOffset, second.StartOffset, location)
}
func (p coordinationPair) description(key, location string) []string {
	return []string{
		fmt.Sprintf("selector %s; roots %d -> %d", p.left.selector.key(p.left.selector.root.Name()), p.root(p.left).StartLine, p.root(p.right).StartLine),
		"location " + location,
		fmt.Sprintf("comparisons %v -> %v; else/default may include undeclared values", p.left.partitions, p.right.partitions),
		"lexical evidence only: paths, aliases, intervening writes and call effects are not resolved",
		p.uncertainty(key),
	}
}
func (p coordinationPair) uncertainty(key string) string {
	u := coordinationUncertainty{info: p.scan.info, left: p.left, right: p.right, location: key}
	ast.Inspect(p.scan.owner.fn.Body, u.visit)
	return u.text()
}
