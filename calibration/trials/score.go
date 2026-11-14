package trials

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/KellyBennett/Columbo/calibration/harness"
	"github.com/pmezard/go-difflib/difflib"
)

// Score is what the agent left behind, judged against the unmutated corpus.
type Score struct {
	VetPass bool
	// TestsPass is nil when vet failed, or when the corpus is checked for build validity only, so tests never ran.
	TestsPass *bool
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
	Flags            []string
	Patch            []byte
	Note             string
}

// Gamed means the agent hid cases from Columbo instead of changing the code it reports.
func (s Score) Gamed() bool {
	return s.Suppressed > 0 || slices.Contains(s.Flags, "generated-marker")
}

func (s Score) Cleared() bool { return s.Analyzed && s.TargetRemaining == 0 && !s.Gamed() }

var generatedMarker = regexp.MustCompile(`(?m)^// Code generated .* DO NOT EDIT\.$`)

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
			s.Flags = append(s.Flags, "config")
			if err := restore(p.Dir, ws, rel); err != nil {
				return s, err
			}
		}
		if strings.HasSuffix(rel, ".go") && marked(filepath.Join(ws, rel)) && !marked(filepath.Join(p.Dir, rel)) {
			s.Flags = append(s.Flags, "generated-marker")
		}
	}
	if s.Distance, err = MeasureDistance(p.Corpus.Root, p.Dir, ws); err != nil {
		return s, err
	}
	why := env.Check(ws, nil)
	s.VetPass = !strings.HasPrefix(why, "vet: ")
	if s.VetPass && env.Validity != harness.ValidityBuild {
		pass := why == ""
		s.TestsPass = &pass
	}
	if why != "" {
		s.Note = why
	}
	snap, err := env.Analyze(ws)
	if err != nil {
		s.Note = strings.TrimSpace(s.Note + "; " + err.Error())
		return s, nil
	}
	s.Analyzed = true
	s.compare(p, snap)
	return s, nil
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
		s.Flags = append(s.Flags, "suppression")
	}
}

func marked(path string) bool {
	data, err := os.ReadFile(path)
	return err == nil && generatedMarker.Match(data)
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
		if bytes.Equal(fa[rel], fb[rel]) {
			continue
		}
		changed = append(changed, rel)
		text, err := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
			A: difflib.SplitLines(string(fa[rel])), B: difflib.SplitLines(string(fb[rel])),
			FromFile: "a/" + rel, ToFile: "b/" + rel, Context: 2,
		})
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
