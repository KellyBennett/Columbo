package feedback

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/KellyBennett/Columbo/internal/columbo"
	"github.com/KellyBennett/Columbo/internal/feedbackdb"
	"github.com/stretchr/testify/require"
)

func TestPilotBalancesReactionsAndRequiresExplicitReconstruction(t *testing.T) {
	var observations []Observation
	for i, group := range []string{"thumbs-up", "thumbs-up", "thumbs-down", "unreacted"} {
		sha := strings.Repeat(string(rune('a'+i)), 40)
		comment := Comment{ID: int64(i + 1), Kind: ReviewComment, CommitID: &sha, Marker: &Marker{CaseID: group + string(rune('a'+i)), Role: "finding", Rule: "feature-envy"}}
		o := Observation{GitDir: "retained.git", Record: PullRequestRecord{PullRequest: PullRequest{Number: int64(i + 1)}, Comments: []Comment{comment}}, Revisions: []Revision{{SHA: sha, Present: true, Roles: []string{"pr-head"}}}}
		if group != "unreacted" {
			content := "+1"
			if group == "thumbs-down" {
				content = "-1"
			}
			o.Record.Reactions = []Reaction{{CommentKind: ReviewComment, CommentID: comment.ID, Content: content}}
		}
		observations = append(observations, o)
	}
	options := PilotOptions{Limit: 3, Seed: "repeatable"}
	plan := makePilotPlan(context.Background(), options, observations)
	require.Empty(t, plan.Cases)
	require.Len(t, plan.Excluded, 4)
	options.AllowReconstructed = true
	plan = makePilotPlan(context.Background(), options, observations)
	require.Len(t, plan.Cases, 3)
	require.Equal(t, "thumbs-down", plan.Cases[0].Reaction)
	require.Equal(t, "thumbs-up", plan.Cases[1].Reaction)
	require.Equal(t, "unreacted", plan.Cases[2].Reaction)
	require.Equal(t, plan, makePilotPlan(context.Background(), options, observations))
}

func shellQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'" }

func executable(t *testing.T, path, script string) string {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte("#!/bin/sh\nset -eu\n"+script), 0o700))
	return path
}

func TestPilotRunsRejectedFindingRetriesTestsAndResumesWithoutCallingAgent(t *testing.T) {
	testPilotReaction(t, "thumbs-down", true)
}

func TestPilotEvaluatesActualChangesWithoutExploringOtherReactions(t *testing.T) {
	for _, group := range []string{"thumbs-up", "unreacted", "mixed", "other"} {
		t.Run(group, func(t *testing.T) { testPilotReaction(t, group, true) })
	}
}

func TestThumbsUpWithoutLaterCodeStaysInconclusiveWithoutExploring(t *testing.T) {
	testPilotReaction(t, "thumbs-up", false)
}

