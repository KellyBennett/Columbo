package feedback

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
	Columbo       CommandResult   `json:"columbo"`
	Patch         string          `json:"patch"`
	PolicyChanges []string        `json:"policy_changes,omitempty"`
	TestChanges   []string        `json:"test_changes,omitempty"`
	HasChanges    bool            `json:"has_changes"`
}

type TrialResult struct {
	AnalysisMode               string               `json:"analysis_mode"`
	ActualBehaviorChecksPassed bool                 `json:"actual_behavior_checks_passed"`
	Version                    int                  `json:"version"`
	Complete                   bool                 `json:"complete"`
	CaseID                     string               `json:"case_id"`
	Execution                  string               `json:"execution"`
	Before                     string               `json:"before"`
	After                      string               `json:"after,omitempty"`
	RevisionEvidence           string               `json:"revision_evidence"`
	BaselineChecks             []CommandResult      `json:"baseline_checks"`
	BaselineColumbo            CommandResult        `json:"baseline_columbo"`
	AssessmentCommand          CommandResult        `json:"assessment_command"`
	Assessment                 DesignAssessment     `json:"assessment"`
	Attempts                   []RefactoringAttempt `json:"attempts"`
	ActualChecks               []CommandResult      `json:"actual_checks,omitempty"`
	ActualColumbo              *CommandResult       `json:"actual_columbo,omitempty"`
	ActualAncestry             string               `json:"actual_ancestry"`
	ReviewCommand              CommandResult        `json:"review_command"`
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
}

