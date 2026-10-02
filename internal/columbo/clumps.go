package columbo

import "sort"

// Closed frequent multisets are intersections of supporting signatures. Mining
// distinct intersections avoids enumerating every subset of a large signature.
func (a *engine) clumps() error {
	packages := map[string][]*declaration{}
	for _, d := range a.declarations {
		if d.file.included {
			packages[d.file.pkg.PkgPath] = append(packages[d.file.pkg.PkgPath], d)
		}
	}
	for _, ds := range packages {
		sort.Slice(ds, func(i, j int) bool {
			if ds[i].file.rel != ds[j].file.rel {
				return ds[i].file.rel < ds[j].file.rel
			}
			return ds[i].fn.Pos() < ds[j].fn.Pos()
		})
		sets := map[*declaration]map[string]int{}
		patterns := []map[string]int{}
		seen := map[string]bool{}
		serialize := func(m map[string]int) []string {
			out := []string{}
			for t, n := range m {
				for i := 0; i < n; i++ {
					out = append(out, t)
				}
			}
			sort.Strings(out)
			return out
		}
		add := func(m map[string]int) {
			types := serialize(m)
			if int64(len(types)) < a.config.Counts["data-clump-size"] {
				return
			}
			key := canonical(types)
			if !seen[key] {
				seen[key] = true
				patterns = append(patterns, m)
			}
		}
		for _, d := range ds {
			m := map[string]int{}
			for _, p := range d.params {
				m[canonicalType(p.typ, d.signature)]++
			}
			sets[d] = m
			add(m)
		}
		for i := 0; i < len(patterns); i++ {
			for _, d := range ds {
				m := map[string]int{}
				for t, n := range patterns[i] {
					if k := sets[d][t]; k > 0 {
						m[t] = min(n, k)
					}
				}
				add(m)
			}
		}
		for _, m := range patterns {
			support := []*declaration{}
			for _, d := range ds {
				contains := true
				for t, n := range m {
					if sets[d][t] < n {
						contains = false
						break
					}
				}
				if contains {
					support = append(support, d)
				}
			}
			if int64(len(support)) < a.config.Counts["data-clump-occurrences"] {
				continue
			}
			ts := serialize(m)
			d := support[0]
			c, e := a.newCase(d, "data-clump", canonical(ts))
			if e != nil {
				return e
			}
			if c == nil {
				continue
			}
			c.Clues = append(c.Clues, metric("clump-types", d.symbol, ts, nil, nil), metric("clump-size", d.symbol, len(ts), a.config.Counts["data-clump-size"], ">="), metric("clump-occurrences", d.symbol, len(support), a.config.Counts["data-clump-occurrences"], ">="))
			for _, s := range support {
				c.Receipts = append(c.Receipts, s.declReceipt())
				need := map[string]int{}
				for t, n := range m {
					need[t] = n
				}
				for _, p := range s.params {
					t := canonicalType(p.typ, s.signature)
					if need[t] > 0 {
						need[t]--
						c.Receipts = append(c.Receipts, s.parameterReceipt(p))
					}
				}
			}
			a.report.Cases = append(a.report.Cases, *c)
		}
	}
	return nil
}
