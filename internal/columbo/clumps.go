package columbo

import "sort"

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
func (m parameterMultiset) copy() parameterMultiset {
	out := parameterMultiset{}
	for t, n := range m {
		out[t] = n
	}
	return out
}
func (d *declaration) clumpParameters() parameterMultiset {
	m := parameterMultiset{}
	for _, p := range d.params {
		if key, ok := d.clumpElement(p); ok {
			m[key]++
		}
	}
	return m
}
func (d *declaration) before(other *declaration) bool {
	if d.file.rel != other.file.rel {
		return d.file.rel < other.file.rel
	}
	return d.fn.Pos() < other.fn.Pos()
}

type clumpMiner struct {
	engine       *engine
	declarations []*declaration
	signatures   map[*declaration]parameterMultiset
	patterns     []parameterMultiset
	seen         map[string]bool
}

func (a *engine) packageDeclarations() map[string][]*declaration {
	out := map[string][]*declaration{}
	for _, d := range a.declarations {
		if d.file.included {
			out[d.file.pkg.PkgPath] = append(out[d.file.pkg.PkgPath], d)
		}
	}
	return out
}
func (a *engine) clumps() error {
	for _, ds := range a.packageDeclarations() {
		if e := a.mineClumps(ds); e != nil {
			return e
		}
	}
	return nil
}
func (a *engine) mineClumps(ds []*declaration) error {
	sort.Slice(ds, func(i, j int) bool { return ds[i].before(ds[j]) })
	m := &clumpMiner{engine: a, declarations: ds, signatures: map[*declaration]parameterMultiset{}, seen: map[string]bool{}}
	m.signaturesFromParameters()
	m.intersections()
	return m.report()
}
func (m *clumpMiner) signaturesFromParameters() {
	for _, d := range m.declarations {
		s := d.clumpParameters()
		m.signatures[d] = s
		m.add(s)
	}
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
	for _, d := range m.declarations {
		m.add(s.intersection(m.signatures[d]))
	}
}
func (m *clumpMiner) support(s parameterMultiset) []*declaration {
	out := []*declaration{}
	for _, d := range m.declarations {
		if m.signatures[d].contains(s) {
			out = append(out, d)
		}
	}
	return out
}
func (m *clumpMiner) report() error {
	for _, s := range m.patterns {
		if e := m.reportPattern(s); e != nil {
			return e
		}
	}
	return nil
}
func (m *clumpMiner) reportPattern(s parameterMultiset) error {
	support := m.support(s)
	if int64(len(support)) < m.engine.config.Counts["data-clump-occurrences"] {
		return nil
	}
	c, err := m.patternCase(s, support)
	if err != nil || c == nil {
		return err
	}
	m.engine.report.Cases = append(m.engine.report.Cases, *c)
	return nil
}
func (m *clumpMiner) patternCase(pattern parameterMultiset, support []*declaration) (*Case, error) {
	types := pattern.elements()
	c, err := m.engine.newCase(support[0], "data-clump", canonical(types))
	if err != nil || c == nil {
		return c, err
	}
	evidence := c.clumpClues(types, len(support), m.engine.config)
	evidence.include(support, pattern)
	return c, nil
}

type clumpEvidence struct {
	finding                  *Case
	types, size, occurrences *clueSupport
}

func (c *Case) clumpClues(types []string, count int, config Config) *clumpEvidence {
	typeClue := metric("clump-types", c.Symbol, types).forDeclaration(c.PrimaryDeclaration)
	sizeClue := metric("clump-size", c.Symbol, len(types)).compare(config.Counts["data-clump-size"], ">=").forDeclaration(c.PrimaryDeclaration)
	occurrenceClue := metric("clump-occurrences", c.Symbol, count).compare(config.Counts["data-clump-occurrences"], ">=").forDeclaration(c.PrimaryDeclaration)
	return &clumpEvidence{finding: c, types: newClueSupport(&typeClue), size: newClueSupport(&sizeClue), occurrences: newClueSupport(&occurrenceClue)}
}
func (e *clumpEvidence) include(support []*declaration, pattern parameterMultiset) {
	for _, d := range support {
		e.declarationReceipts(d, pattern)
	}
	e.finding.Clues = append(e.finding.Clues, *e.types.clue, *e.size.clue, *e.occurrences.clue)
}
func (e *clumpEvidence) declarationReceipts(d *declaration, s parameterMultiset) {
	e.finding.includeDeclaration(d, "clump-support")
	e.occurrences.add(d.declReceipt())
	need := s.copy()
	for _, p := range d.params {
		e.parameter(d, need, p)
	}
}
func (e *clumpEvidence) parameter(d *declaration, need parameterMultiset, p parameter) {
	key, ok := d.clumpElement(p)
	if !ok || need[key] == 0 {
		return
	}
	need[key]--
	e.parameterReceipt(d.parameterReceipt(p))
}

func (d *declaration) clumpElement(p parameter) (string, bool) {
	name, t := p.obj.Name(), canonicalType(p.typ, d.signature)
	if name == "" || name == "_" || t == "context.Context" {
		return "", false
	}
	return name + " " + t, true
}
func (e *clumpEvidence) parameterReceipt(receipt Source) {
	e.finding.Receipts = append(e.finding.Receipts, receipt)
	e.types.add(receipt)
	e.size.add(receipt)
}
