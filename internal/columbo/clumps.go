package columbo

import "sort"

type typeMultiset map[string]int

func (m typeMultiset) types() []string {
	out := []string{}
	for t, n := range m {
		for i := 0; i < n; i++ {
			out = append(out, t)
		}
	}
	sort.Strings(out)
	return out
}
func (m typeMultiset) intersection(other typeMultiset) typeMultiset {
	out := typeMultiset{}
	for t, n := range m {
		if k := other[t]; k > 0 {
			out[t] = min(n, k)
		}
	}
	return out
}
func (m typeMultiset) contains(other typeMultiset) bool {
	for t, n := range other {
		if m[t] < n {
			return false
		}
	}
	return true
}
func (m typeMultiset) copy() typeMultiset {
	out := typeMultiset{}
	for t, n := range m {
		out[t] = n
	}
	return out
}
func (d *declaration) parameterTypes() typeMultiset {
	m := typeMultiset{}
	for _, p := range d.params {
		m[canonicalType(p.typ, d.signature)]++
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
	signatures   map[*declaration]typeMultiset
	patterns     []typeMultiset
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
	m := &clumpMiner{engine: a, declarations: ds, signatures: map[*declaration]typeMultiset{}, seen: map[string]bool{}}
	m.signaturesFromParameters()
	m.intersections()
	return m.report()
}
func (m *clumpMiner) signaturesFromParameters() {
	for _, d := range m.declarations {
		s := d.parameterTypes()
		m.signatures[d] = s
		m.add(s)
	}
}
func (m *clumpMiner) add(s typeMultiset) {
	ts := s.types()
	if int64(len(ts)) < m.engine.config.Counts["data-clump-size"] {
		return
	}
	key := canonical(ts)
	if m.seen[key] {
		return
	}
	m.seen[key] = true
	m.patterns = append(m.patterns, s)
}

// Closed frequent multisets are intersections of supporting signatures.
// Keep each intersection once rather than enumerate every possible subset.
func (m *clumpMiner) intersections() {
	for i := 0; i < len(m.patterns); i++ {
		m.intersectSignatures(m.patterns[i])
	}
}
func (m *clumpMiner) intersectSignatures(s typeMultiset) {
	for _, d := range m.declarations {
		m.add(s.intersection(m.signatures[d]))
	}
}
func (m *clumpMiner) support(s typeMultiset) []*declaration {
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
func (m *clumpMiner) reportPattern(s typeMultiset) error {
	support := m.support(s)
	if int64(len(support)) < m.engine.config.Counts["data-clump-occurrences"] {
		return nil
	}
	ts := s.types()
	c, e := m.engine.newCase(support[0], "data-clump", canonical(ts))
	if e != nil || c == nil {
		return e
	}
	m.clues(c, ts, len(support))
	for _, d := range support {
		d.clumpReceipts(c, s)
	}
	m.engine.report.Cases = append(m.engine.report.Cases, *c)
	return nil
}
func (m *clumpMiner) clues(c *Case, ts []string, count int) {
	c.Clues = append(c.Clues, metric("clump-types", c.Symbol, ts))
	c.Clues = append(c.Clues, metric("clump-size", c.Symbol, len(ts)).compare(m.engine.config.Counts["data-clump-size"], ">="))
	c.Clues = append(c.Clues, metric("clump-occurrences", c.Symbol, count).compare(m.engine.config.Counts["data-clump-occurrences"], ">="))
}
func (d *declaration) clumpReceipts(c *Case, s typeMultiset) {
	c.Receipts = append(c.Receipts, d.declReceipt())
	need := s.copy()
	for _, p := range d.params {
		d.clumpParameter(c, need, p)
	}
}
func (d *declaration) clumpParameter(c *Case, need typeMultiset, p parameter) {
	t := canonicalType(p.typ, d.signature)
	if need[t] == 0 {
		return
	}
	need[t]--
	c.Receipts = append(c.Receipts, d.parameterReceipt(p))
}
