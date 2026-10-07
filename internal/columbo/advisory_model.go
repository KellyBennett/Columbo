package columbo

type advisoryValue struct{ kind, identity, value string }
type advisorySite struct {
	symbol, representation, input string
	seeds                         []ChoiceConstant
	receipts                      []Source
}
type advisoryGroup struct {
	id, kind, subject, lead, limits string
	values                          []advisoryValue
	sites                           []advisorySite
}

func (report Report) advisories() []advisoryGroup {
	groups := append([]advisoryGroup{}, report.Coordination...)
	groups = append(groups, report.CategoryBehavior...)
	for _, group := range report.Choices.Groups {
		groups = append(groups, group.advisory())
	}
	for _, group := range report.Tangles.Groups {
		groups = append(groups, group.advisory())
	}
	return groups
}
func (g ChoiceSetGroup) advisory() advisoryGroup {
	group := advisoryGroup{id: g.ID, kind: "choice-set", subject: g.Collection, lead: g.Lead, limits: g.Limits}
	group.values = append(group.values, advisoryValue{"projection-field", g.Projection, ""})
	for _, value := range g.Constants {
		group.values = append(group.values, advisoryValue{"constant", value.Identity, value.Value})
	}
	for _, site := range g.Sites {
		group.sites = append(group.sites, advisorySite{site.Symbol, site.Representation, site.Input, site.Seeds, site.Receipts})
	}
	return group
}
func (g TangleGroup) advisory() advisoryGroup {
	group := advisoryGroup{id: g.ID, kind: "nested-field-decision", subject: g.Field, lead: g.Lead, limits: g.Limits}
	for _, value := range g.Values {
		group.values = append(group.values, advisoryValue{"repeated-value", "", value})
	}
	for _, field := range g.WrittenFields {
		group.values = append(group.values, advisoryValue{"written-field", field, ""})
	}
	for _, site := range g.Sites {
		group.sites = append(group.sites, site.advisory(g.Symbol))
	}
	return group
}
func (s TangleSite) advisory(symbol string) advisorySite {
	receipts := append([]Source{s.Condition}, s.Comparisons...)
	receipts = append(receipts, s.Writes...)
	return advisorySite{symbol: symbol, receipts: receipts}
}
func (selection evidenceSelection) addAdvisories(report Report) {
	for _, group := range report.advisories() {
		for _, site := range group.sites {
			selection[site.declaration()] = true
		}
	}
}
func (site advisorySite) declaration() DeclarationRef {
	if len(site.receipts) == 0 {
		return DeclarationRef{}
	}
	return DeclarationRef{File: site.receipts[0].File, Symbol: site.symbol}
}
