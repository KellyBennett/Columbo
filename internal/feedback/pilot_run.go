package feedback

import (
	"cmp"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/KellyBennett/Columbo/internal/snapshotdb"
)

type DesignAssessment struct {
	Relationship string   `json:"relationship"`
	Opportunity  string   `json:"opportunity"`
	FutureChange string   `json:"future_change"`
	ChangeBasis  string   `json:"change_basis"`
	Costs        string   `json:"costs"`
	Evidence     []string `json:"evidence"`
}

type Judgment struct {
	Outcome  string   `json:"outcome"`
	Reason   string   `json:"reason"`
	Evidence []string `json:"evidence"`
}

type DesignReview struct {
	Finding       Judgment `json:"finding"`
	Direction     Judgment `json:"direction"`
	Candidate     Judgment `json:"candidate"`
	Actual        Judgment `json:"actual"`
	Limitations   []string `json:"limitations"`
	ReportedModel string   `json:"reported_model"`
}

type RefactoringAttempt struct {
	Number        int             `json:"number"`
	Agent         CommandResult   `json:"agent"`
	Checks        []CommandResult `json:"checks"`
	TestScope     TestScope       `json:"test_scope"`
	Columbo       CommandResult   `json:"columbo"`
	Patch         string          `json:"patch"`
	PolicyChanges []string        `json:"policy_changes,omitempty"`
	TestChanges   []string        `json:"test_changes,omitempty"`
	HasChanges    bool            `json:"has_changes"`
	Gofmt         *CommandResult  `json:"gofmt,omitempty"`
	Unformatted   []string        `json:"unformatted,omitempty"`
	Target        *TargetOutcome  `json:"target,omitempty"`
}

type TrialResult struct {
	ActualChange               ActualChange         `json:"actual_change"`
	AnalysisMode               string               `json:"analysis_mode"`
	ActualBehaviorChecksPassed bool                 `json:"actual_behavior_checks_passed"`
	Version                    int                  `json:"version"`
	Complete                   bool                 `json:"complete"`
	CaseID                     string               `json:"case_id"`
	Execution                  string               `json:"execution"`
	Before                     string               `json:"before"`
	After                      string               `json:"after,omitempty"`
	RevisionEvidence           string               `json:"revision_evidence"`
	ChangedPaths               []string             `json:"changed_paths,omitempty"`
	BaselineChecks             []CommandResult      `json:"baseline_checks"`
	BaselineTestScope          TestScope            `json:"baseline_test_scope"`
	BaselineColumbo            CommandResult        `json:"baseline_columbo"`
	AssessmentCommands         []CommandResult      `json:"assessment_commands"`
	Assessment                 DesignAssessment     `json:"assessment"`
	Attempts                   []RefactoringAttempt `json:"attempts"`
	ActualChecks               []CommandResult      `json:"actual_checks,omitempty"`
	ActualTestScope            *TestScope           `json:"actual_test_scope,omitempty"`
	ActualColumbo              *CommandResult       `json:"actual_columbo,omitempty"`
	ActualTarget               *TargetOutcome       `json:"actual_target,omitempty"`
	ActualAncestry             string               `json:"actual_ancestry"`
	ReviewCommands             []CommandResult      `json:"review_commands"`
	Review                     DesignReview         `json:"review"`
	BehaviorChecksPassed       bool                 `json:"behavior_checks_passed"`
	Error                      string               `json:"error,omitempty"`
	AnalysisConfig             string               `json:"analysis_config"`
	AnalysisConfigSHA256       string               `json:"analysis_config_sha256"`
	AnalysisConfigOrigin       string               `json:"analysis_config_origin"`
}

type pilotTrial struct {
	options                         PilotOptions
	finding                         PilotCase
	result                          TrialResult
	root, before, candidate, actual string
	actualPaths                     []string
}

