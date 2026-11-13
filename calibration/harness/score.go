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

type scorer func(o *Outcome, exp Expectation, before, after []Finding)

var scorers = map[Class]scorer{
	Inject:  scoreInject,
	Evade:   scoreEvade,
	Neutral: scoreNeutral,
	Cure:    scoreCure,
}

func Score(o *Outcome, exp Expectation, before, after []Finding) {
	o.Added, o.Removed = Diff(before, after)
	o.Predict = exp.Prediction
	scorers[o.Class](o, exp, before, after)
}

func hits(fs []Finding, smells []string, touched []string) []Finding {
	var out []Finding
	for _, f := range fs {
		if slices.Contains(smells, f.Smell) && (len(touched) == 0 || slices.Contains(touched, f.Symbol)) {
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

func scoreEvade(o *Outcome, exp Expectation, _, _ []Finding) {
	catches := exp.Catches
	if len(catches) == 0 {
		catches = []string{o.Target}
	}
	if h := hits(o.Added, catches, exp.Touched); len(h) > 0 {
		o.Status, o.Reason = Killed, "still reported as "+h[0].Smell+" "+h[0].CaseID
		return
	}
	o.Status, o.Reason = Survived, "evade silenced "+o.Target
}

func scoreCure(o *Outcome, exp Expectation, _, _ []Finding) {
	if len(hits(o.Removed, []string{o.Target}, exp.Touched)) == 0 {
		o.Status, o.Reason = Survived, "target case remains"
		return
	}
	if len(o.Added) > 0 {
		names := make([]string, len(o.Added))
		for i, f := range o.Added {
			names[i] = f.Smell + "@" + shortSymbol(f.Symbol)
		}
		o.Status, o.Reason = Conflict, "cleared target but added "+strings.Join(names, ", ")
		return
	}
	o.Status, o.Reason = Killed, "target cleared, nothing added"
}

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
