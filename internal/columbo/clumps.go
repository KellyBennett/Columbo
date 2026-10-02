package columbo

import (
	"sort"
)

type clump struct {
	types   []string
	support []*declaration
}

func (a *engine) clumps() error {
	packages := map[string][]*declaration{}
	for _, d := range a.declarations {
		if d.file.included {
			packages[d.file.pkg.PkgPath] = append(packages[d.file.pkg.PkgPath], d)
		}
	}
	for _, ds := range packages {
		sets := map[*declaration]map[string]int{}
		all := map[string]bool{}
		for _, d := range ds {
			m := map[string]int{}
			for _, p := range d.params {
				s := canonicalType(p.typ, d.signature)
				m[s]++
				all[s] = true
			}
			sets[d] = m
		}
		universe := sortedSet(all)
		freq := []clump{}
		var search func(int, []string, []*declaration)
		search = func(start int, key []string, support []*declaration) {
			for i := start; i < len(universe); i++ {
				typ := universe[i]
				count := 1
				for _, t := range key {
					if t == typ {
						count++
					}
				}
				next := []*declaration{}
				for _, d := range support {
					if sets[d][typ] >= count {
						next = append(next, d)
					}
				}
				if int64(len(next)) < a.config.Counts["data-clump-occurrences"] {
					continue
				}
				k := append(append([]string{}, key...), typ)
				if int64(len(k)) >= a.config.Counts["data-clump-size"] {
					freq = append(freq, clump{k, next})
				}
				search(i, k, next)
			}
		}
		search(0, []string{}, ds)
		for _, f := range freq {
			closed := true
			for _, g := range freq {
				if len(g.types) <= len(f.types) || len(g.support) != len(f.support) {
					continue
				}
				same := true
				for i := range f.support {
					if f.support[i] != g.support[i] {
						same = false
						break
					}
				}
				if !same {
					continue
				}
				m := map[string]int{}
				for _, s := range g.types {
					m[s]++
				}
				contains := true
				for _, s := range f.types {
					m[s]--
					if m[s] < 0 {
						contains = false
					}
				}
				if contains {
					closed = false
					break
				}
			}
			if !closed {
				continue
			}
			sort.Slice(f.support, func(i, j int) bool {
				d, e := f.support[i], f.support[j]
				if d.file.rel != e.file.rel {
					return d.file.rel < e.file.rel
				}
				return d.fn.Pos() < e.fn.Pos()
			})
			d := f.support[0]
			c, e := a.newCase(d, "data-clump", canonical(f.types))
			if e != nil {
				return e
			}
			if c == nil {
				continue
			}
			c.Clues = append(c.Clues, metric("clump-types", d.symbol, f.types, nil, nil), metric("clump-size", d.symbol, len(f.types), a.config.Counts["data-clump-size"], ">="), metric("clump-occurrences", d.symbol, len(f.support), a.config.Counts["data-clump-occurrences"], ">="))
			for _, s := range f.support {
				c.Receipts = append(c.Receipts, s.declReceipt())
				need := map[string]int{}
				for _, t := range f.types {
					need[t]++
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