func runPilotCase(ctx context.Context, options PilotOptions, finding PilotCase) (err error) {
	resultPath := filepath.Join(options.Out, "trials", finding.ID, "result.json")
	if complete, err := trialComplete(resultPath); complete || err != nil {
		return err
	}
	t := newPilotTrial(ctx, options, finding)
	defer func() { err = t.save(err, resultPath) }()
	for _, step := range t.steps() {
		if err := step(ctx); err != nil {
			return err
		}
	}
	return nil
}

func trialComplete(resultPath string) (bool, error) {
	data, err := os.ReadFile(resultPath)
	if os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	var previous TrialResult
	if err := json.Unmarshal(data, &previous); err != nil {
		return false, err
	}
	return previous.Complete && previous.Version == pilotVersion, nil
}

func newPilotTrial(ctx context.Context, options PilotOptions, finding PilotCase) *pilotTrial {
	if finding.ActualChange.Status == "" {
		finding.ActualChange = finding.inspectActualChange(ctx)
	}
	root := filepath.Join(options.Out, "trials", finding.ID, "executions", rand.Text())
	return &pilotTrial{options: options, finding: finding, result: finding.trialResult(root), root: root, before: filepath.Join(root, "before"), candidate: filepath.Join(root, "candidate")}
}

func (c PilotCase) trialResult(execution string) TrialResult {
	return TrialResult{Version: pilotVersion, ActualChange: c.ActualChange, AnalysisMode: c.analysisMode(), CaseID: c.CaseID, Execution: execution, Before: c.Before, After: c.After, RevisionEvidence: c.RevisionEvidence, ActualAncestry: "unavailable"}
}

func (t *pilotTrial) steps() []func(context.Context) error {
	if !t.finding.explores() && t.finding.ActualChange.Status == actualNoResponse {
		return []func(context.Context) error{t.recordNoResponse}
	}
	return []func(context.Context) error{t.prepare, t.assess, t.refactor, t.collectActual, t.validateBaseline, t.review}
}

func (t *pilotTrial) recordNoResponse(context.Context) error {
	if err := saveJSON(filepath.Join(t.root, "discussion.json"), t.finding); err != nil {
		return err
	}
	skipped := Judgment{Outcome: "not-applicable", Reason: "No later change touches the finding, so no design comparison was performed.", Evidence: []string{t.finding.ActualChange.Reason}}
	t.result.Review = DesignReview{Finding: skipped, Direction: skipped, Candidate: skipped, Actual: t.finding.ActualChange.judgment()}
	t.result.Complete = true
	return nil
}

func (t *pilotTrial) save(err error, resultPath string) error {
	if err != nil {
		t.result.Error = err.Error()
	}
	if saveErr := saveJSON(filepath.Join(t.root, "result.json"), t.result); err == nil {
		err = saveErr
	}
	if saveErr := saveJSON(resultPath, t.result); err == nil {
		err = saveErr
	}
	return err
}

func (t *pilotTrial) prepare(ctx context.Context) error {
	if err := t.checkoutBefore(ctx); err != nil {
		return err
	}
	if err := checkModule(t.before, t.options.Module); err != nil {
		return err
	}
	if err := t.freezeConfig(); err != nil {
		return err
	}
	var err error
	if t.actualPaths, err = t.finding.actualPaths(ctx); err != nil {
		return err
	}
	t.result.ChangedPaths = t.actualPaths
	t.result.BaselineColumbo = analyzeRevision(ctx, t.options, t.before, filepath.Join(t.root, "baseline-columbo"))
	return nil
}

func (t *pilotTrial) checkoutBefore(ctx context.Context) error {
	dirs := []string{t.before}
	if t.finding.explores() {
		dirs = append(dirs, t.candidate)
	} else {
		t.candidate = ""
	}
	for _, dir := range dirs {
		if err := checkoutRevision(ctx, t.finding.Initial.GitDir, t.finding.Before, dir); err != nil {
			return err
		}
	}
	return nil
}

