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

type CaseDeclaration struct {
	Symbol, Path, Role string
}

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
	declarations, err := loadCaseDeclarations(ctx, queries)
	if err != nil {
		return nil, err
	}
	return declarations.measuredCases(ctx, queries, rows)
}

type caseDeclarations map[string][]CaseDeclaration

func loadCaseDeclarations(ctx context.Context, queries snapshotdb.Querier) (caseDeclarations, error) {
	links, err := queries.SummaryCaseDeclarations(ctx)
	if err != nil {
		return nil, err
	}
	declarations := caseDeclarations{}
	for _, link := range links {
		declarations[link.CaseID] = append(declarations[link.CaseID], CaseDeclaration{Symbol: link.Symbol, Path: link.Path, Role: link.Role})
	}
	return declarations, nil
}

func (d caseDeclarations) measuredCases(ctx context.Context, queries snapshotdb.Querier, rows []snapshotdb.SummaryCasesRow) ([]AnalyzedCase, error) {
	cases := make([]AnalyzedCase, len(rows))
	for i, row := range rows {
		cases[i] = AnalyzedCase{SummaryCasesRow: row, Declarations: d[row.ID]}
		if err := cases[i].loadMetrics(ctx, queries); err != nil {
			return nil, err
		}
	}
	return cases, nil
}

func (c *AnalyzedCase) loadMetrics(ctx context.Context, queries snapshotdb.Querier) (err error) {
	c.Metrics, err = queries.SummaryMetrics(ctx, snapshotdb.SummaryMetricsParams{CaseID: c.ID})
	return err
}

func casesPath(prefix string) string { return prefix + ".cases.json" }