func runPilotCase(ctx context.Context, options PilotOptions, finding PilotCase) (err error) {
	resultPath := filepath.Join(options.Out, "trials", finding.ID, "result.json")
	if data, readErr := os.ReadFile(resultPath); readErr == nil {
		var previous TrialResult
		if err := json.Unmarshal(data, &previous); err != nil {
			return err
		}
		if previous.Complete {
			return nil
		}
	} else if !os.IsNotExist(readErr) {
		return readErr
	}
	root := filepath.Join(options.Out, "trials", finding.ID, "executions", rand.Text())
	t := &pilotTrial{options: options, finding: finding, root: root, before: filepath.Join(root, "before"), candidate: filepath.Join(root, "candidate")}
	t.result = TrialResult{Version: 2, AnalysisMode: finding.analysisMode(), CaseID: finding.CaseID, Execution: root, Before: finding.Before, After: finding.After, RevisionEvidence: finding.RevisionEvidence, ActualAncestry: "unavailable"}
	defer func() {
		if err != nil {
			t.result.Error = err.Error()
		}
		if saveErr := saveJSON(filepath.Join(root, "result.json"), t.result); err == nil {
			err = saveErr
		}
		if saveErr := saveJSON(resultPath, t.result); err == nil {
			err = saveErr
		}
	}()
	for _, step := range []func(context.Context) error{t.prepare, t.assess, t.refactor, t.collectActual, t.review} {
		if err := step(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (t *pilotTrial) prepare(ctx context.Context) error {
	options, finding, root, result := t.options, t.finding, t.root, &t.result
	before, candidate := t.before, t.candidate
	dirs := []string{before}
	if finding.explores() {
		dirs = append(dirs, candidate)
	} else {
		t.candidate = ""
	}
	for _, dir := range dirs {
		if err := checkoutRevision(ctx, finding.Initial.GitDir, finding.Before, dir); err != nil {
			return err
		}
	}
	if err := checkModule(before, options.Module); err != nil {
		return err
	}
	if err := t.freezeConfig(); err != nil {
		return err
	}
	options = t.options
	result.BaselineColumbo = analyzeRevision(ctx, options, before, filepath.Join(root, "baseline-columbo"))
	result.BaselineChecks = validateRevision(ctx, options, before, filepath.Join(root, "baseline"))
	return nil
}

func (t *pilotTrial) assess(ctx context.Context) error {
	options, finding, root, result, candidate := t.options, t.finding, t.root, &t.result, t.candidate
	if candidate == "" {
		candidate = t.before
	}
	prompt := assessmentPrompt(finding)
	result.AssessmentCommand = invokeAgent(ctx, options, candidate, "assess", prompt, filepath.Join(root, "assessment"))
	if !commandPassed(result.AssessmentCommand) {
		return fmt.Errorf("initial assessment failed; see %s", result.AssessmentCommand.Stderr)
	}
	if err := readAgentJSON(result.AssessmentCommand.Stdout, &result.Assessment); err != nil {
		return fmt.Errorf("assessment must return JSON: %w", err)
	}
	if result.Assessment.Relationship == "" || result.Assessment.FutureChange == "" || result.Assessment.ChangeBasis != "hypothetical" || len(result.Assessment.Evidence) == 0 {
		return fmt.Errorf("assessment lacks a relationship, evidence or a prospective hypothetical change")
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
	options, finding, root, result, candidate := t.options, t.finding, t.root, &t.result, t.candidate
	for attempt := 1; attempt <= options.Attempts; attempt++ {
		prefix := filepath.Join(root, "attempt-"+strconv.Itoa(attempt))
		refactoring := RefactoringAttempt{Number: attempt}
		refactoring.Agent = invokeAgent(ctx, options, candidate, "refactor", refactoringPrompt(finding, result.Assessment, result.Attempts), prefix+"-agent")
		refactoring.Checks = validateRevision(ctx, options, candidate, prefix)
		refactoring.Columbo = analyzeRevision(ctx, options, candidate, prefix+"-columbo")
		patch, err := sourcePatch(ctx, candidate, finding.Before)
		if err != nil {
			return err
		}
		refactoring.HasChanges, refactoring.Patch = len(patch) != 0, prefix+".patch"
		if err := writePrivate(refactoring.Patch, patch); err != nil {
			return err
		}
		refactoring.PolicyChanges, refactoring.TestChanges = inspectPatch(string(patch))
		result.Attempts = append(result.Attempts, refactoring)
		if err := saveJSON(prefix+".json", refactoring); err != nil {
			return err
		}
		if commandPassed(refactoring.Agent) && checksPassed(refactoring.Checks) && analyzerRan(refactoring.Columbo) && refactoring.HasChanges && len(refactoring.PolicyChanges) == 0 {
			break
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
	last := result.Attempts[len(result.Attempts)-1]
	result.BehaviorChecksPassed = checksPassed(result.BaselineChecks) && checksPassed(last.Checks) && commandPassed(last.Agent) && last.HasChanges && len(last.PolicyChanges) == 0
	return nil
}

func (t *pilotTrial) collectActual(ctx context.Context) error {
	options, finding, root, result := t.options, t.finding, t.root, &t.result
	if err := saveJSON(filepath.Join(root, "discussion.json"), finding); err != nil {
		return err
	}
	actual := ""
	if finding.After != "" {
		actual = filepath.Join(root, "actual")
		if err := checkoutRevision(ctx, finding.Latest.GitDir, finding.After, actual); err != nil {
			return err
		}
		result.ActualAncestry = "not established; revisions may have been rewritten"
		if _, err := git(ctx, finding.Initial.GitDir, "merge-base", "--is-ancestor", finding.Before, finding.After); err == nil {
			result.ActualAncestry = "later PR head descends from starting revision"
		}
		result.ActualColumbo = new(CommandResult)
		*result.ActualColumbo = analyzeRevision(ctx, options, actual, filepath.Join(root, "actual-columbo"))
		result.ActualChecks = validateRevision(ctx, options, actual, filepath.Join(root, "actual"))
		patch, err := git(ctx, finding.Latest.GitDir, "diff", "--binary", "--no-ext-diff", "--no-textconv", finding.Before, finding.After)
		if err == nil {
			if err := writePrivate(filepath.Join(root, "actual.patch"), []byte(patch)); err != nil {
				return err
			}
		}
	}
	t.actual = actual
	result.ActualBehaviorChecksPassed = checksPassed(result.BaselineChecks) && checksPassed(result.ActualChecks)
	return nil
}

func (t *pilotTrial) review(ctx context.Context) error {
	options, finding, root, result := t.options, t.finding, t.root, &t.result
	before, candidate, actual := t.before, t.candidate, t.actual
	prompt := reviewPrompt(finding, *result, before, candidate, actual)
	reviewDir := candidate
	if reviewDir == "" {
		reviewDir = before
	}
	result.ReviewCommand = invokeAgent(ctx, options, reviewDir, "review", prompt, filepath.Join(root, "review"))
	if !commandPassed(result.ReviewCommand) {
		return fmt.Errorf("design review failed; see %s", result.ReviewCommand.Stderr)
	}
	if err := readAgentJSON(result.ReviewCommand.Stdout, &result.Review); err != nil {
		return fmt.Errorf("design review must return JSON: %w", err)
	}
	if patch, err := sourcePatch(ctx, reviewDir, finding.Before); err != nil {
		return err
	} else {
		var saved []byte
		if len(result.Attempts) > 0 {
			saved, err = os.ReadFile(result.Attempts[len(result.Attempts)-1].Patch)
			if err != nil {
				return err
			}
		}
		if string(patch) != string(saved) {
			return fmt.Errorf("review changed source after validation")
		}
	}
	if !finding.explores() {
		result.Review.Candidate = Judgment{Outcome: "not-applicable", Reason: "Exploratory refactoring is reserved for thumbs-down findings.", Evidence: []string{"reaction group: " + finding.Reaction}}
	}
	for name, judgment := range map[string]Judgment{"finding": result.Review.Finding, "direction": result.Review.Direction, "candidate": result.Review.Candidate, "actual": result.Review.Actual} {
		if name == "candidate" && !finding.explores() {
			continue
		}
		if !contains([]string{"beneficial", "tradeoff", "no-demonstrated-benefit", "inconclusive"}, judgment.Outcome) || judgment.Reason == "" || len(judgment.Evidence) == 0 {
			return fmt.Errorf("review has an invalid %s judgment", name)
		}
	}
	if finding.explores() && (!result.BehaviorChecksPassed || !analyzerRan(result.BaselineColumbo) || !analyzerRan(result.Attempts[len(result.Attempts)-1].Columbo)) {
		result.Review.Limitations = append(result.Review.Limitations, "Runner could not verify a changed candidate against a passing baseline with unchanged policy and completed analyzer runs.")
		if result.Review.Candidate.Outcome == "beneficial" {
			result.Review.Candidate.Outcome = "inconclusive"
		}
	}
	if actual == "" || !checksPassed(result.BaselineChecks) || !checksPassed(result.ActualChecks) {
		result.Review.Limitations = append(result.Review.Limitations, "Team outcome lacks an available revision or comparable passing behavior checks.")
		if actual == "" {
			result.Review.Actual = Judgment{Outcome: "inconclusive", Reason: "No later PR revision is available to evaluate the team's changes.", Evidence: []string{"later PR head unavailable"}}
		} else if result.Review.Actual.Outcome == "beneficial" {
			result.Review.Actual.Outcome = "inconclusive"
		}
	}
	result.Complete = true
	return nil
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
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("module escapes trial checkout")
	}
	return nil
}

func validateRevision(ctx context.Context, options PilotOptions, dir, prefix string) []CommandResult {
	if err := checkModule(dir, options.Module); err != nil {
		return []CommandResult{{Exit: -1, Error: err.Error()}}
	}
	var results []CommandResult
	for i, check := range options.Checks {
		results = append(results, runTrialCommand(ctx, filepath.Join(dir, options.Module), []string{"/bin/sh", "-c", check}, "", prefix+"-check-"+strconv.Itoa(i+1), options.CheckTimeout, options.PassEnv...))
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
	cases, err := snapshot.Queries().SummaryCases(ctx)
	if err == nil {
		err = saveJSON(prefix+".cases.json", cases)
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

func invokeAgent(ctx context.Context, options PilotOptions, dir, phase, prompt, prefix string) CommandResult {
	args := append([]string{options.Agent}, options.AgentArgs...)
	if len(options.AgentArgs) == 0 {
		tools, allowed := "Read,Glob,Grep,Bash", "Read,Glob,Grep,Bash(git show:*),Bash(git log:*),Bash(git diff:*)"
		if phase == "refactor" {
			tools = "Read,Edit,Write,Glob,Grep,Bash"
			allowed = "Read,Edit,Write,Glob,Grep,Bash(go build:*),Bash(go vet:*),Bash(go test:*),Bash(columbo:*)"
		}
		args = append(args, "-p", "--safe-mode", "--restricted", "--no-session-persistence", "--permission-mode", "dontAsk", "--max-turns", strconv.Itoa(options.Turns), "--output-format", "text", "--tools", tools, "--allowedTools", allowed)
		if options.Model != "" {
			args = append(args, "--model", options.Model)
		}
		if phase != "assess" {
			args = append(args, "--add-dir", filepath.Dir(dir))
		}
	} else {
		for i := 1; i < len(args); i++ {
			args[i] = strings.NewReplacer("{model}", options.Model, "{turns}", strconv.Itoa(options.Turns), "{phase}", phase).Replace(args[i])
		}
	}
	if err := writePrivate(prefix+".prompt", []byte(prompt)); err != nil {
		return CommandResult{Exit: -1, Error: err.Error()}
	}
	return runTrialCommand(ctx, dir, args, prompt, prefix, options.Timeout, options.PassEnv...)
}

func readAgentJSON(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	text := strings.TrimSpace(string(data))
	if strings.HasPrefix(text, "```json\n") && strings.HasSuffix(text, "```") {
		text = strings.TrimSuffix(strings.TrimPrefix(text, "```json\n"), "```")
	}
	return json.Unmarshal([]byte(text), target)
}

func sourcePatch(ctx context.Context, dir, baseline string) ([]byte, error) {
	if _, err := git(ctx, dir, "add", "--intent-to-add", "--", "."); err != nil {
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
