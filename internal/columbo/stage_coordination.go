package columbo

func (a *engine) coordinationAdvisories() []advisoryGroup {
	index := variantIndex{engine: a, domains: map[string][]variantSite{}}
	index.collect()
	return index.coordinationAdvisories()
}
func (index *variantIndex) coordinationAdvisories() []advisoryGroup {
	groups := []advisoryGroup{}
	for _, domain := range sortedVariantDomains(index.domains) {
		if index.eligibleDomain(domain) {
			groups = append(groups, index.engine.coordinationDomain(domain)...)
		}
	}
	return groups
}
func (index *variantIndex) eligibleDomain(domain string) bool {
	config := index.engine.config
	group := repeatedVariantGroup(index.domains[domain], int(config.Counts["repeated-variant-sites"]))
	return len(group.repeated) >= int(config.Counts["repeated-variant-variants"])
}
func (a *engine) coordinationDomain(domain string) []advisoryGroup {
	evidence := Case{}
	a.variantCoordination(&evidence, domain)
	groups := []advisoryGroup{}
	for _, clue := range evidence.Clues {
		groups = append(groups, clue.coordinationGroup(evidence))
	}
	return groups
}
func (clue Clue) coordinationGroup(evidence Case) advisoryGroup {
	id, _ := identity("variant-coordination", "", clue.Kind, clue.Subject)
	group := advisoryGroup{id: "V" + id, kind: "variant-coordination", subject: clue.Subject, lead: untangleTask, limits: coordinationLimits}
	group.values = clue.coordinationValues()
	site := advisorySite{symbol: clue.Declaration.Symbol}
	for _, key := range clue.SupportingReceipts {
		site.receipts = append(site.receipts, coordinationReceipt(evidence, key))
	}
	group.sites = []advisorySite{site}
	return group
}

const coordinationLimits = "Lexical coordination evidence, not proof of feasible paths or design quality. Aliases, intervening writes and call effects are not resolved."

func (clue Clue) coordinationValues() []advisoryValue {
	values := []advisoryValue{{"coordination-kind", "", clue.Kind}}
	for _, value := range clue.Value.([]string) {
		values = append(values, advisoryValue{"coordination-detail", "", value})
	}
	return values
}
func coordinationReceipt(evidence Case, key string) Source {
	for _, item := range evidence.Receipts {
		if source, ok := item.(Source); ok && sourceEvidenceKey(source) == key {
			return source
		}
	}
	return Source{}
}