func (c PilotCase) actualPaths(ctx context.Context) ([]string, error) {
	if !c.comparesActual() {
		return nil, nil
	}
	return changedPaths(ctx, c.Latest.GitDir, c.Before, c.After, "--")
}

func (t *pilotTrial) assess(ctx context.Context) error {
	options, finding, root, result, candidate := t.options, t.finding, t.root, &t.result, t.candidate
	if candidate == "" {
		candidate = t.before
	}
	var err error
	call := agentCall{dir: candidate, prompt: assessmentPrompt(finding), prefix: filepath.Join(root, "assessment"), readDirs: []string{candidate}}
	result.Assessment, result.AssessmentCommands, err = assessmentPhase().run(ctx, options, call)
	if err != nil {
		return err
	}
	if err := saveJSON(filepath.Join(root, "assessment.json"), result.Assessment); err != nil {
		return err
	}
	if patch, err := sourcePatch(ctx, candidate, finding.Before); err != nil {
		return err
	} else if len(patch) != 0 {
		return fmt.Errorf("assessment modified source before the refactoring phase")
	}
	return nil
}

func (t *pilotTrial) refactor(ctx context.Context) error {
	if !t.finding.explores() {
		return nil
	}
	for number := 1; number <= t.options.Attempts; number++ {
		prefix := filepath.Join(t.root, "attempt-"+strconv.Itoa(number))
		refactoring, err := t.attempt(ctx, number, prefix)
		if err != nil {
			return err
		}
		t.result.Attempts = append(t.result.Attempts, refactoring)
		if err := saveJSON(prefix+".json", refactoring); err != nil {
			return err
		}
		if refactoring.succeeded() {
			break
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
	return nil
}

func (t *pilotTrial) attempt(ctx context.Context, number int, prefix string) (RefactoringAttempt, error) {
	refactoring := RefactoringAttempt{Number: number, Agent: invokeAgent(ctx, t.options, t.refactoringCall(prefix))}
	if err := t.evaluateCandidate(ctx, &refactoring, prefix); err != nil {
		return refactoring, err
	}
	return refactoring, refactoring.recordPatch(ctx, t.candidate, t.finding.Before, prefix)
}

func (t *pilotTrial) evaluateCandidate(ctx context.Context, attempt *RefactoringAttempt, prefix string) error {
	own, err := t.collectCandidatePaths(ctx)
	if err != nil {
		return err
	}
	attempt.Gofmt, attempt.Unformatted = t.options.unformattedFiles(ctx, t.candidate, prefix, own)
	attempt.TestScope, attempt.Checks = t.options.validateRevision(ctx, t.candidate, prefix, t.result.ChangedPaths)
	attempt.Columbo, attempt.Target = t.analyzeWithTarget(ctx, t.candidate, prefix+"-columbo", t.candidateRenames)
	return nil
}

func (t *pilotTrial) refactoringCall(prefix string) agentCall {
	return agentCall{phase: "refactor", dir: t.candidate, prompt: refactoringPrompt(t.finding, t.result.Assessment, t.result.Attempts), prefix: prefix + "-agent"}
}

func (t *pilotTrial) collectCandidatePaths(ctx context.Context) ([]string, error) {
	own, err := candidatePaths(ctx, t.candidate, t.finding.Before)
	if err == nil {
		t.result.ChangedPaths = unionPaths(own, t.actualPaths)
	}
	return own, err
}

func (a *RefactoringAttempt) recordPatch(ctx context.Context, dir, baseline, prefix string) error {
	patch, err := sourcePatch(ctx, dir, baseline)
	if err != nil {
		return err
	}
	a.HasChanges, a.Patch = len(patch) != 0, prefix+".patch"
	a.PolicyChanges, a.TestChanges = inspectPatch(string(patch))
	return writePrivate(a.Patch, patch)
}

func (a RefactoringAttempt) verified() bool {
	return commandPassed(a.Agent) && checksPassed(a.Checks) && a.HasChanges && len(a.PolicyChanges) == 0
}

func (a RefactoringAttempt) succeeded() bool {
	return a.verified() && analyzerRan(a.Columbo)
}

func (t *pilotTrial) collectActual(ctx context.Context) error {
	if err := saveJSON(filepath.Join(t.root, "discussion.json"), t.finding); err != nil {
		return err
	}
	if t.finding.comparesActual() {
		if err := t.checkoutActual(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (t *pilotTrial) validateBaseline(ctx context.Context) error {
	t.result.BaselineTestScope, t.result.BaselineChecks = t.options.validateRevision(ctx, t.before, filepath.Join(t.root, "baseline"), t.result.ChangedPaths)
	t.result.recordBehaviorChecks()
	return nil
}

func (r *TrialResult) recordBehaviorChecks() {
	r.ActualBehaviorChecksPassed = checksPassed(r.BaselineChecks) && checksPassed(r.ActualChecks)
	if len(r.Attempts) > 0 {
		r.BehaviorChecksPassed = checksPassed(r.BaselineChecks) && r.Attempts[len(r.Attempts)-1].verified()
	}
}

func (t *pilotTrial) checkoutActual(ctx context.Context) error {
	finding, result := t.finding, &t.result
	t.actual = filepath.Join(t.root, "actual")
	if err := checkoutRevision(ctx, finding.Latest.GitDir, finding.After, t.actual); err != nil {
		return err
	}
	result.ActualAncestry = finding.actualAncestry(ctx)
	t.validateActual(ctx)
	if patch, err := finding.actualPatch(ctx); err == nil {
		return writePrivate(filepath.Join(t.root, "actual.patch"), []byte(patch))
	}
	return nil
}

func (t *pilotTrial) validateActual(ctx context.Context) {
	columbo, target := t.analyzeWithTarget(ctx, t.actual, filepath.Join(t.root, "actual-columbo"), t.actualRenames)
	t.result.ActualColumbo, t.result.ActualTarget = &columbo, target
	scope, checks := t.options.validateRevision(ctx, t.actual, filepath.Join(t.root, "actual"), t.result.ChangedPaths)
	t.result.ActualTestScope, t.result.ActualChecks = &scope, checks
}

func (t *pilotTrial) review(ctx context.Context) error {
	reviewDir := cmp.Or(t.candidate, t.before)
	call := agentCall{dir: reviewDir, prompt: reviewPrompt(t.finding, t.result, t.before, t.candidate, t.actual), prefix: filepath.Join(t.root, "review"), readDirs: nonempty(t.before, t.candidate, t.actual)}
	var err error
	t.result.Review, t.result.ReviewCommands, err = reviewPhase(t.finding.explores()).run(ctx, t.options, call)
	if err != nil {
		return err
	}
	if err := t.checkReviewLeftSource(ctx, reviewDir); err != nil {
		return err
	}
	t.qualifyCandidate()
	t.qualifyActual()
	t.result.Complete = true
	return nil
}

func (t *pilotTrial) checkReviewLeftSource(ctx context.Context, reviewDir string) error {
	patch, err := sourcePatch(ctx, reviewDir, t.finding.Before)
	if err != nil {
		return err
	}
	var saved []byte
	if attempts := t.result.Attempts; len(attempts) > 0 {
		if saved, err = os.ReadFile(attempts[len(attempts)-1].Patch); err != nil {
			return err
		}
	}
	if string(patch) != string(saved) {
		return fmt.Errorf("review changed source after validation")
	}
	return nil
}

func (t *pilotTrial) qualifyCandidate() {
	review := &t.result.Review
	if !t.finding.explores() {
		review.Candidate = Judgment{Outcome: "not-applicable", Reason: "Exploratory refactoring is reserved for thumbs-down findings.", Evidence: []string{"reaction group: " + t.finding.Reaction}}
		return
	}
	if t.result.BehaviorChecksPassed && analyzerRan(t.result.BaselineColumbo) && analyzerRan(t.result.Attempts[len(t.result.Attempts)-1].Columbo) {
		return
	}
	review.Limitations = append(review.Limitations, "Runner could not verify a changed candidate against a passing baseline with unchanged policy and completed analyzer runs.")
	if review.Candidate.Outcome == "beneficial" {
		review.Candidate.Outcome = "inconclusive"
	}
}

func (t *pilotTrial) qualifyActual() {
	review, change := &t.result.Review, t.finding.ActualChange
	switch {
	case change.Status == actualNoResponse:
		review.Actual = change.judgment()
		return
	case change.Status == actualChanged && checksPassed(t.result.BaselineChecks) && checksPassed(t.result.ActualChecks):
		return
	}
	review.Limitations = append(review.Limitations, "Team outcome lacks an available revision or comparable passing behavior checks.")
	if change.Status == actualUnavailable {
		review.Actual = change.judgment()
	} else if review.Actual.Outcome == "beneficial" {
		review.Actual.Outcome = "inconclusive"
	}
}

func nonempty(values ...string) []string {
	return slices.DeleteFunc(values, func(value string) bool { return value == "" })
}

func checkModule(root, module string) error {
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	path, err := filepath.EvalSymlinks(filepath.Join(root, module))
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(root, path)
	if err != nil || escapes(rel) {
		return fmt.Errorf("module escapes trial checkout")
	}
	return nil
}

func (o PilotOptions) validateRevision(ctx context.Context, dir, prefix string, changed []string) (TestScope, []CommandResult) {
	scope := affectedScope(ctx, o, dir, changed)
	if err := checkModule(dir, o.Module); err != nil {
		return scope, []CommandResult{{Exit: -1, Error: err.Error()}}
	}
	return scope, o.runChecks(ctx, dir, prefix, scope)
}

func (o PilotOptions) runChecks(ctx context.Context, dir, prefix string, scope TestScope) []CommandResult {
	var results []CommandResult
	for i, check := range o.checkCommands(scope) {
		results = append(results, runTrialCommand(ctx, filepath.Join(dir, o.Module), []string{"/bin/sh", "-c", check}, "", prefix+"-check-"+strconv.Itoa(i+1), o.CheckTimeout, o.PassEnv...))
	}
	return results
}

func analyzeRevision(ctx context.Context, options PilotOptions, dir, prefix string) CommandResult {
	if err := checkModule(dir, options.Module); err != nil {
		return CommandResult{Exit: -1, Error: err.Error()}
	}
	args := []string{options.Columbo, "--staged=false", "--output", prefix + ".sqlite"}
	if options.Config != "" {
		content, err := os.ReadFile(options.Config)
		if err != nil {
			return CommandResult{Exit: -1, Error: err.Error()}
		}
		digest := sha256.Sum256(content)
		if hex.EncodeToString(digest[:]) != options.ConfigSHA256 {
			return CommandResult{Exit: -1, Error: "frozen analyzer configuration changed"}
		}
		config, err := os.CreateTemp(filepath.Join(dir, options.Module), ".columbo-pilot-*.yml")
		if err != nil {
			return CommandResult{Exit: -1, Error: err.Error()}
		}
		defer os.Remove(config.Name())
		if _, err := config.Write(content); err != nil {
			config.Close()
			return CommandResult{Exit: -1, Error: err.Error()}
		}
		if err := config.Close(); err != nil {
			return CommandResult{Exit: -1, Error: err.Error()}
		}
		args = append(args, "--config", config.Name())
	}
	args = append(args, "./...")
	result := runTrialCommand(ctx, filepath.Join(dir, options.Module), args, "", prefix, options.CheckTimeout, options.PassEnv...)
	if result.TimedOut || (result.Exit != 0 && result.Exit != 1) {
		return result
	}
	snapshot, err := snapshotdb.Open(prefix + ".sqlite")
	if err != nil {
		result.Error = "invalid analysis evidence: " + err.Error()
		return result
	}
	defer snapshot.Close()
	cases, err := loadAnalyzedCases(ctx, snapshot.Queries())
	if err == nil {
		err = saveJSON(casesPath(prefix), cases)
	}
	if err != nil {
		result.Error = "analysis evidence: " + err.Error()
		return result
	}
	result.EvidenceValid = true
	return result
}

func (t *pilotTrial) freezeConfig() error {
	var content []byte
	origin := "starting checkout"
	path := t.options.Config
	if path == "" {
		configs := map[string]bool{}
		for _, artifact := range t.finding.Initial.Artifacts {
			var p struct {
				AnalyzedCommit string   `json:"analyzed_commit"`
				ConfigText     *string  `json:"config_text"`
				Clean          bool     `json:"clean"`
				Module         string   `json:"module"`
				Cases          []string `json:"cases"`
			}
			if json.Unmarshal(artifact.Provenance, &p) == nil && p.Clean && filepath.Clean(p.Module) == filepath.Clean(t.options.Module) && contains(p.Cases, t.finding.CaseID) && p.AnalyzedCommit == t.finding.Before && p.ConfigText != nil {
				configs[*p.ConfigText] = true
				content, origin = []byte(*p.ConfigText), "captured CI configuration"
			}
		}
		if len(configs) > 1 {
			return fmt.Errorf("multiple captured CI configurations match this revision; choose --config within the module")
		}
		path = ".columbo.yml"
	}
	if content == nil {
		var err error
		content, err = os.ReadFile(filepath.Join(t.before, t.options.Module, path))
		if err != nil && !(os.IsNotExist(err) && t.options.Config == "") {
			return err
		}
		if os.IsNotExist(err) {
			content, origin = []byte("{}\n"), "default configuration"
		}
	}
	t.options.Config = filepath.Join(t.root, "analysis-config.yml")
	digest := sha256.Sum256(content)
	t.options.ConfigSHA256 = hex.EncodeToString(digest[:])
	t.result.AnalysisConfig, t.result.AnalysisConfigSHA256, t.result.AnalysisConfigOrigin = t.options.Config, t.options.ConfigSHA256, origin
	return writePrivate(t.options.Config, content)
}

func commandPassed(result CommandResult) bool {
	return result.Exit == 0 && result.Error == "" && !result.TimedOut
}
func analyzerRan(result CommandResult) bool {
	return result.EvidenceValid && !result.TimedOut && (result.Exit == 0 || result.Exit == 1)
}
func checksPassed(results []CommandResult) bool {
	if len(results) == 0 {
		return false
	}
	for _, result := range results {
		if !commandPassed(result) {
			return false
		}
	}
	return true
}

func sourcePatch(ctx context.Context, dir, baseline string) ([]byte, error) {
	if err := intendToAddUntracked(ctx, dir); err != nil {
		return nil, err
	}
	patch, err := git(ctx, dir, "diff", "--binary", "--no-ext-diff", "--no-textconv", baseline, "--", ".")
	return []byte(patch), err
}

func inspectPatch(patch string) (policy, tests []string) {
	file := ""
	for _, line := range strings.Split(patch, "\n") {
		if strings.HasPrefix(line, "+++ b/") {
			file = strings.TrimPrefix(line, "+++ b/")
		}
		if strings.HasPrefix(line, "--- a/") {
			file = strings.TrimPrefix(line, "--- a/")
		}
		if strings.HasPrefix(line, "@@") {
			if filepath.Base(file) == ".columbo.yml" && !contains(policy, file) {
				policy = append(policy, file)
			}
			if strings.HasSuffix(file, "_test.go") && !contains(tests, file) {
				tests = append(tests, file)
			}
		}
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") && strings.Contains(line, "columbo:ignore") {
			policy = append(policy, "added suppression in "+file)
		}
	}
	return
}
