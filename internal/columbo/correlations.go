package columbo

import (
	"cmp"
	"crypto/sha256"
	"fmt"
	"maps"
	"slices"
)

const missingRoleKind = "missing-polymorphic-role"

type Correlation struct {
	ID, Kind, Confidence, Domain, RoleID, Diagnosis string
	CaseIDs, Players, Messages, Leads, Avoid        []string
	Evidence                                        []CorrelationEvidence
	Policy                                          PolicyReview
}
type CorrelationEvidence struct{ CaseID, ReceiptKey string }
type correlationGroup struct {
	domain   string
	contexts []SelectionContext
}

func (r *Report) correlate() {
	r.Correlations = nil
	groups := groupSelections(r.Selections)
	for _, group := range groups.ordered() {
		if c := group.correlation(r); c != nil {
			r.Correlations = append(r.Correlations, *c)
		}
	}
}

type correlationGroups map[string]*correlationGroup

func (groups correlationGroups) ordered() []*correlationGroup {
	var result []*correlationGroup
	for _, key := range slices.Sorted(maps.Keys(groups)) {
		result = append(result, groups[key])
	}
	return result
}
func groupSelections(contexts []SelectionContext) correlationGroups {
	groups := correlationGroups{}
	for _, ctx := range contexts {
		key := ctx.groupKey()
		if groups[key] == nil {
			groups[key] = &correlationGroup{domain: ctx.Domain}
		}
		groups[key].contexts = append(groups[key].contexts, ctx)
	}
	return groups
}
func (ctx SelectionContext) groupKey() string {
	if ctx.Domain == "" {
		return canonical([]any{ctx.Surface, ctx.Players})
	}
	return canonical([]string{ctx.Domain, ctx.Surface})
}
func (g *correlationGroup) correlation(r *Report) *Correlation {
	c := newCorrelation(g.domain, g.contexts)
	role := g.commonRole(r.Roles, c.Players)
	repeated := g.repeatedCase(r.Cases)
	if role == nil && repeated == nil {
		return nil
	}
	c.join(role, repeated)
	c.diagnose(repeated != nil, role != nil, g.mappingState())
	c.finish()
	return c
}
func newCorrelation(domain string, contexts []SelectionContext) *Correlation {
	c := &Correlation{Kind: missingRoleKind, Confidence: "partial", Domain: domain, Policy: missingRolePolicy}
	for _, ctx := range contexts {
		c.includeSelection(ctx)
	}
	c.Players = sortedUnique(c.Players)
	return c
}
func (c *Correlation) includeSelection(ctx SelectionContext) {
	c.CaseIDs = append(c.CaseIDs, ctx.CaseID)
	c.Players = append(c.Players, ctx.Players...)
	c.Evidence = append(c.Evidence, CorrelationEvidence{ctx.CaseID, ctx.DecisionKey})
	for _, mapping := range ctx.Mappings {
		c.Evidence = append(c.Evidence, CorrelationEvidence{ctx.CaseID, mapping.ReceiptKey})
	}
}
func (c *Correlation) join(role *RoleCandidate, repeated *Case) {
	if role != nil {
		c.RoleID = role.ID
		c.Messages = slices.Clone(role.Messages)
	}
	if repeated != nil {
		c.includeRepeated(*repeated)
	}
}
func (c *Correlation) finish() {
	c.CaseIDs = sortedUnique(c.CaseIDs)
	c.Leads = missingRoleLeads(c.Confidence)
	c.Avoid = slices.Clone(missingRoleAvoid)
	c.identify()
	c.normalizeEvidence()
}
func (g *correlationGroup) commonRole(roles []RoleCandidate, players []string) *RoleCandidate {
	var chosen *RoleCandidate
	for i := range roles {
		role := &roles[i]
		if role.coversPlayers(players) && g.observes(role.Messages) && role.preferredTo(chosen) {
			chosen = role
		}
	}
	return chosen
}
func (r *RoleCandidate) coversPlayers(players []string) bool {
	return len(r.Messages) > 0 && containsStrings(r.Implementations, players)
}
func (r *RoleCandidate) preferredTo(other *RoleCandidate) bool {
	if other == nil {
		return true
	}
	return cmp.Or(cmp.Compare(len(r.Implementations), len(other.Implementations)), cmp.Compare(r.ID, other.ID)) < 0
}
func (g *correlationGroup) observes(messages []string) bool {
	for _, ctx := range g.contexts {
		if !containsStrings(ctx.Messages, messages) {
			return false
		}
	}
	return true
}
func containsStrings(haystack, needles []string) bool {
	for _, needle := range needles {
		if !slices.Contains(haystack, needle) {
			return false
		}
	}
	return true
}
func (g *correlationGroup) repeatedCase(cases []Case) *Case {
	if g.domain == "" {
		return nil
	}
	for i := range cases {
		if cases[i].variantDomain() == g.domain {
			return &cases[i]
		}
	}
	return nil
}
func (c Case) variantDomain() string {
	if c.Smell != variantSmell {
		return ""
	}
	for _, receipt := range c.Receipts {
		if source, ok := receipt.(Source); ok && source.Kind == "variant-decision" {
			return source.Detail.Subject
		}
	}
	return ""
}

type variantPlayerMapping map[string]string

func (m variantPlayerMapping) include(mappings []VariantPlayer) bool {
	for _, mapping := range mappings {
		if old, ok := m[mapping.Variant]; ok && old != mapping.Implementation {
			return false
		}
		m[mapping.Variant] = mapping.Implementation
	}
	return true
}
func (g *correlationGroup) mappingState() string {
	stable := variantPlayerMapping{}
	complete := true
	for _, ctx := range g.contexts {
		complete = complete && ctx.Complete && len(ctx.Mappings) > 0
		if !stable.include(ctx.Mappings) {
			return "conflicting"
		}
	}
	if !complete {
		return "incomplete"
	}
	return "stable"
}
func (c *Correlation) includeRepeated(member Case) {
	c.CaseIDs = append(c.CaseIDs, member.ID)
	for _, receipt := range member.Receipts {
		if source, ok := receipt.(Source); ok && (source.Kind == "variant-decision" || source.Kind == "variant-arm") {
			c.Evidence = append(c.Evidence, CorrelationEvidence{member.ID, sourceEvidenceKey(source)})
		}
	}
}
func (c *Correlation) identify() {
	digest := sha256.Sum256([]byte(canonical([]any{c.Kind, c.Domain, c.CaseIDs, c.RoleID})))
	c.ID = fmt.Sprintf("MPR-%x", digest)
}
func (c *Correlation) normalizeEvidence() {
	slices.SortFunc(c.Evidence, func(a, b CorrelationEvidence) int { return cmp.Compare(canonical(a), canonical(b)) })
	c.Evidence = slices.Compact(c.Evidence)
}
