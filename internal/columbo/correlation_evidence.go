package columbo

import (
	"go/ast"
	"maps"
	"slices"
)

type SelectionContext struct {
	CaseID, Domain, Surface string
	Players, Messages       []string
	Mappings                []VariantPlayer
	Complete                bool
	DecisionKey             string
}
type VariantPlayer struct {
	Variant, Implementation, ReceiptKey string
}

type selectionCorrelation struct {
	site    *selectionSite
	finding *Case
	context SelectionContext
	parsed  variantSite
}

func (site *selectionSite) correlationContext(c *Case) SelectionContext {
	builder := selectionCorrelation{site: site, finding: c}
	builder.initialize()
	builder.mapOrigins()
	return builder.context
}
func (b *selectionCorrelation) initialize() {
	site := b.site
	b.context = SelectionContext{CaseID: b.finding.ID, Surface: site.role, Complete: true}
	b.context.DecisionKey = sourceEvidenceKey(site.receipt("selection-decision", site.decision, site.role))
	b.context.Players = site.correlationPlayers()
	b.context.useRole(site.roleGroup().candidate())
	b.parsed = site.variantDecision()
}
func (site *selectionSite) correlationPlayers() []string {
	var players []string
	for origin := range site.origins {
		players = append(players, roleImplementation(site.owner.file.typeInfo().TypeOf(origin)))
	}
	return sortedUnique(players)
}
func (c *SelectionContext) useRole(role *RoleCandidate) {
	if role == nil {
		return
	}
	c.Messages = role.Messages
	c.Surface = canonical(role.Messages)
}
func (site *selectionSite) variantDecision() variantSite {
	info := site.owner.file.typeInfo()
	return variantSite{owner: site.owner, info: info, facts: variantTypeFacts{info}, arms: map[string][]Source{}}
}
func (b *selectionCorrelation) mapOrigins() {
	if !b.parsed.interpret(b.site.decision, nil) {
		b.context.Complete = false
		return
	}
	b.context.Domain = b.parsed.domain
	for origin := range b.site.origins {
		b.mapOrigin(origin)
	}
}
func (b *selectionCorrelation) mapOrigin(origin ast.Expr) {
	variants := b.originVariants(origin)
	if len(variants) == 0 {
		b.context.Complete = false
	}
	for _, variant := range variants {
		receipt := b.site.mappingReceipt(origin, variant)
		b.finding.Receipts = append(b.finding.Receipts, receipt)
		b.context.Mappings = append(b.context.Mappings, VariantPlayer{variant, receipt.Spelling, sourceEvidenceKey(receipt)})
	}
}
func (b *selectionCorrelation) originVariants(origin ast.Expr) []string {
	for _, arm := range selectionRoleArms(b.site.decision) {
		if origin.Pos() >= arm.body.Pos() && origin.End() <= arm.body.End() {
			return b.armVariants(arm)
		}
	}
	return nil
}
func (b *selectionCorrelation) armVariants(arm roleArm) []string {
	var result []string
	for key, receipts := range b.parsed.arms {
		for _, receipt := range receipts {
			if armContainsVariant(arm, receipt, b.site.owner) {
				result = append(result, key)
			}
		}
	}
	return sortedUnique(result)
}
func armContainsVariant(arm roleArm, receipt Source, owner *declaration) bool {
	for _, expr := range arm.labels {
		if owner.file.tf.Offset(expr.Pos()) <= receipt.StartOffset && owner.file.tf.Offset(expr.End()) >= receipt.EndOffset {
			return true
		}
	}
	return false
}
func sortedUnique(values []string) []string {
	set := map[string]bool{}
	for _, value := range values {
		set[value] = true
	}
	return slices.Sorted(maps.Keys(set))
}

func (site *selectionSite) mappingReceipt(origin ast.Expr, variant string) Source {
	receipt := site.receipt("selection-variant-mapping", origin, variant)
	receipt.Spelling = roleImplementation(site.owner.file.typeInfo().TypeOf(origin))
	return receipt
}
