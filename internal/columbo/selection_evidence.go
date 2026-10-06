package columbo

import (
	"fmt"
	"go/ast"
	"go/token"
	"maps"
	"slices"
)

func (a *engine) selectionUses() error {
	if a.config.Severity[selectionSmell] == "off" {
		return nil
	}
	for _, d := range a.declarations {
		if !selectionBoundary(d) {
			if err := a.selectionDeclaration(d); err != nil {
				return err
			}
		}
	}
	return nil
}
func selectionBoundary(d *declaration) bool {
	if !d.file.included || !d.hasBody() {
		return true
	}
	if d.isRuntimeBoundary() {
		return true
	}
	return hasSelectionBranch(d.fn.Body, token.GOTO)
}
func (a *engine) selectionDeclaration(d *declaration) error {
	scan := newSelectionScan(d, int(a.config.Counts["selection-use-implementations"]))
	scan.run()
	groups := map[string][]*selectionSite{}
	for _, site := range scan.qualifyingSites() {
		groups[site.role] = append(groups[site.role], site)
	}
	for _, role := range slices.Sorted(maps.Keys(groups)) {
		if err := a.reportSelection(d, role, coalesceSelectionSites(groups[role])); err != nil {
			return err
		}
	}
	return nil
}
func coalesceSelectionSites(sites []*selectionSite) []*selectionSite {
	byDecision := map[ast.Node]*selectionSite{}
	var result []*selectionSite
	for _, site := range sites {
		if old := byDecision[site.decision]; old != nil {
			old.include(site)
			continue
		}
		byDecision[site.decision] = site
		result = append(result, site)
	}
	return result
}
func (site *selectionSite) include(other *selectionSite) {
	maps.Copy(site.origins, other.origins)
	maps.Copy(site.messages, other.messages)
	maps.Copy(site.flows, other.flows)
}
func (a *engine) reportSelection(d *declaration, role string, sites []*selectionSite) error {
	id, raw := identity(selectionSmell, "", d.symbol, role)
	if err := a.claimIdentity(id, raw); err != nil {
		return err
	}
	c := caseFromSource(selectionSmell, a.config.Severity[selectionSmell], d.declReceipt())
	c.ID = id
	selectionEvidence(c, sites, a.config.Counts["selection-use-implementations"])
	a.report.Cases = append(a.report.Cases, *c)
	return nil
}
func selectionEvidence(c *Case, sites []*selectionSite, minimum int64) {
	var decisions []Source
	for _, site := range sites {
		decisions = append(decisions, site.receipt("selection-decision", site.decision, site.role))
		site.evidence(c, minimum)
	}
	c.Clues = append(c.Clues, metric("selection-use-sites", c.Symbol+":"+sites[0].role, len(sites)).forDeclaration(c.PrimaryDeclaration).supportedBy(decisions),
		c.metric("selected-role", []string{sites[0].role}).supportedBy(decisions))
}
func (site *selectionSite) receipt(kind string, node ast.Node, subject string) Source {
	return site.owner.source(kind, node.Pos(), node.End(), Detail{Subject: subject})
}
func (site *selectionSite) originsEvidence() []Source {
	var sources []Source
	for expr, name := range site.origins {
		sources = append(sources, site.receipt("selection-origin", expr, name))
	}
	return sources
}
func (site *selectionSite) messagesEvidence() []Source {
	var sources []Source
	for call, name := range site.messages {
		sources = append(sources, site.receipt("selected-message", call, name))
	}
	return sources
}
func (site *selectionSite) flowsEvidence() []Source {
	var sources []Source
	for node := range site.flows {
		sources = append(sources, site.receipt("selection-flow", node, site.role))
	}
	return sources
}
func (site *selectionSite) evidence(c *Case, minimum int64) {
	decision := site.receipt("selection-decision", site.decision, site.role)
	origins, messages, flows := site.originsEvidence(), site.messagesEvidence(), site.flowsEvidence()
	support := append([]Source{decision}, origins...)
	support = append(support, flows...)
	support = append(support, messages...)
	appendSources(c, support)
	site.clues(c, decision, support, minimum)
}
func (site *selectionSite) clues(c *Case, decision Source, support []Source, minimum int64) {
	subject := fmt.Sprintf("%s:%d", decision.File, decision.StartOffset)
	implementations := site.implementations()
	c.Clues = append(c.Clues,
		metric("selected-implementation-count", subject, len(implementations)).forDeclaration(c.PrimaryDeclaration).compare(minimum, ">=").supportedBy(support),
		metric("selected-implementation-set", subject, implementations).forDeclaration(c.PrimaryDeclaration).supportedBy(site.originsEvidence()),
		metric("selected-message-set", subject, site.messageSet()).forDeclaration(c.PrimaryDeclaration).supportedBy(append(site.flowsEvidence(), site.messagesEvidence()...)),
		metric("selected-use-count", subject, site.useCount()).forDeclaration(c.PrimaryDeclaration).supportedBy(site.messagesEvidence()))
}
func (site *selectionSite) messageSet() []string {
	set := map[string]bool{}
	for _, message := range site.messages {
		set[message] = true
	}
	return slices.Sorted(maps.Keys(set))
}

func (d *declaration) isRuntimeBoundary() bool {
	if d.hasReceiver() {
		return false
	}
	return d.obj.Name() == "init" || d.obj.Name() == "main" && d.obj.Pkg().Name() == "main"
}
func (s *selectionScan) run() { s.block(s.owner.fn.Body.List, selectionEnv{}) }
func (site *selectionSite) implementations() []string {
	return (selectedValue{origins: site.origins}).implementations()
}
func (site *selectionSite) useCount() int { return len(site.messages) }