func testPilotReaction(t *testing.T, group string, hasLater bool) {
	source, before, after := newHistory(t)
	root := t.TempDir()
	client, err := newGitHubClient("https://api.github.com", "")
	require.NoError(t, err)
	c := collector{repository: "o/r", github: client, options: CollectOptions{Checkout: source, EvidencePath: filepath.Join(root, "evidence")}}
	flaggedPath := "f.go"
	comment := Comment{Path: &flaggedPath, Kind: ReviewComment, ID: 11, CommitID: &before, Body: "Investigate ownership", Marker: &Marker{CaseID: "C-1", Role: "finding", Rule: "feature-envy"}}
	o := Observation{ID: "observation", ObservedAt: "2026-01-01T00:00:00Z", Record: PullRequestRecord{PullRequest: PullRequest{Number: 1}, Comments: []Comment{comment}, Reactions: []Reaction{{CommentKind: ReviewComment, CommentID: 11, Content: "-1"}}}, Revisions: []Revision{{SHA: before, Roles: []string{"original-review-anchor"}}, {SHA: after, Roles: []string{"pr-head"}}}}
	if !hasLater {
		o.Revisions[1].Roles = []string{"pr-commit"}
	}
	o.Record.Reactions = nil
	for _, content := range map[string][]string{"thumbs-down": {"-1"}, "thumbs-up": {"+1"}, "mixed": {"+1", "-1"}, "other": {"heart"}}[group] {
		o.Record.Reactions = append(o.Record.Reactions, Reaction{CommentKind: ReviewComment, CommentID: 11, Content: content})
	}
	o.GitDir, err = c.retainHistory(context.Background(), &o)
	require.NoError(t, err)
	storePath := filepath.Join(root, "feedback.sqlite")
	store := openStore(t, storePath)
	require.NoError(t, store.Update(context.Background(), func(q feedbackdb.Querier) error {
		if err := q.UpsertRepository(context.Background(), feedbackdb.UpsertRepositoryParams{Name: "o/r", Seen: o.ObservedAt}); err != nil {
			return err
		}
		return q.InsertObservation(context.Background(), feedbackdb.InsertObservationParams{ID: o.ID, Repository: "o/r", Number: 1, ObservedAt: o.ObservedAt, Payload: o.payload()})
	}))
	self, err := os.Executable()
	require.NoError(t, err)
	analyzer := executable(t, filepath.Join(root, "analyzer"), "exec "+shellQuote(self)+" -test.run=TestPilotAnalyzerHelper -- \"$@\"\n")
	counter := filepath.Join(root, "first-attempt")
	trace := filepath.Join(root, "agent-trace")
	ready := filepath.Join(root, "agent-ready")
	candidateOutcome := "tradeoff"
	if group != "thumbs-down" {
		candidateOutcome = "not-applicable"
	}
	agent := executable(t, filepath.Join(root, "agent"), `
prompt=$(cat)
echo "$1" >> `+shellQuote(trace)+`
if [ "$1" = assess ] && [ ! -f `+shellQuote(ready)+` ]; then echo unavailable >&2; exit 7; fi
case "$1" in
assess)
 case "$prompt" in *thumbs-down*|*discussion.json*) exit 9;; esac
 echo 'Here is the assessment.'
 echo '`+"```json"+`'
 echo '{"relationship":"shared policy","opportunity":"one owner","future_change":"add an operation","change_basis":"hypothetical","costs":"extra call","evidence":["f.go:2"]}'
 echo '`+"```"+`'
 ;;
refactor)
 if [ ! -f `+shellQuote(counter)+` ]; then
   touch `+shellQuote(counter)+`
   printf 'BROKEN\n' > f.go
 else
   printf 'package p\nfunc Sum(a, b int) int { return add(a,b) }\nfunc add(a,b int) int { return a+b }\n' > f.go
 fi
 echo 'Attempted refactoring'
 ;;
review)
 echo '{"finding":{"outcome":"tradeoff","reason":"strict policy opportunity","evidence":["f.go:2"]},"direction":{"outcome":"tradeoff","reason":"extra abstraction","evidence":["f.go:3"]},"candidate":{"outcome":"`+candidateOutcome+`","reason":"moves ownership with a cost","evidence":["f.go:2-3"]},"actual":{"outcome":"no-demonstrated-benefit","reason":"same operation","evidence":["f.go:2"]},"limitations":["synthetic test"],"reported_model":"test"}'
 ;;
esac
`)
	options := PilotOptions{Repository: "o/r", Store: storePath, Out: filepath.Join(root, "pilot"), Agent: agent, AgentArgs: stringList{"{phase}"}, Columbo: analyzer, Limit: 1, Turns: 3, Attempts: 2, Timeout: 5 * time.Second, CheckTimeout: 30 * time.Second, AllowReconstructed: true, Module: "."}
	var out strings.Builder
	require.ErrorContains(t, Pilot(context.Background(), options, &out), "1 trials incomplete")
	require.NoError(t, os.WriteFile(ready, []byte("ready"), 0o600))
	require.NoError(t, Pilot(context.Background(), options, &out), out.String())
	var plan PilotPlan
	data, err := os.ReadFile(filepath.Join(options.Out, "plan.json"))
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, &plan))
	require.Equal(t, group, plan.Cases[0].Reaction)
	var result TrialResult
	data, err = os.ReadFile(filepath.Join(options.Out, "trials", plan.Cases[0].ID, "result.json"))
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, &result))
	executions, err := os.ReadDir(filepath.Join(options.Out, "trials", plan.Cases[0].ID, "executions"))
	require.NoError(t, err)
	require.Len(t, executions, 2, "failed invocations must remain available after retry")
	require.True(t, result.Complete)
	require.Equal(t, group == "thumbs-down", result.BehaviorChecksPassed)
	require.Equal(t, hasLater, result.ActualBehaviorChecksPassed)
	require.True(t, result.BaselineColumbo.EvidenceValid)
	require.Equal(t, "default configuration", result.AnalysisConfigOrigin)
	if hasLater || group == "thumbs-down" {
		require.Equal(t, []string{"f.go"}, result.ChangedPaths)
		require.Equal(t, TestScope{Packages: []string{"example.invalid/pilot"}, Reason: "affected packages"}, result.BaselineTestScope)
	} else {
		require.Equal(t, TestScope{Reason: "no changed files"}, result.BaselineTestScope)
	}
	if group == "thumbs-down" {
		require.Len(t, result.Attempts, 2)
		require.False(t, checksPassed(result.Attempts[0].Checks))
		require.True(t, checksPassed(result.Attempts[1].Checks))
		require.FileExists(t, result.Attempts[0].Patch)
		require.FileExists(t, result.Attempts[1].Patch)
	} else {
		require.Empty(t, result.Attempts)
		require.Equal(t, "actual-changes", result.AnalysisMode)
		require.Equal(t, "not-applicable", result.Review.Candidate.Outcome)
		require.NoDirExists(t, filepath.Join(result.Execution, "candidate"))
		require.NoFileExists(t, counter)
		report, err := os.ReadFile(filepath.Join(options.Out, "report.md"))
		require.NoError(t, err)
		if hasLater {
			require.Contains(t, string(report), "actual-changes | no-demonstrated-benefit | passed in 1 affected packages; coverage limits apply")
			require.Contains(t, string(report), "Tests covered in the later PR revision: example.invalid/pilot.")
		} else {
			require.Equal(t, "inconclusive", result.Review.Actual.Outcome)
			require.Contains(t, string(report), "actual-changes | inconclusive | not verified")
		}
	}
	status, err := git(context.Background(), source, "status", "--porcelain")
	require.NoError(t, err)
	require.Empty(t, status)
	traceBefore, err := os.ReadFile(trace)
	require.NoError(t, err)
	expected := "assess\nassess\nreview\n"
	if group == "thumbs-down" {
		expected = "assess\nassess\nrefactor\nrefactor\nreview\n"
	}
	require.Equal(t, expected, string(traceBefore))
	require.NoError(t, Pilot(context.Background(), options, &out), out.String())
	traceAfter, err := os.ReadFile(trace)
	require.NoError(t, err)
	require.Equal(t, traceBefore, traceAfter)
	options.Attempts = 3
	require.ErrorContains(t, Pilot(context.Background(), options, &out), "options or executable hashes changed")
}

