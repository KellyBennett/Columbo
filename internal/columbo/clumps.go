package columbo

import (
	"slices"
	"sort"
)

const primitiveClumpLead = "Every member of this clump has a primitive type. Consider a distinct named type for each kind of value, such as one ID type per identifier; the type checker would then see the clump and reject values passed in each other's place."

type parameterMultiset map[string]int

func (m parameterMultiset) elements() []string {
	out := []string{}
	for t, n := range m {
		for i := 0; i < n; i++ {
			out = append(out, t)
		}
	}
	sort.Strings(out)
	return out
}
func (m parameterMultiset) intersection(other parameterMultiset) parameterMultiset {
	out := parameterMultiset{}
	for t, n := range m {
		if k := other[t]; k > 0 {
			out[t] = min(n, k)
		}
	}
	return out
}
func (m parameterMultiset) contains(other parameterMultiset) bool {
	for t, n := range other {
		if m[t] < n {
			return false
		}
	}
	return true
}
func (m parameterMultiset) size() int {
	total := 0
	for _, n := range m {
		total += n
	}
	return total
}
func (m parameterMultiset) copy() parameterMultiset {
	out := parameterMultiset{}
	for t, n := range m {
		out[t] = n
	}
	return out
}

type clumpMiner struct {
	engine   *engine
	sites    []clumpSite
	patterns []parameterMultiset
	seen     map[string]bool
}

func (a *engine) clumps() error {
	for _, sites := range a.packageClumpSites() {
		if e := a.mineClumps(sites); e != nil {
			return e
		}
	}
	return nil
}
func (a *engine) mineClumps(sites []clumpSite) error {
	m := &clumpMiner{engine: a, sites: sites, seen: map[string]bool{}}
	for _, site := range sites {
		m.add(site.signature)
	}
	m.intersections()
	return m.report()
}
func (m *clumpMiner) add(s parameterMultiset) {
	keys := s.elements()
	if int64(len(keys)) < m.engine.config.Counts["data-clump-size"] {
		return
	}
	key := canonical(keys)
	if m.seen[key] {
		return
	}
	m.seen[key] = true
	m.patterns = append(m.patterns, s)
}

func (m *clumpMiner) intersections() {
	for i := 0; i < len(m.patterns); i++ {
		m.intersectSignatures(m.patterns[i])
	}
}
func (m *clumpMiner) intersectSignatures(s parameterMultiset) {
	for _, site := range m.sites {
		m.add(s.intersection(site.signature))
	}
}
func (m *clumpMiner) support(s parameterMultiset) []clumpSite {
	out := []clumpSite{}
	for _, site := range m.sites {
		if site.signature.contains(s) {
			out = append(out, site)
		}
	}
	return out
}

type clumpPattern struct {
	members parameterMultiset
	support []clumpSite
}

func (m *clumpMiner) qualifying() []clumpPattern {
	out := []clumpPattern{}
	for _, s := range m.patterns {
		if support := m.support(s); int64(len(support)) >= m.engine.config.Counts["data-clump-occurrences"] {
			out = append(out, clumpPattern{members: s, support: support})
		}
	}
	return out
}
func (p clumpPattern) minimal(qualifying []clumpPattern) bool {
	for _, other := range qualifying {
		if other.members.size() < p.members.size() && p.members.contains(other.members) {
			return false
		}
	}
	return true
}
func (m *clumpMiner) report() error {
	qualifying := m.qualifying()
	for _, p := range qualifying {
		if !p.minimal(qualifying) {
			continue
		}
		c, err := m.patternCase(p.members, p.support)
		if err != nil {
			return err
		}
		if c != nil {
			m.engine.report.Cases = append(m.engine.report.Cases, *c)
		}
	}
	return nil
}
func (m *clumpMiner) patternCase(pattern parameterMultiset, support []clumpSite) (*Case, error) {
	parameters := pattern.elements()
	c, err := m.engine.identifiedCase(m.engine.sourceCase("data-clump", support[0].receipt), canonical(parameters))
	if err != nil || c == nil {
		return c, err
	}
	evidence := c.clumpClues(parameters, len(support), m.engine.config)
	evidence.include(support, pattern)
	if allPrimitive(support[0].matched(pattern)) {
		c.Leads = append(c.Leads, primitiveClumpLead)
	}
	return c, nil
}

type clumpEvidence struct {
	finding                       *Case
	parameters, size, occurrences *clueSupport
}

func (c *Case) clumpClues(parameters []string, count int, config Config) *clumpEvidence {
	parameterClue := metric("clump-parameters", c.Symbol, parameters).forDeclaration(c.PrimaryDeclaration)
	sizeClue := metric("clump-size", c.Symbol, len(parameters)).compare(config.Counts["data-clump-size"], ">=").forDeclaration(c.PrimaryDeclaration)
	occurrenceClue := metric("clump-occurrences", c.Symbol, count).compare(config.Counts["data-clump-occurrences"], ">=").forDeclaration(c.PrimaryDeclaration)
	return &clumpEvidence{finding: c, parameters: newClueSupport(&parameterClue), size: newClueSupport(&sizeClue), occurrences: newClueSupport(&occurrenceClue)}
}
func (e *clumpEvidence) include(support []clumpSite, pattern parameterMultiset) {
	for _, site := range support {
		e.siteReceipts(site, pattern)
	}
	e.finding.Clues = append(e.finding.Clues, *e.parameters.clue, *e.size.clue, *e.occurrences.clue)
}
func (e *clumpEvidence) siteReceipts(site clumpSite, pattern parameterMultiset) {
	e.finding.includeReceipt(site.receipt, "clump-support")
	e.occurrences.add(site.receipt)
	for _, member := range site.matched(pattern) {
		e.memberReceipt(member.receipt)
	}
}
func allPrimitive(members []clumpMember) bool {
	return !slices.ContainsFunc(members, func(m clumpMember) bool { return !m.primitive })
}
func (e *clumpEvidence) memberReceipt(receipt Source) {
	e.finding.Receipts = append(e.finding.Receipts, receipt)
	e.parameters.add(receipt)
	e.size.add(receipt)
}
