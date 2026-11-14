package trials

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/KellyBennett/Columbo/calibration/harness"
	"github.com/KellyBennett/Columbo/calibration/operators"
)

// Plant is a killed inject mutant ready to hand to agents: Columbo noticed the planted smell, and the copy builds and passes its tests.
type Plant struct {
	Smell    string
	Operator string
	Corpus   *harness.Corpus
	Site     harness.Site
	Dose     int
	// Dir holds the mutant; every trial copies it.
	Dir    string
	Mutant harness.Snapshot
	// Added lists the cases the mutant added against the unmutated corpus, the ones the prompt shows.
	Added []harness.Finding
	Patch []byte
}

// Planter turns plan rows into plants, preparing each corpus once.
type Planter struct {
	Env     harness.Env
	PlanDir string
	Workers int
	Log     func(format string, args ...any)

	corpora map[string]*harness.Corpus
}

type plantJob struct {
	row     PlanRow
	corpus  *harness.Corpus
	op      harness.Operator
	site    harness.Site
	dose    int
	applied harness.Applied
}

// PlantPlan plants one smell at a time in plan order. With limit > 0 it stops once the schedule would hold limit trials,
// so a pilot probes only the smells it runs. Unplanted lists the smells whose rows killed no mutant.
func (pl *Planter) PlantPlan(rows []PlanRow, perSmell, limit int) (plants []*Plant, unplanted []string, err error) {
	var smells []string
	bySmell := map[string][]PlanRow{}
	for _, row := range rows {
		if _, seen := bySmell[row.Smell]; !seen {
			smells = append(smells, row.Smell)
		}
		bySmell[row.Smell] = append(bySmell[row.Smell], row)
	}
	for _, smell := range smells {
		if limit > 0 && len(Schedule(plants, perSmell)) >= limit {
			break
		}
		got, err := pl.Plant(bySmell[smell])
		plants = append(plants, got...)
		if err != nil {
			return plants, unplanted, err
		}
		if len(got) == 0 {
			unplanted = append(unplanted, smell)
		}
	}
	return plants, unplanted, nil
}

// Plant returns killed mutants in plan order, then site order, then dose order.
func (pl *Planter) Plant(rows []PlanRow) ([]*Plant, error) {
	var jobs []plantJob
	for _, row := range rows {
		c, err := pl.corpus(row.Corpus)
		if err != nil {
			return nil, err
		}
		ops, err := operators.Select([]string{row.Operator})
		if err != nil {
			return nil, err
		}
		op := ops[0]
		if op.Class() != harness.Inject || op.Target() != row.Smell {
			return nil, fmt.Errorf("%s is not an inject operator for %s", row.Operator, row.Smell)
		}
		for _, site := range op.Sites(c) {
			for _, dose := range op.Doses(c, site) {
				applied, err := op.Apply(c, site, dose)
				if err != nil {
					continue
				}
				jobs = append(jobs, plantJob{row, c, op, site, dose, applied})
			}
		}
	}
	plants := make([]*Plant, len(jobs))
	harness.ForEach(len(jobs), pl.Workers, func(i int) { plants[i] = pl.probe(jobs[i]) })
	return slices.DeleteFunc(plants, func(p *Plant) bool { return p == nil }), nil
}

func (pl *Planter) probe(j plantJob) *Plant {
	p := pl.Env.Probe(j.corpus.Root, j.applied.Edit)
	o := harness.Outcome{Class: harness.Inject, Target: j.op.Target()}
	if p.Invalid == "" {
		harness.Score(&o, j.applied.Expectation, j.corpus.Baseline.Findings, p.Snapshot.Findings)
	}
	if p.Invalid != "" || o.Status != harness.Killed {
		_ = os.RemoveAll(p.Dir)
		return nil
	}
	patch, _, err := harness.Measure(j.corpus.Root, j.applied.Edit)
	if err != nil {
		_ = os.RemoveAll(p.Dir)
		return nil
	}
	pl.logf("planted %s with %s on %s dose %d in %s", j.row.Smell, j.op.ID(), j.site.Symbol, j.dose, j.corpus.Module)
	return &Plant{Smell: j.row.Smell, Operator: j.op.ID(), Corpus: j.corpus, Site: j.site, Dose: j.dose,
		Dir: p.Dir, Mutant: p.Snapshot, Added: o.Added, Patch: patch}
}

func (pl *Planter) corpus(spec string) (*harness.Corpus, error) {
	if c, ok := pl.corpora[spec]; ok {
		return c, nil
	}
	src, err := Resolve(spec, pl.PlanDir)
	if err != nil {
		return nil, err
	}
	c, err := pl.Env.PrepareCorpus(src.Dir, nil, src.Module, src.Version)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", spec, err)
	}
	if pl.corpora == nil {
		pl.corpora = map[string]*harness.Corpus{}
	}
	pl.corpora[spec] = c
	pl.logf("prepared %s: %d baseline cases", c.Module, len(c.Baseline.Findings))
	return c, nil
}

func (pl *Planter) Cleanup(plants []*Plant) {
	for _, p := range plants {
		_ = os.RemoveAll(p.Dir)
	}
	for _, c := range pl.corpora {
		_ = os.RemoveAll(c.Root)
	}
}

func (pl *Planter) logf(format string, args ...any) {
	if pl.Log != nil {
		pl.Log(format, args...)
	}
}

func shortName(p *Plant) string {
	return fmt.Sprintf("%s-%s-%s-%s-d%d", p.Smell, p.Operator, filepath.Base(p.Corpus.Module), shortSymbol(p.Site.Symbol), p.Dose)
}