func TestPilotAnalyzerHelper(t *testing.T) {
	for i, arg := range os.Args {
		if arg == "--" {
			dir, err := os.Getwd()
			if err != nil {
				os.Exit(2)
			}
			os.Exit(columbo.Run(os.Args[i+1:], columbo.Invocation{Dir: dir, Version: "test", Stdout: os.Stdout, Stderr: os.Stderr}))
		}
	}
}

func TestTrialTimeoutKillsChildAndRecordsFailure(t *testing.T) {
	dir := t.TempDir()
	result := runTrialCommand(context.Background(), dir, []string{"/bin/sh", "-c", "sleep 30 & wait"}, "", filepath.Join(dir, "timeout"), 100*time.Millisecond)
	require.True(t, result.TimedOut)
	require.Less(t, result.Seconds, 3.0)
	require.NotEqual(t, 0, result.Exit)
}

func TestPolicyChangesAreReportedAndTestEditsRemainVisible(t *testing.T) {
	policy, tests := inspectPatch("--- a/.columbo.yml\n+++ b/.columbo.yml\n@@ -1 +1 @@\n-limit: 7\n+limit: 10\n--- a/f_test.go\n+++ b/f_test.go\n@@ -1 +1 @@\n-old assertion\n+new assertion\n--- a/f.go\n+++ b/f.go\n@@ -1 +1 @@\n+// columbo:ignore feature-envy -- bypass\n")
	require.Equal(t, []string{".columbo.yml", "added suppression in f.go"}, policy)
	require.Equal(t, []string{"f_test.go"}, tests)
}

