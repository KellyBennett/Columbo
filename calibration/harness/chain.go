package harness

import (
	"fmt"
	"os"
	"slices"
	"strings"
)

// ChainStep names an edit by what survives earlier edits: offsets move, symbols and doses do not.
type ChainStep struct {
	Operator string `json:"operator"`
	Symbol   string `json:"symbol"`
	Dose     int    `json:"dose"`
}

type Invariant string

const (
	NeutralKeepsIDs Invariant = "neutral-chain-keeps-case-ids"
	InjectNoticed   Invariant = "inject-chain-is-noticed"
)

type Verdict string

const (
	Holds    Verdict = "holds"
	Violated Verdict = "violated"
	// Vacuous: no invariant covers the chain, such as evades with no inject predicted to fire.
	Vacuous Verdict = "vacuous"
	// Inapplicable: a step could not be applied or the chained code no longer builds.
	Inapplicable Verdict = "inapplicable"
)

type ChainOutcome struct {
	Steps     []ChainStep `json:"steps"`
	Invariant Invariant   `json:"invariant,omitempty"`
	Verdict   Verdict     `json:"verdict"`
	Reason    string      `json:"reason"`
	// Choices replays the chain through a BytePicker; it seeds the fuzzer.
	Choices []byte `json:"-"`
}

// Pick is one chosen edit plus the indices that select it, so any walk can be re-encoded as fuzz input.
type Pick struct {
	Op      Operator
	Site    Site
	Dose    int
	Choices []byte
}

// Picker is the Strategy that drives a Walk: fuzz bytes or a recorded chain choose each next edit.
type Picker interface {
	// Next returns false when the chain is complete; an error means a recorded step no longer applies.
	Next(ops []Operator, c *Corpus) (Pick, bool, error)
}

type Chainer struct {
	Env    Env
	Source string
	Ops    []Operator

	base   string
	corpus *Corpus
}

func (ch *Chainer) Prepare() error {
	base, err := TempDir("calibration-chain-base-")
	if err != nil {
		return err
	}
	ch.base = base
	if err := CopyModule(ch.Source, base); err != nil {
		return err
	}
	if why := ch.Env.Check(base, nil); why != "" {
		return fmt.Errorf("unmutated corpus is not valid: %s", why)
	}
	snap, err := ch.Env.Analyze(base)
	if err != nil {
		return err
	}
	pkgs, err := LoadPackages(base)
	if err != nil {
		return err
	}
	ch.corpus = &Corpus{Root: base, Packages: pkgs, Baseline: snap}
	return nil
}

func (ch *Chainer) Cleanup() { _ = os.RemoveAll(ch.base) }

func (ch *Chainer) Replay(steps []ChainStep) ChainOutcome {
	return ch.Walk(&recordedPicker{steps: steps})
}

func (ch *Chainer) Walk(p Picker) ChainOutcome {
	w := &walk{corpus: ch.corpus, edit: Edit{}, track: newTrack()}
	defer w.cleanup()
	for {
		pick, more, err := p.Next(ch.Ops, w.corpus)
		if err != nil {
			return w.outcome(Inapplicable, "", err.Error())
		}
		if !more {
			break
		}
		applied, err := pick.Op.Apply(w.corpus, pick.Site, pick.Dose)
		if err != nil {
			if _, recorded := p.(*recordedPicker); recorded {
				return w.outcome(Inapplicable, "", fmt.Sprintf("%s on %s: %v", pick.Op.ID(), pick.Site.Symbol, err))
			}
			continue
		}
		if err := ch.advance(w, pick, applied); err != nil {
			return w.outcome(Inapplicable, "", err.Error())
		}
	}
	return ch.judge(w)
}

func (ch *Chainer) advance(w *walk, pick Pick, applied Applied) error {
	w.edit = mergeEdits(w.edit, applied.Edit)
	w.steps = append(w.steps, ChainStep{Operator: pick.Op.ID(), Symbol: pick.Site.Symbol, Dose: pick.Dose})
	w.choices = append(w.choices, pick.Choices...)
	w.track.add(pick.Op, applied.Expectation)
	dir, err := ch.materialize(w.edit)
	w.dirs = append(w.dirs, dir)
	if err != nil {
		return err
	}
	pkgs, err := LoadPackages(dir)
	if err != nil {
		return fmt.Errorf("after %s: %w", pick.Op.ID(), err)
	}
	w.corpus = &Corpus{Root: dir, Packages: pkgs, Baseline: ch.corpus.Baseline}
	return nil
}

func (ch *Chainer) materialize(edit Edit) (string, error) {
	dir, err := TempDir("calibration-chain-")
	if err != nil {
		return "", err
	}
	if err := CopyModule(ch.base, dir); err != nil {
		return dir, err
	}
	return dir, WriteEdit(dir, edit)
}