func readValidAnalyzedCases(result CommandResult, prefix string) ([]AnalyzedCase, error) {
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

func (t *pilotTrial) observeTarget(revision CommandResult, prefix string, renames pathRenames, renameErr error) *TargetOutcome {
	baseline, err := readValidAnalyzedCases(t.result.BaselineColumbo, filepath.Join(t.root, "baseline-columbo"))
	var current []AnalyzedCase
	if err == nil {
		current, err = readValidAnalyzedCases(revision, prefix)
	}
	if err == nil && renameErr != nil {
		err = fmt.Errorf("rename detection failed: %w", renameErr)
	}
	if err != nil {
		return &TargetOutcome{Status: TargetUnknown, Reason: "evidence unavailable: " + err.Error()}
	}
	outcome := newTargetSearch(baseline, current, renames).classify(t.finding.CaseID)
	return &outcome
}

func (t *pilotTrial) analyzeWithTarget(ctx context.Context, dir, prefix string, renames func(context.Context) (pathRenames, error)) (CommandResult, *TargetOutcome) {
	columbo := analyzeRevision(ctx, t.options, dir, prefix)
	found, err := renames(ctx)
	return columbo, t.observeTarget(columbo, prefix, found, err)
}

type targetSearch struct {
	baseline map[string]AnalyzedCase
	revision []AnalyzedCase
	renames  pathRenames
}

func newTargetSearch(baseline, revision []AnalyzedCase, renames pathRenames) targetSearch {
	known := map[string]AnalyzedCase{}
	for _, c := range baseline {
		known[c.ID] = c
	}
	return targetSearch{baseline: known, revision: revision, renames: renames}
}

func (s targetSearch) classify(targetID string) TargetOutcome {
	target, ok := s.baseline[targetID]
	if !ok {
		return TargetOutcome{Status: TargetUnknown, Reason: "rule 1: the target case is absent from the baseline analysis", NewCases: s.newCaseIDs("")}
	}
	outcome := s.follow(target)
	outcome.NewCases = s.newCaseIDs(outcome.matchID())
	return outcome
}

func (s targetSearch) follow(target AnalyzedCase) TargetOutcome {
	if i := slices.IndexFunc(s.revision, func(c AnalyzedCase) bool { return c.ID == target.ID }); i >= 0 {
		return TargetOutcome{Status: TargetPersisted, Reason: "rule 2: the same case ID is present in this revision", Baseline: &target, Match: &s.revision[i]}
	}
	if match, reason := s.successor(target); match != nil {
		return TargetOutcome{Status: TargetPersistedChanged, Reason: reason, Baseline: &target, Match: match}
	}
	return TargetOutcome{Status: TargetCleared, Reason: "rule 4: no same-ID case, and no new " + target.Smell + " case shares a declaration symbol or the primary file", Baseline: &target}
}

func (s targetSearch) successor(target AnalyzedCase) (*AnalyzedCase, string) {
	candidates := s.introducedCasesOf(target.Smell)
	if match, shared := target.closestBySymbols(candidates); match != nil {
		return match, fmt.Sprintf("rule 3: new %s case %s shares %d declaration symbol(s) with the target", target.Smell, match.ID, shared)
	}
	primary := s.renames.current(target.Path)
	if i := slices.IndexFunc(candidates, func(c *AnalyzedCase) bool { return c.Path == primary }); i >= 0 {
		return candidates[i], fmt.Sprintf("rule 3: new %s case %s shares the target's primary file %s", target.Smell, candidates[i].ID, primary)
	}
	return nil, ""
}

func (s targetSearch) introducedCasesOf(smell string) []*AnalyzedCase {
	var cases []*AnalyzedCase
	for i := range s.revision {
		if candidate := &s.revision[i]; s.introduced(*candidate) && candidate.Smell == smell {
			cases = append(cases, candidate)
		}
	}
	return cases
}

func (s targetSearch) introduced(c AnalyzedCase) bool {
	_, existed := s.baseline[c.ID]
	return !existed
}

func (s targetSearch) newCaseIDs(excluded string) []string {
	ids := []string{}
	for _, c := range s.revision {
		if s.introduced(c) && c.ID != excluded {
			ids = append(ids, c.ID)
		}
	}
	return ids
}

func (o *TargetOutcome) matchID() string {
	if o.Match == nil {
		return ""
	}
	return o.Match.ID
}

func (c AnalyzedCase) closestBySymbols(candidates []*AnalyzedCase) (*AnalyzedCase, int) {
	symbols := c.symbols()
	var closest *AnalyzedCase
	most := 0
	for _, candidate := range candidates {
		if shared := candidate.sharedSymbols(symbols); shared > most {
			closest, most = candidate, shared
		}
	}
	return closest, most
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

func (c AnalyzedCase) metric(kind string) (any, bool) {
	i := slices.IndexFunc(c.Metrics, func(m snapshotdb.SummaryMetricsRow) bool { return m.Kind == kind })
	if i < 0 {
		return nil, false
	}
	return c.Metrics[i].NumericValue, true
}

func (t *pilotTrial) candidateRenames(ctx context.Context) (pathRenames, error) {
	if err := intendToAddUntracked(ctx, t.candidate); err != nil {
		return nil, err
	}
	return moduleRelativeRenames(ctx, t.candidate, t.options.Module, t.finding.Before, "--", ".")
}

func (t *pilotTrial) actualRenames(ctx context.Context) (pathRenames, error) {
	return moduleRelativeRenames(ctx, t.finding.Latest.GitDir, t.options.Module, t.finding.Before, t.finding.After, "--")
}

func moduleRelativeRenames(ctx context.Context, dir, module string, args ...string) (pathRenames, error) {
	out, err := git(ctx, dir, append([]string{"diff", "--no-ext-diff", "--no-textconv", "-M", "--name-status", "-z"}, args...)...)
	if err != nil {
		return nil, err
	}
	return parseRenames(out, module)
}

type pathRenames map[string]string

func (r pathRenames) current(path string) string {
	if renamed, ok := r[path]; ok {
		return renamed
	}
	return path
}

func parseRenames(nameStatus, module string) (pathRenames, error) {
	records, err := nameStatusRecords(nameStatus)
	if err != nil {
		return nil, err
	}
	renames := pathRenames{}
	for _, record := range records {
		renames.record(module, record)
	}
	return renames, nil
}

func (r pathRenames) record(module string, record []string) {
	if !strings.HasPrefix(record[0], "R") {
		return
	}
	oldPath, oldOK := moduleRelative(module, record[1])
	newPath, newOK := moduleRelative(module, record[2])
	if oldOK && newOK {
		r[oldPath] = newPath
	}
}

func nameStatusRecords(nameStatus string) ([][]string, error) {
	fields := strings.Split(strings.TrimSuffix(nameStatus, "\x00"), "\x00")
	var records [][]string
	for len(fields) > 0 {
		width := min(nameStatusWidth(fields[0]), len(fields))
		records, fields = append(records, fields[:width]), fields[width:]
	}
	if last := records[len(records)-1]; pairsPaths(last[0]) && len(last) < 3 {
		return nil, errors.New("truncated git rename record")
	}
	return records, nil
}

func nameStatusWidth(status string) int {
	if pairsPaths(status) {
		return 3
	}
	return 2
}

func pairsPaths(status string) bool {
	return strings.HasPrefix(status, "R") || strings.HasPrefix(status, "C")
}

func moduleRelative(module, path string) (string, bool) {
	rel, err := filepath.Rel(filepath.Clean(module), filepath.FromSlash(path))
	if err != nil || escapes(rel) {
		return "", false
	}
	return filepath.ToSlash(rel), true
}