func TestFrozenConfigurationIsIndependentOfLaterChangesAndCannotBeOverwritten(t *testing.T) {
	before := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(before, ".columbo.yml"), []byte("version: 1\n"), 0o600))
	trial := pilotTrial{options: PilotOptions{Module: "."}, before: before, root: t.TempDir()}
	require.NoError(t, trial.freezeConfig())
	require.Equal(t, "starting checkout", trial.result.AnalysisConfigOrigin)
	require.NoError(t, os.WriteFile(filepath.Join(before, ".columbo.yml"), []byte("changed"), 0o600))
	data, err := os.ReadFile(trial.options.Config)
	require.NoError(t, err)
	require.Equal(t, "version: 1\n", string(data))
	require.NoError(t, os.WriteFile(trial.options.Config, []byte("tampered"), 0o600))
	result := analyzeRevision(context.Background(), trial.options, before, filepath.Join(trial.root, "analysis"))
	require.Equal(t, "frozen analyzer configuration changed", result.Error)
	require.False(t, analyzerRan(result))
}

func TestInitialAssessmentWithholdsReactionsAndClearedState(t *testing.T) {
	finding := PilotCase{Reaction: "thumbs-down", Finding: Comment{Body: "<!-- columbo-feedback case=C-1 role=finding state=cleared -->\nInvestigate ownership"}}
	prompt := assessmentPrompt(finding)
	require.NotContains(t, prompt, "thumbs-down")
	require.NotContains(t, prompt, "state=cleared")
	require.Contains(t, prompt, "Investigate ownership")
}

func TestAmbiguousCIConfigurationsRequireAnExplicitChoice(t *testing.T) {
	before := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(before, ".columbo.yml"), []byte("version: 1\n"), 0o600))
	trial := pilotTrial{options: PilotOptions{Module: "."}, before: before, root: t.TempDir(), finding: PilotCase{Before: "commit", CaseID: "C-1"}}
	for _, content := range []string{"version: 1\n", "{}\n"} {
		data, err := json.Marshal(map[string]any{"clean": true, "module": ".", "cases": []string{"C-1"}, "analyzed_commit": "commit", "config_text": content})
		require.NoError(t, err)
		trial.finding.Initial.Artifacts = append(trial.finding.Initial.Artifacts, CIArtifact{Provenance: data})
	}
	require.ErrorContains(t, trial.freezeConfig(), "multiple captured CI configurations")
	trial.options.Config = ".columbo.yml"
	require.NoError(t, trial.freezeConfig())
	require.Equal(t, "starting checkout", trial.result.AnalysisConfigOrigin)
}

func TestLegacyPilotPlanRequiresNewOutputDirectory(t *testing.T) {
	options := PilotOptions{Out: t.TempDir()}
	require.NoError(t, saveJSON(filepath.Join(options.Out, "plan.json"), PilotPlan{Version: 1, Options: options}))
	_, err := loadOrPlan(context.Background(), options)
	require.ErrorContains(t, err, "pilot analysis policy changed; choose a new --out directory")
}