func (ch *Chainer) judge(w *walk) ChainOutcome {
	invariant := w.track.invariant()
	if len(w.steps) == 0 {
		return w.outcome(Vacuous, "", "no step applied")
	}
	if invariant == "" {
		return w.outcome(Vacuous, "", "no inject predicted to fire and not neutral-only")
	}
	dir := w.corpus.Root
	if why := ch.Env.Check(dir, editedPackages(w.edit)); why != "" {
		return w.outcome(Inapplicable, invariant, "chained code is not valid: "+why)
	}
	snap, err := ch.Env.Analyze(dir)
	if err != nil {
		return w.outcome(Inapplicable, invariant, err.Error())
	}
	before, after := ch.corpus.Baseline.Findings, snap.Findings
	if invariant == NeutralKeepsIDs {
		o := Outcome{Class: Neutral}
		Score(&o, w.track.expectation(), before, after)
		if o.Status == IDDrift {
			return w.outcome(Violated, invariant, o.Reason)
		}
		return w.outcome(Holds, invariant, o.Reason)
	}
	added, _ := Diff(renamed(before, w.track.renames), after)
	if h := w.track.noticed(added); len(h) > 0 {
		return w.outcome(Holds, invariant, "still reported as "+h[0].Smell+" "+h[0].CaseID)
	}
	return w.outcome(Violated, invariant, "no finding added on "+strings.Join(shortSymbols(w.track.touched), ", "))
}

// Shrink removes one step at a time while the same invariant stays violated, so the result is 1-minimal.
func (ch *Chainer) Shrink(o ChainOutcome) ChainOutcome {
	best := o
	for shrunk := true; shrunk && len(best.Steps) > 1; {
		shrunk = false
		for i := range best.Steps {
			candidate := ch.Replay(slices.Delete(slices.Clone(best.Steps), i, i+1))
			if candidate.Verdict == Violated && candidate.Invariant == best.Invariant {
				best, shrunk = candidate, true
				break
			}
		}
	}
	return best
}

type walk struct {
	corpus  *Corpus
	edit    Edit
	steps   []ChainStep
	choices []byte
	track   *track
	dirs    []string
}

func (w *walk) cleanup() {
	for _, d := range w.dirs {
		_ = os.RemoveAll(d)
	}
}

func (w *walk) outcome(v Verdict, inv Invariant, reason string) ChainOutcome {
	return ChainOutcome{Steps: w.steps, Invariant: inv, Verdict: v, Reason: reason, Choices: w.choices}
}

// track composes the per-step expectations into one, keyed by the symbols of the unmutated code.
// It counts what the pairwise scorers count: a planted smell, a smell an evade declares it still trips, or any smell for an AnyAdded evade.
type track struct {
	renames     map[string]string
	moves       map[string]string
	touched     []string
	smells      []string
	anyAdded    bool
	noticeable  bool
	neutralOnly bool
}

func newTrack() *track {
	return &track{renames: map[string]string{}, moves: map[string]string{}, neutralOnly: true}
}

func (t *track) add(op Operator, exp Expectation) {
	class := op.Class()
	t.neutralOnly = t.neutralOnly && class == Neutral
	if class == Inject && exp.Kill != nil && *exp.Kill {
		t.noticeable = true
		t.smells = append(t.smells, targetOf(op, exp))
	}
	if class == Evade {
		t.smells = append(t.smells, exp.Catches...)
		t.anyAdded = t.anyAdded || exp.AnyAdded
	}
	for sym, file := range exp.Moves {
		t.moves[t.original(sym)] = file
	}
	t.rename(exp.Renames)
	t.touched = append(t.touched, exp.Touched...)
}

func (t *track) rename(r map[string]string) {
	current := map[string]bool{}
	for orig, cur := range t.renames {
		current[cur] = true
		if next, ok := r[cur]; ok {
			t.renames[orig] = next
		}
	}
	for from, to := range r {
		if !current[from] {
			t.renames[from] = to
		}
	}
	for i, s := range t.touched {
		if to, ok := r[s]; ok {
			t.touched[i] = to
		}
	}
}

func targetOf(op Operator, exp Expectation) string {
	if exp.Target != "" {
		return exp.Target
	}
	return op.Target()
}

func (t *track) noticed(added []Finding) []Finding {
	if t.anyAdded {
		return hits(added, nil, t.touched)
	}
	return hits(added, t.smells, t.touched)
}

func (t *track) original(sym string) string {
	for orig, cur := range t.renames {
		if cur == sym {
			return orig
		}
	}
	return sym
}

func (t *track) invariant() Invariant {
	switch {
	case t.noticeable:
		return InjectNoticed
	case t.neutralOnly:
		return NeutralKeepsIDs
	}
	return ""
}

func (t *track) expectation() Expectation {
	return Expectation{Renames: t.renames, Moves: t.moves}
}

func shortSymbols(ss []string) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = shortSymbol(s)
	}
	return slices.Compact(slices.Sorted(slices.Values(out)))
}
