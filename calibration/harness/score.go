package harness

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"
)

type findingKey struct{ smell, symbol string }

// Diff keys on smell and primary symbol so a secondary-key change, such as a Data Clump's type list, is not an add plus a remove.
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

// An injected evade reads only added findings because baseline cases can share a symbol (prose comments anchor on the package clause); a FromBaseline evade reads everything.
func scoreEvade(o *Outcome, exp Expectation, _, after []Finding) {
	catches := exp.Catches
	if len(catches) == 0 {
		catches = []string{o.Target}
	}
	seen := o.Added
	if exp.FromBaseline {
		seen = inFiles(after, exp.Files)
	}
	if h := hits(seen, catches, exp.Touched); len(h) > 0 {
		o.Status, o.Reason = Killed, "still reported as "+h[0].Smell+" "+h[0].CaseID
		return
	}
	if h := hits(o.Added, nil, exp.Touched); exp.AnyAdded && len(h) > 0 {
		o.Status, o.Reason = Killed, "evade set off "+h[0].Smell+" "+h[0].CaseID
		return
	}
	o.Status, o.Reason = Survived, "evade silenced "+o.Target
}

// A smell leaving one touched declaration for another moved rather than appeared: extracting envious code carries its envy to the helper.
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

// idShape follows SPEC "Case identity and ordering": keyed smells add a secondary key the snapshot does not store; anchored smells take the earliest supporting declaration.
type idShape struct{ file, symbol, keyed, anchored bool }

var declaredShape = idShape{file: true, symbol: true}

var idShapes = map[string]idShape{
	"data-clump":                {file: true, symbol: true, keyed: true, anchored: true},
	"duplicate-code":            {file: true, symbol: true, keyed: true, anchored: true},
	"selection-use-coupling":    {symbol: true, keyed: true},
	"repeated-variant-decision": {keyed: true},
}

func shapeOf(smell string) idShape {
	if s, ok := idShapes[smell]; ok {
		return s
	}
	return declaredShape
}

func (exp Expectation) relocate(d Decl) Decl {
	if file, ok := exp.Moves[d.Symbol]; ok {
		d.File = file
	}
	if symbol, ok := exp.Renames[d.Symbol]; ok {
		d.Symbol = symbol
	}
	return d
}

// caseID must reproduce Columbo's case identity function exactly.
func caseID(smell, file, symbol, key string) string {
	esc := strings.NewReplacer("%", "%25", "|", "%7C", "\r", "%0D", "\n", "%0A")
	h := sha256.Sum256([]byte(strings.Join([]string{esc.Replace(smell), esc.Replace(file), esc.Replace(symbol), esc.Replace(key)}, "|")))
	return "C-" + hex.EncodeToString(h[:])[:10]
}

type neutralMatch struct {
	exp     Expectation
	after   []Finding
	claimed []bool
}

func (m *neutralMatch) claim(pred func(Finding) bool) bool {
	for i, a := range m.after {
		if !m.claimed[i] && pred(a) {
			m.claimed[i] = true
			return true
		}
	}
	return false
}

func (m *neutralMatch) match(b Finding) string {
	shape := shapeOf(b.Smell)
	if shape.anchored {
		return m.matchAnchored(b)
	}
	moved := m.exp.relocate(Decl{File: b.File, Symbol: b.Symbol})
	file, symbol, nextFile, nextSymbol := "", "", "", ""
	if shape.file {
		file, nextFile = b.File, moved.File
	}
	if shape.symbol {
		symbol, nextSymbol = b.Symbol, moved.Symbol
	}
	switch {
	case file == nextFile && symbol == nextSymbol:
		if m.claim(func(a Finding) bool { return a.CaseID == b.CaseID }) {
			return ""
		}
		return fmt.Sprintf("%s %s on %s changed ID", b.Smell, b.CaseID, shortSymbol(b.Symbol))
	case !shape.keyed:
		want := caseID(b.Smell, nextFile, nextSymbol, "")
		if m.claim(func(a Finding) bool { return a.CaseID == want }) {
			return ""
		}
		return fmt.Sprintf("%s %s did not reappear as %s on %s", b.Smell, b.CaseID, want, shortSymbol(nextSymbol))
	default:
		if m.claim(func(a Finding) bool { return a.Smell == b.Smell && a.Symbol == moved.Symbol && a.File == moved.File }) {
			return ""
		}
		return fmt.Sprintf("%s %s did not reappear on %s", b.Smell, b.CaseID, shortSymbol(moved.Symbol))
	}
}

