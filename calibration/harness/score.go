package harness

import (
	"fmt"
	"slices"
	"strings"
)

type findingKey struct{ smell, symbol string }

// Diff matches findings by smell and primary symbol, as multisets, so a secondary-key
// change such as a Data Clump's type list is not counted as an add and a remove.
func Diff(before, after []Finding) (added, removed []Finding) {
	count := map[findingKey]int{}
	for _, f := range before {
		count[findingKey{f.Smell, f.Symbol}]++
	}
	for _, f := range after {
		k := findingKey{f.Smell, f.Symbol}
		if count[k] > 0 {
			count[k]--
			continue
		}
		added = append(added, f)
	}
	count = map[findingKey]int{}
	for _, f := range after {
		count[findingKey{f.Smell, f.Symbol}]++
	}
	for _, f := range before {
		k := findingKey{f.Smell, f.Symbol}
		if count[k] > 0 {
			count[k]--
			continue
		}
		removed = append(removed, f)
	}
	return added, removed
}

// renamed carries findings on a renamed declaration over to its new symbol, so a smell
// that moves with its declaration is neither an add nor a remove.
func renamed(fs []Finding, renames map[string]string) []Finding {
	out := make([]Finding, len(fs))
	for i, f := range fs {
		if to, ok := renames[f.Symbol]; ok {
			f.Symbol = to
		}
		out[i] = f
	}
	return out
}

type scorer func(o *Outcome, exp Expectation, before, after []Finding)

var scorers = map[Class]scorer{
	Inject:  scoreInject,
	Evade:   scoreEvade,
	Neutral: scoreNeutral,
	Cure:    scoreCure,
}

func Score(o *Outcome, exp Expectation, before, after []Finding) {
	o.Added, o.Removed = Diff(renamed(before, exp.Renames), after)
	symbols := map[string]string{}
	for _, f := range before {
		symbols[f.CaseID] = f.Symbol
	}
	for i := range o.Removed {
		o.Removed[i].Symbol = symbols[o.Removed[i].CaseID]
	}
	o.Predict = exp.Prediction
	if exp.Target != "" {
		o.Target = exp.Target
	}
	scorers[o.Class](o, exp, before, after)
}

func hits(fs []Finding, smells []string, touched []string) []Finding {
	var out []Finding
	for _, f := range fs {
		if (smells == nil || slices.Contains(smells, f.Smell)) && (len(touched) == 0 || slices.Contains(touched, f.Symbol)) {
			out = append(out, f)
		}
	}
	return out
}

func scoreInject(o *Outcome, exp Expectation, _, _ []Finding) {
	if h := hits(o.Added, []string{o.Target}, exp.Touched); len(h) > 0 {
		o.Status, o.Reason = Killed, "added "+h[0].CaseID
		return
	}
	if exp.Kill != nil && !*exp.Kill {
		o.Status, o.Reason = Equivalent, exp.Reason
		return
	}
	o.Status, o.Reason = Survived, exp.Reason
}

// scoreEvade asks whether Columbo still reports a caught smell on a touched declaration
// after the evade, whether that case is new or survived from the baseline.
func scoreEvade(o *Outcome, exp Expectation, _, after []Finding) {
	catches := exp.Catches
	if len(catches) == 0 {
		catches = []string{o.Target}
	}
	if h := hits(after, catches, exp.Touched); len(h) > 0 {
		o.Status, o.Reason = Killed, "still reported as "+h[0].Smell+" "+h[0].CaseID
		return
	}
	if h := hits(o.Added, nil, exp.Touched); exp.AnyAdded && len(h) > 0 {
		o.Status, o.Reason = Killed, "evade set off "+h[0].Smell+" "+h[0].CaseID
		return
	}
	o.Status, o.Reason = Survived, "evade silenced "+o.Target
}

// scoreCure counts a smell as moved, not new, when the same smell left a touched
// declaration in the same mutant: extracting envious code carries its envy to the helper.
func scoreCure(o *Outcome, exp Expectation, _, _ []Finding) {
	if len(hits(o.Removed, []string{o.Target}, exp.Touched)) == 0 {
		o.Status, o.Reason = Survived, "target case remains"
		return
	}
	if h := hits(o.Added, []string{o.Target}, exp.Touched); len(h) > 0 {
		o.Status, o.Reason = Survived, "target moved into "+shortSymbol(h[0].Symbol)
		return
	}
	left := map[string]int{}
	for _, f := range hits(o.Removed, nil, exp.Touched) {
		left[f.Smell]++
	}
	left[o.Target]--
	var added, moved []string
	for _, f := range o.Added {
		name := f.Smell + "@" + shortSymbol(f.Symbol)
		if left[f.Smell] > 0 && containsSymbol(exp.Touched, f.Symbol) {
			left[f.Smell]--
			moved = append(moved, name)
			continue
		}
		added = append(added, name)
	}
	note := ""
	if len(moved) > 0 {
		note = "; moved " + strings.Join(moved, ", ")
	}
	if len(added) > 0 {
		o.Status, o.Reason = Conflict, "cleared target but added "+strings.Join(added, ", ")+note
		return
	}
	o.Status, o.Reason = Killed, "target cleared, nothing new added"+note
}

func containsSymbol(touched []string, s string) bool { return slices.Contains(touched, s) }

// scoreNeutral compares case IDs. A case whose primary declaration was renamed or moved
// must reappear under its new symbol; every other case must keep its exact ID.
func scoreNeutral(o *Outcome, exp Expectation, before, after []Finding) {
	ids := map[string]bool{}
	bySymbol := map[findingKey]int{}
	for _, f := range after {
		ids[f.CaseID] = true
		bySymbol[findingKey{f.Smell, f.Symbol}]++
	}
	var drift []string
	for _, f := range before {
		renamed, isRenamed := exp.Renames[f.Symbol]
		_, isMoved := exp.Moves[f.Symbol]
		switch {
		case ids[f.CaseID]:
		case isRenamed || isMoved:
			target := f.Symbol
			if isRenamed {
				target = renamed
			}
			k := findingKey{f.Smell, target}
			if bySymbol[k] == 0 {
				drift = append(drift, fmt.Sprintf("%s %s did not reappear as %s", f.Smell, f.CaseID, shortSymbol(target)))
			}
		default:
			drift = append(drift, fmt.Sprintf("%s %s on %s changed ID", f.Smell, f.CaseID, shortSymbol(f.Symbol)))
		}
	}
	if len(before) != len(after) {
		drift = append(drift, fmt.Sprintf("case count %d -> %d", len(before), len(after)))
	}
	if len(drift) > 0 {
		o.Status, o.Reason = IDDrift, strings.Join(drift, "; ")
		return
	}
	o.Status, o.Reason = Equivalent, "no case added, removed, or re-identified"
}

func shortSymbol(s string) string {
	if i := strings.LastIndex(s, "/"); i >= 0 {
		return s[i+1:]
	}
	return s
}
