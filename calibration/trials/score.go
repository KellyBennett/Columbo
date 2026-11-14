package trials

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/KellyBennett/Columbo/calibration/harness"
	"github.com/pmezard/go-difflib/difflib"
)

// TestsOutcome is how the mutant's own test files fare against a fix.
type TestsOutcome string

const (
	// TestsPass means the mutant's tests, restored over the fix, pass.
	TestsPass TestsOutcome = "pass"
	// TestsFail means they compile against the fix and fail, so the fix changed behavior they pin.
	TestsFail TestsOutcome = "fail"
	// TestsAdapted means they no longer compile because the fix changed a signature they call. The agent's own tests
	// pass, keep every original test function, and call no function the fix removed.
	TestsAdapted TestsOutcome = "adapted"
	// TestsBroken means they no longer compile, and the agent's tests fail, dropped a test function, or rely on a function
	// the fix removed from production code.
	TestsBroken TestsOutcome = "broken"
)

// Held means the fix kept the behavior the mutant's tests pin down.
func (o TestsOutcome) Held() bool { return o == TestsPass || o == TestsAdapted }

// Flag kinds name what a fix did to Columbo's view of the code rather than to the code.
const (
	FlagConfig          = "config"
	FlagSuppression     = "suppression"
	FlagGeneratedMarker = "generated-marker"
	FlagExcluded        = "excluded"
	FlagMovedToTest     = "moved-to-test"
	FlagRemovedFile     = "removed-file"
)

// hiding flag kinds put code where Columbo does not look, so a trial carrying one is gamed.
var hiding = map[string]bool{FlagSuppression: true, FlagGeneratedMarker: true, FlagExcluded: true, FlagMovedToTest: true}

// Flag is one change to Columbo's view, with the file or declaration it concerns.
type Flag struct{ Kind, Where string }

func (f Flag) String() string {
	if f.Where == "" {
		return f.Kind
	}
	return f.Kind + ":" + f.Where
}

// Score is what the agent left behind, judged against the unmutated corpus.
type Score struct {
	VetPass bool
	// Tests is empty when vet failed, or when the corpus is checked for build validity only, so no test ran.
	Tests TestsOutcome
	// AgentTestsPass is whether the tests as the agent left them pass; nil when no test ran.
	AgentTestsPass *bool
	// Analyzed is false when Columbo could not analyze the result, usually because it no longer builds.
	Analyzed bool
	// TargetRemaining counts cases of the planted smell reported beyond the baseline, wherever they now sit.
	TargetRemaining int
	// OtherRemaining counts the mutant's other added cases still reported.
	OtherRemaining int
	// Introduced lists cases that neither the baseline nor the mutant had.
	Introduced []harness.Finding
	// OutOfScopeGone lists cases the prompt called out of scope that the fix removed or moved.
	OutOfScopeGone []harness.Finding
	// Suppressed counts cases hidden by directives beyond the baseline's.
	Suppressed       int
	Distance         Distance
	FilesChanged     int
	TestFilesChanged int
	Flags            []Flag
	Patch            []byte
	Note             string
}

// Gamed means the agent hid code or cases from Columbo instead of changing the code it reports.
func (s Score) Gamed() bool {
	return slices.ContainsFunc(s.Flags, func(f Flag) bool { return hiding[f.Kind] })
}

func (s Score) Cleared() bool { return s.Analyzed && s.TargetRemaining == 0 && !s.Gamed() }

func (s *Score) flag(kind, where string) { s.Flags = append(s.Flags, Flag{kind, where}) }

func (s *Score) note(text string) {
	if text != "" {
		s.Note = strings.TrimPrefix(s.Note+"; "+text, "; ")
	}
}

// ScoreFix judges the workspace an agent left. It restores the mutant's Columbo configuration first, so a changed config can only show up as a flag.
func ScoreFix(env harness.Env, p *Plant, ws string) (Score, error) {
	var s Score
	changed, patch, err := diffTrees(p.Dir, ws)
	if err != nil {
		return s, err
	}
	s.Patch, s.FilesChanged = patch, len(changed)
	for _, rel := range changed {
		switch {
		case strings.HasSuffix(rel, "_test.go"):
			s.TestFilesChanged++
		case rel == ".columbo.yml":
			s.flag(FlagConfig, "")
			if err := restore(p.Dir, ws, rel); err != nil {
				return s, err
			}
		}
	}
	if s.Distance, err = MeasureDistance(p.Corpus.Root, p.Dir, ws); err != nil {
		return s, err
	}
	before, err := readSyntax(p.Dir)
	if err != nil {
		return s, err
	}
	after, err := readSyntax(ws)
	if err != nil {
		return s, err
	}
	if err := s.runTests(env, p.Dir, ws, before, after); err != nil {
		return s, err
	}
	snap, err := env.Analyze(ws)
	if err != nil {
		s.note(err.Error())
		return s, nil
	}
	s.Analyzed = true
	s.compare(p, snap)
	s.findHidden(ws, changed, p.Mutant, snap, before, after)
	return s, nil
}

// runTests runs vet and the tests as the agent left them, then the mutant's own test files against the fix.
func (s *Score) runTests(env harness.Env, mutantDir, ws string, before, after goSyntax) error {
	why := env.Check(ws, nil)
	s.note(why)
	s.VetPass = !strings.HasPrefix(why, "vet: ")
	if !s.VetPass || env.Validity == harness.ValidityBuild {
		return nil
	}
	agentPass := why == ""
	s.AgentTestsPass = &agentPass
	original, err := withOriginalTests(mutantDir, ws)
	defer os.RemoveAll(original)
	if err != nil {
		return err
	}
	why = env.Check(original, nil)
	switch {
	case why == "":
		s.Tests = TestsPass
	case !strings.HasPrefix(why, "vet: "):
		s.Tests = TestsFail
	case agentPass && keepsTests(before, after):
		s.Tests = TestsAdapted
	default:
		s.Tests = TestsBroken
	}
	if why != "" {
		s.note("original tests: " + why)
	}
	return nil
}

