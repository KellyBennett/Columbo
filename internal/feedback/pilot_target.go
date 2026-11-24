package feedback

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/KellyBennett/Columbo/internal/snapshotdb"
)

// CaseDeclaration is one declaration a case involves, as case_declarations records it.
type CaseDeclaration struct {
	Symbol, Path, Role string
}

// AnalyzedCase is one analyzer case with the declarations it involves and its
// measured values. It is the shape of every <prefix>.cases.json file.
type AnalyzedCase struct {
	snapshotdb.SummaryCasesRow
	Declarations []CaseDeclaration
	Metrics      []snapshotdb.SummaryMetricsRow
}

type TargetStatus string

const (
	TargetCleared          TargetStatus = "cleared"
	TargetPersisted        TargetStatus = "persisted"
	TargetPersistedChanged TargetStatus = "persisted-changed"
	TargetUnknown          TargetStatus = "unknown"
)

// TargetOutcome is the runner's own observation of what a revision did to the
// targeted finding. It is independent of any agent's judgment.
type TargetOutcome struct {
	Status   TargetStatus  `json:"status"`
	Reason   string        `json:"reason"`
	Baseline *AnalyzedCase `json:"baseline,omitempty"`
	Match    *AnalyzedCase `json:"match,omitempty"`
	NewCases []string      `json:"new_cases"`
}

func loadAnalyzedCases(ctx context.Context, queries snapshotdb.Querier) ([]AnalyzedCase, error) {
	rows, err := queries.SummaryCases(ctx)
	if err != nil {
		return nil, err
	}
	links, err := queries.SummaryCaseDeclarations(ctx)
	if err != nil {
		return nil, err
	}
	declarations := map[string][]CaseDeclaration{}
	for _, link := range links {
		declarations[link.CaseID] = append(declarations[link.CaseID], CaseDeclaration{Symbol: link.Symbol, Path: link.Path, Role: link.Role})
	}
	cases := make([]AnalyzedCase, len(rows))
	for i, row := range rows {
		metrics, err := queries.SummaryMetrics(ctx, snapshotdb.SummaryMetricsParams{CaseID: row.ID})
		if err != nil {
			return nil, err
		}
		cases[i] = AnalyzedCase{SummaryCasesRow: row, Declarations: declarations[row.ID], Metrics: metrics}
	}
	return cases, nil
}

func casesPath(prefix string) string { return prefix + ".cases.json" }

// readAnalyzedCases loads the cases an analyzer run recorded, refusing runs
// without valid evidence so that missing data stays unknown.
func readAnalyzedCases(result CommandResult, prefix string) ([]AnalyzedCase, error) {
	if !analyzerRan(result) {
		return nil, errors.New("analyzer run at " + prefix + " lacks valid evidence")
	}
	data, err := os.ReadFile(casesPath(prefix))
	if err != nil {
		return nil, err
	}
	var cases []AnalyzedCase
	return cases, json.Unmarshal(data, &cases)
}

// observeTarget classifies the target in one revision against the baseline
// analysis. Any missing evidence yields unknown.
func (t *pilotTrial) observeTarget(revision CommandResult, prefix string, renames map[string]string, renameErr error) *TargetOutcome {
	baseline, err := readAnalyzedCases(t.result.BaselineColumbo, filepath.Join(t.root, "baseline-columbo"))
	var current []AnalyzedCase
	if err == nil {
		current, err = readAnalyzedCases(revision, prefix)
	}
	if err == nil && renameErr != nil {
		err = fmt.Errorf("rename detection failed: %w", renameErr)
	}
	if err != nil {
		return &TargetOutcome{Status: TargetUnknown, Reason: "evidence unavailable: " + err.Error()}
	}
	outcome := classifyTarget(t.finding.CaseID, baseline, current, renames)
	return &outcome
}