// An unmoved anchor must keep its exact ID because a neutral edit leaves the secondary key unchanged.
func (m *neutralMatch) matchAnchored(b Finding) string {
	want := make([]Decl, len(b.Support))
	for i, d := range b.Support {
		want[i] = m.exp.relocate(d)
	}
	var why string
	found := m.claim(func(a Finding) bool {
		if a.Smell != b.Smell || !sameDecls(a.Support, want) {
			return false
		}
		first := earliest(a.Support)
		switch {
		case a.File != first.File || a.Symbol != first.Symbol:
			why = fmt.Sprintf("%s %s anchored at %s, not its earliest supporting declaration %s", a.Smell, a.CaseID, shortSymbol(a.Symbol), shortSymbol(first.Symbol))
		case a.File == b.File && a.Symbol == b.Symbol && a.CaseID != b.CaseID:
			why = fmt.Sprintf("%s %s kept anchor %s but became %s", b.Smell, b.CaseID, shortSymbol(b.Symbol), a.CaseID)
		}
		return true
	})
	if !found {
		return fmt.Sprintf("%s %s on %s did not reappear with the same supporting declarations", b.Smell, b.CaseID, shortSymbol(b.Symbol))
	}
	return why
}

func sameDecls(got, want []Decl) bool {
	key := func(ds []Decl) []string {
		out := make([]string, len(ds))
		for i, d := range ds {
			out[i] = d.File + "|" + d.Symbol
		}
		slices.Sort(out)
		return slices.Compact(out)
	}
	return slices.Equal(key(got), key(want))
}

func earliest(ds []Decl) Decl {
	return slices.MinFunc(ds, func(x, y Decl) int {
		if c := strings.Compare(x.File, y.File); c != 0 {
			return c
		}
		return x.Offset - y.Offset
	})
}

func scoreNeutral(o *Outcome, exp Expectation, before, after []Finding) {
	m := &neutralMatch{exp: exp, after: after, claimed: make([]bool, len(after))}
	var drift []string
	for _, b := range before {
		if why := m.match(b); why != "" {
			drift = append(drift, why)
		}
	}
	for i, a := range after {
		if !m.claimed[i] {
			drift = append(drift, fmt.Sprintf("unexplained %s %s on %s", a.Smell, a.CaseID, shortSymbol(a.Symbol)))
		}
	}
	if len(drift) > 0 {
		o.Status, o.Reason = IDDrift, strings.Join(drift, "; ")
		return
	}
	kept := map[string]bool{}
	for _, a := range after {
		kept[a.CaseID] = true
	}
	remapped := 0
	for _, b := range before {
		if !kept[b.CaseID] {
			remapped++
		}
	}
	if remapped == 0 {
		o.Status, o.Reason = Equivalent, "no case added, removed, or re-identified"
		return
	}
	o.Status, o.Reason = Equivalent, fmt.Sprintf("%d case IDs re-identified as SPEC identity predicts", remapped)
}

func shortSymbol(s string) string {
	if i := strings.LastIndex(s, "/"); i >= 0 {
		return s[i+1:]
	}
	return s
}

func inFiles(fs []Finding, files []string) []Finding {
	if len(files) == 0 {
		return fs
	}
	var out []Finding
	for _, f := range fs {
		if slices.Contains(files, f.File) {
			out = append(out, f)
		}
	}
	return out
}