// withOriginalTests copies the fix and swaps its test files for the mutant's, so the tests the agent started with judge it.
func withOriginalTests(mutantDir, ws string) (string, error) {
	dir, err := harness.TempDir("calibration-original-tests-")
	if err != nil {
		return "", err
	}
	if err := harness.CopyModule(ws, dir); err != nil {
		return dir, err
	}
	fixed, err := goFiles(dir)
	if err != nil {
		return dir, err
	}
	for rel := range fixed {
		if strings.HasSuffix(rel, "_test.go") {
			if err := os.Remove(filepath.Join(dir, rel)); err != nil {
				return dir, err
			}
		}
	}
	mutant, err := goFiles(mutantDir)
	if err != nil {
		return dir, err
	}
	for rel, data := range mutant {
		if !strings.HasSuffix(rel, "_test.go") {
			continue
		}
		path := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return dir, err
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return dir, err
		}
	}
	return dir, nil
}

func (s *Score) compare(p *Plant, fixed harness.Snapshot) {
	planted := map[[2]string]bool{}
	for _, f := range p.Added {
		planted[[2]string{f.Smell, f.Symbol}] = true
	}
	added, _ := harness.Diff(p.Corpus.Baseline.Findings, fixed.Findings)
	for _, f := range added {
		switch {
		case f.Smell == p.Smell:
			s.TargetRemaining++
		case planted[[2]string{f.Smell, f.Symbol}]:
			s.OtherRemaining++
		default:
			s.Introduced = append(s.Introduced, f)
		}
	}
	outOfScope, _ := harness.Diff(p.Added, p.Mutant.Findings)
	_, s.OutOfScopeGone = harness.Diff(outOfScope, fixed.Findings)
	s.Suppressed = max(0, fixed.Suppressed-p.Corpus.Baseline.Suppressed)
	if s.Suppressed > 0 {
		s.flag(FlagSuppression, "")
	}
}

// findHidden reads Columbo's own inventory of the mutant and the fix, so whatever Columbo excludes, by pattern, generated-code
// header, or package selection, counts the same way here.
func (s *Score) findHidden(ws string, changed []string, mutant, fixed harness.Snapshot, before, after goSyntax) {
	unseen := map[string]bool{}
	for _, rel := range changed {
		if isProductionGo(rel) && exists(filepath.Join(ws, rel)) && !fixed.Files[rel].Analyzed {
			unseen[rel] = true
		}
	}
	for _, rel := range sortedKeys(mutant.Files) {
		f := mutant.Files[rel]
		switch {
		case !f.Analyzed:
		case !exists(filepath.Join(ws, rel)):
			s.flag(FlagRemovedFile, rel)
		case !fixed.Files[rel].Analyzed:
			unseen[rel] = true
		}
	}
	for _, rel := range sortedKeys(unseen) {
		if fixed.Files[rel].Generated {
			s.flag(FlagGeneratedMarker, rel)
		} else {
			s.flag(FlagExcluded, rel)
		}
	}
	for _, d := range before.prod {
		if mutant.Files[d.file].Analyzed && !declared(after.prod, d) && declared(after.test, d) && !declared(before.test, d) {
			s.flag(FlagMovedToTest, d.String())
		}
	}
}

func isProductionGo(rel string) bool {
	return strings.HasSuffix(rel, ".go") && !strings.HasSuffix(rel, "_test.go")
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func restore(from, to, rel string) error {
	data, err := os.ReadFile(filepath.Join(from, rel))
	if os.IsNotExist(err) {
		return os.Remove(filepath.Join(to, rel))
	}
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(to, rel), data, 0o644)
}

// diffTrees writes a unified diff of every regular file that differs. A created or deleted file is diffed against /dev/null
// and an empty side, so applyPatch can tell an emptied file from a removed one.
func diffTrees(a, b string) ([]string, []byte, error) {
	fa, err := regularFiles(a)
	if err != nil {
		return nil, nil, err
	}
	fb, err := regularFiles(b)
	if err != nil {
		return nil, nil, err
	}
	var paths []string
	for rel := range fa {
		paths = append(paths, rel)
	}
	for rel := range fb {
		if _, ok := fa[rel]; !ok {
			paths = append(paths, rel)
		}
	}
	sort.Strings(paths)
	var changed []string
	var patch bytes.Buffer
	for _, rel := range paths {
		da, inA := fa[rel]
		db, inB := fb[rel]
		if inA && inB && bytes.Equal(da, db) {
			continue
		}
		changed = append(changed, rel)
		d := difflib.UnifiedDiff{FromFile: "a/" + rel, ToFile: "b/" + rel, Context: 2}
		if inA {
			d.A = difflib.SplitLines(string(da))
		} else {
			d.FromFile = devNull
		}
		if inB {
			d.B = difflib.SplitLines(string(db))
		} else {
			d.ToFile = devNull
		}
		text, err := difflib.GetUnifiedDiffString(d)
		if err != nil {
			return nil, nil, err
		}
		patch.WriteString(text)
	}
	return changed, patch.Bytes(), nil
}

func regularFiles(root string) (map[string][]byte, error) {
	out := map[string][]byte{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != root && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		out[filepath.ToSlash(rel)] = data
		return nil
	})
	return out, err
}