func classifyTarget(targetID string, baseline, revision []AnalyzedCase, renames map[string]string) TargetOutcome {
	known := map[string]AnalyzedCase{}
	for _, c := range baseline {
		known[c.ID] = c
	}
	target, ok := known[targetID]
	if !ok {
		return TargetOutcome{Status: TargetUnknown, Reason: "rule 1: the target case is absent from the baseline analysis", NewCases: newCaseIDs(revision, known, "")}
	}
	outcome := TargetOutcome{Baseline: &target}
	if i := slices.IndexFunc(revision, func(c AnalyzedCase) bool { return c.ID == targetID }); i >= 0 {
		outcome.Status, outcome.Match = TargetPersisted, &revision[i]
		outcome.Reason = "rule 2: the same case ID is present in this revision"
	} else if match, reason := successor(target, revision, known, renames); match != nil {
		outcome.Status, outcome.Match, outcome.Reason = TargetPersistedChanged, match, reason
	} else {
		outcome.Status = TargetCleared
		outcome.Reason = "rule 4: no same-ID case, and no new " + target.Smell + " case shares a declaration symbol or the primary file"
	}
	matchID := ""
	if outcome.Match != nil {
		matchID = outcome.Match.ID
	}
	outcome.NewCases = newCaseIDs(revision, known, matchID)
	return outcome
}

// successor finds the revision case that carries the target forward under a
// new ID. Only new cases of the same smell qualify, so a pre-existing case in
// the same file cannot pose as the target.
func successor(target AnalyzedCase, revision []AnalyzedCase, known map[string]AnalyzedCase, renames map[string]string) (*AnalyzedCase, string) {
	symbols := target.symbols()
	primary := target.Path
	if renamed, ok := renames[primary]; ok {
		primary = renamed
	}
	var bySymbol, byPath *AnalyzedCase
	most := 0
	for i := range revision {
		candidate := &revision[i]
		if _, existed := known[candidate.ID]; existed || candidate.Smell != target.Smell {
			continue
		}
		if shared := candidate.sharedSymbols(symbols); shared > most {
			bySymbol, most = candidate, shared
		}
		if byPath == nil && candidate.Path == primary {
			byPath = candidate
		}
	}
	switch {
	case bySymbol != nil:
		return bySymbol, fmt.Sprintf("rule 3: new %s case %s shares %d declaration symbol(s) with the target", target.Smell, bySymbol.ID, most)
	case byPath != nil:
		return byPath, fmt.Sprintf("rule 3: new %s case %s shares the target's primary file %s", target.Smell, byPath.ID, primary)
	}
	return nil, ""
}

func (c AnalyzedCase) symbols() map[string]bool {
	symbols := map[string]bool{c.Symbol: true}
	for _, declaration := range c.Declarations {
		symbols[declaration.Symbol] = true
	}
	return symbols
}

func (c AnalyzedCase) sharedSymbols(symbols map[string]bool) int {
	shared := 0
	for symbol := range c.symbols() {
		if symbols[symbol] {
			shared++
		}
	}
	return shared
}

func newCaseIDs(revision []AnalyzedCase, known map[string]AnalyzedCase, exclude string) []string {
	ids := []string{}
	for _, c := range revision {
		if _, existed := known[c.ID]; !existed && c.ID != exclude {
			ids = append(ids, c.ID)
		}
	}
	return ids
}

// candidateRenames reports files the candidate renamed since baseline, old path to new.
func candidateRenames(ctx context.Context, dir, baseline, module string) (map[string]string, error) {
	if err := intendToAddUntracked(ctx, dir); err != nil {
		return nil, err
	}
	return renamedPaths(ctx, dir, module, baseline, "--", ".")
}

// renamedPaths maps repository paths renamed by a git diff to their new
// paths, relative to the module because analyzer paths are module-relative.
func renamedPaths(ctx context.Context, dir, module string, args ...string) (map[string]string, error) {
	out, err := git(ctx, dir, append([]string{"diff", "--no-ext-diff", "--no-textconv", "-M", "--name-status", "-z"}, args...)...)
	if err != nil {
		return nil, err
	}
	renames := map[string]string{}
	fields := strings.Split(strings.TrimSuffix(out, "\x00"), "\x00")
	for i := 0; i < len(fields); {
		if !strings.HasPrefix(fields[i], "R") && !strings.HasPrefix(fields[i], "C") {
			i += 2
			continue
		}
		if i+2 >= len(fields) {
			return nil, errors.New("truncated git rename record")
		}
		oldPath, oldOK := moduleRelative(module, fields[i+1])
		newPath, newOK := moduleRelative(module, fields[i+2])
		if strings.HasPrefix(fields[i], "R") && oldOK && newOK {
			renames[oldPath] = newPath
		}
		i += 3
	}
	return renames, nil
}

func moduleRelative(module, path string) (string, bool) {
	rel, err := filepath.Rel(filepath.Clean(module), filepath.FromSlash(path))
	if err != nil || escapes(rel) {
		return "", false
	}
	return filepath.ToSlash(rel), true
}
