package feedback

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAgentJSONAcceptsPreamble(t *testing.T) {
	path := filepath.Join(t.TempDir(), "assessment.stdout")
	require.NoError(t, os.WriteFile(path, []byte("Here is the assessment.\n\n```json\n{\"relationship\":\"shared policy\",\"future_change\":\"add an operation\",\"change_basis\":\"hypothetical\",\"evidence\":[\"f.go:2\"]}\n```\n"), 0600))
	var assessment DesignAssessment
	require.NoError(t, readAgentJSON(path, &assessment))
	require.Equal(t, "shared policy", assessment.Relationship)
}

func TestPilotUnchangedActualChangesNeedsNoAgent(t *testing.T) {
	source, before, _ := newHistory(t)
	options := PilotOptions{Out: t.TempDir(), Module: ".", Agent: "/bin/false"}
	finding := PilotCase{ID: "unchanged", CaseID: "C-unchanged", Reaction: "thumbs-up", Before: before, After: before, Initial: Observation{GitDir: source}, Latest: Observation{GitDir: source}}
	require.NoError(t, runPilotCase(context.Background(), options, finding))
	data, err := os.ReadFile(filepath.Join(options.Out, "trials", finding.ID, "result.json"))
	require.NoError(t, err)
	var result TrialResult
	require.NoError(t, json.Unmarshal(data, &result))
	require.True(t, result.Complete)
	require.Equal(t, "no-response", result.Review.Actual.Outcome)
	require.Empty(t, result.AssessmentCommand.Args)
	require.Empty(t, result.ReviewCommand.Args)
}

func TestAgentJSONExtractsLastCompleteAnswerAndStructuredOutput(t *testing.T) {
	for _, test := range []struct {
		name, reply string
		fail        bool
	}{
		{"plain", `{"relationship":"final"}`, false},
		{"prose object", `I checked it. {"relationship":"final"} Done.`, false},
		{"last fence", "```json\n{\"relationship\":\"example\"}\n```\nAnswer:\n```json\n{\"relationship\":\"final\"}\n```", false},
		{"unclosed trailing fence", "```json\n{\"relationship\":\"final\"}\n```\n```json\nunfinished", false},
		{"unclosed fence object fallback", "Here is the answer. ```json\n{\"relationship\":\"final\"}", false},
		{"envelope", `{"type":"result","subtype":"success","is_error":false,"structured_output":{"relationship":"final"}}`, false},
		{"error envelope", `{"type":"result","subtype":"error_max_turns","is_error":true,"structured_output":{"relationship":"final"}}`, true},
		{"missing structured output", `{"type":"result","subtype":"success","result":"text"}`, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "reply")
			require.NoError(t, os.WriteFile(path, []byte(test.reply), 0600))
			var value DesignAssessment
			err := readAgentJSON(path, &value)
			if test.fail {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, "final", value.Relationship)
			}
		})
	}
}

func TestJSONPhaseRetriesInvalidRepliesAndRetainsBothAttempts(t *testing.T) {
	valid := `{"relationship":"policy","future_change":"add operation","change_basis":"hypothetical","evidence":["f.go:2"]}`
	for _, test := range []struct {
		name, first, second string
		fail                bool
	}{
		{"parse failure", "not JSON", valid, false},
		{"validation failure", `{"relationship":" ","future_change":"add","change_basis":"hypothetical","evidence":["f.go:2"]}`, valid, false},
		{"empty evidence", `{"relationship":"policy","future_change":"add","change_basis":"hypothetical","evidence":[" "]}`, valid, false},
		{"both malformed", "broken", "still broken", true},
		{"no inherited fields", `{"relationship":"policy","change_basis":"hypothetical","evidence":["f.go:2"]}`, `{"future_change":"add","change_basis":"hypothetical","evidence":["f.go:2"]}`, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			marker := filepath.Join(dir, "first")
			agent := executable(t, filepath.Join(dir, "agent"), "cat >/dev/null\nif [ ! -f "+shellQuote(marker)+" ]; then touch "+shellQuote(marker)+"; printf '%s' "+shellQuote(test.first)+"; else printf '%s' "+shellQuote(test.second)+"; fi\n")
			prefix := filepath.Join(dir, "assessment")
			options := PilotOptions{Agent: agent, AgentArgs: stringList{"{phase}"}, Timeout: 5 * time.Second}
			result, commands, err := runJSONPhase(context.Background(), options, dir, "Assess the source.", prefix, assessmentPhase())
			require.Len(t, commands, 2)
			if test.fail {
				require.ErrorContains(t, err, "after one retry")
			} else {
				require.NoError(t, err)
				require.Equal(t, "policy", result.Relationship)
			}
			require.FileExists(t, prefix+".prompt")
			require.FileExists(t, prefix+".stdout")
			require.FileExists(t, prefix+"-retry.stdout")
			prompt, readErr := os.ReadFile(prefix + "-retry.prompt")
			require.NoError(t, readErr)
			require.Contains(t, string(prompt), "Assess the source.")
			require.Contains(t, string(prompt), "previous response could not be accepted")
			require.Contains(t, string(prompt), `"change_basis"`)
		})
	}
}

func TestReviewRetriesInvalidJudgment(t *testing.T) {
	dir := t.TempDir()
	invalid := DesignReview{}
	valid := DesignReview{Finding: Judgment{"tradeoff", "finding", []string{"f.go:2"}}, Direction: Judgment{"tradeoff", "direction", []string{"f.go:2"}}, Candidate: Judgment{"not-applicable", "no exploration", []string{"actual-changes"}}, Actual: Judgment{"beneficial", "change", []string{"f.go:2"}}}
	first, _ := json.Marshal(invalid)
	second, _ := json.Marshal(valid)
	marker := filepath.Join(dir, "first")
	agent := executable(t, filepath.Join(dir, "agent"), "cat >/dev/null\nif [ ! -f "+shellQuote(marker)+" ]; then touch "+shellQuote(marker)+"; printf '%s' "+shellQuote(string(first))+"; else printf '%s' "+shellQuote(string(second))+"; fi\n")
	result, commands, err := runJSONPhase(context.Background(), PilotOptions{Agent: agent, AgentArgs: stringList{"{phase}"}, Timeout: 5 * time.Second}, dir, "Review changes.", filepath.Join(dir, "review"), reviewPhase(false))
	require.NoError(t, err)
	require.Len(t, commands, 2)
	require.Equal(t, "beneficial", result.Actual.Outcome)
}

func TestJSONPhaseDoesNotRetryProcessFailuresOrCancellation(t *testing.T) {
	dir := t.TempDir()
	agent := executable(t, filepath.Join(dir, "agent"), "cat >/dev/null\nexit 7\n")
	options := PilotOptions{Agent: agent, AgentArgs: stringList{"{phase}"}, Timeout: time.Second}
	_, commands, err := runJSONPhase(context.Background(), options, dir, "assess", filepath.Join(dir, "failure"), assessmentPhase())
	require.Error(t, err)
	require.Len(t, commands, 1)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, commands, err = runJSONPhase(ctx, options, dir, "assess", filepath.Join(dir, "cancelled"), assessmentPhase())
	require.ErrorIs(t, err, context.Canceled)
	require.Empty(t, commands)
}

func TestAssessmentRejectsSourceMutationAfterJSONRetry(t *testing.T) {
	source, before, _ := newHistory(t)
	checkout := filepath.Join(t.TempDir(), "checkout")
	require.NoError(t, checkoutRevision(context.Background(), source, before, checkout))
	dir := t.TempDir()
	marker := filepath.Join(dir, "first")
	valid := `{"relationship":"policy","future_change":"add operation","change_basis":"hypothetical","evidence":["f.go:2"]}`
	agent := executable(t, filepath.Join(dir, "agent"), "cat >/dev/null\nif [ ! -f "+shellQuote(marker)+" ]; then touch "+shellQuote(marker)+"; echo changed > f.go; echo broken; else printf '%s' "+shellQuote(valid)+"; fi\n")
	trial := pilotTrial{options: PilotOptions{Agent: agent, AgentArgs: stringList{"{phase}"}, Timeout: time.Second}, finding: PilotCase{Before: before, Reaction: "thumbs-down"}, root: dir, candidate: checkout}
	require.ErrorContains(t, trial.assess(context.Background()), "assessment modified source")
	require.Len(t, trial.result.AssessmentCommands, 2)
}

func TestPilotUnrelatedActualDiffNeedsNoAgentAndResumes(t *testing.T) {
	source, _, before := newHistory(t)
	require.NoError(t, os.WriteFile(filepath.Join(source, "unrelated.txt"), []byte("later change"), 0600))
	after := commitFiles(t, source)
	path := "f.go"
	finding := PilotCase{ID: "unrelated", CaseID: "C-unrelated", Reaction: "unreacted", Before: before, After: after, Finding: Comment{Path: &path}, Initial: Observation{GitDir: source}, Latest: Observation{GitDir: source}}
	options := PilotOptions{Out: t.TempDir(), Agent: "/bin/false", Module: "."}
	require.NoError(t, runPilotCase(context.Background(), options, finding))
	resultPath := filepath.Join(options.Out, "trials", finding.ID, "result.json")
	first, err := os.ReadFile(resultPath)
	require.NoError(t, err)
	var result TrialResult
	require.NoError(t, json.Unmarshal(first, &result))
	require.Equal(t, "no-response", result.Review.Actual.Outcome)
	require.Empty(t, result.AssessmentCommands)
	require.Empty(t, result.ReviewCommands)
	require.Empty(t, result.BaselineChecks)
	require.False(t, result.ActualBehaviorChecksPassed)
	require.NoError(t, pilotSummary(options.Out, PilotPlan{Cases: []PilotCase{finding}, ActualChangeCounts: map[string]ActualChangeCounts{"unreacted": {NoResponse: 1}}}))
	report, err := os.ReadFile(filepath.Join(options.Out, "report.md"))
	require.NoError(t, err)
	require.Contains(t, string(report), "actual-changes | no-response | not verified")
	require.NoError(t, runPilotCase(context.Background(), options, finding))
	second, err := os.ReadFile(resultPath)
	require.NoError(t, err)
	require.Equal(t, string(first), string(second))
}

func TestPilotPrefersRelevantChangesAcrossRulesAndReportsPopulation(t *testing.T) {
	source, before, after := newHistory(t)
	path := "f.go"
	observations := []Observation{}
	for i, rule := range []string{"aaa-unchanged", "zzz-changed"} {
		later := before
		if i == 1 {
			later = after
		}
		comment := Comment{Kind: ReviewComment, ID: int64(i + 1), Path: &path, CommitID: &before, Marker: &Marker{CaseID: rule, Role: "finding", Rule: rule}}
		observations = append(observations, Observation{GitDir: source, Record: PullRequestRecord{PullRequest: PullRequest{Number: int64(i + 1)}, Comments: []Comment{comment}, Reactions: []Reaction{{CommentKind: ReviewComment, CommentID: comment.ID, Content: "+1"}}}, Revisions: []Revision{{SHA: before, Present: true}, {SHA: later, Present: true, Roles: []string{"pr-head"}}}})
	}
	plan := makePilotPlan(context.Background(), PilotOptions{Limit: 1, AllowReconstructed: true, Seed: "same"}, observations)
	require.Len(t, plan.Cases, 1)
	require.Equal(t, "zzz-changed", plan.Cases[0].CaseID)
	require.Equal(t, ActualChangeCounts{Changed: 1, NoResponse: 1}, plan.ActualChangeCounts["thumbs-up"])
	root := t.TempDir()
	require.NoError(t, pilotSummary(root, plan))
	report, err := os.ReadFile(filepath.Join(root, "report.md"))
	require.NoError(t, err)
	require.Contains(t, string(report), "| thumbs-up | 1 | 1 | 0 |")
}

func TestActualChangeMissingEvidenceStaysUnknownAndPathsAreLiteral(t *testing.T) {
	source, before, after := newHistory(t)
	finding := PilotCase{Before: before, After: after, Latest: Observation{GitDir: source}}
	require.Equal(t, actualUnavailable, inspectActualChange(context.Background(), finding).Status)
	path := "f*.go"
	finding.Finding.Path = &path
	require.Equal(t, actualNoResponse, inspectActualChange(context.Background(), finding).Status)
	finding.After = ""
	require.Equal(t, actualUnavailable, inspectActualChange(context.Background(), finding).Status)
	finding.After = strings.Repeat("f", 40)
	require.Equal(t, actualUnavailable, inspectActualChange(context.Background(), finding).Status)
}

func TestDefaultAgentRequestsStructuredOutputAndNarrowReviewCommands(t *testing.T) {
	root := filepath.Join(t.TempDir(), "root with spaces")
	before := filepath.Join(root, "before")
	actual := filepath.Join(root, "actual")
	require.NoError(t, os.MkdirAll(before, 0700))
	require.NoError(t, os.MkdirAll(actual, 0700))
	agent := executable(t, filepath.Join(root, "agent"), "cat >/dev/null\nprintf '%s' '{\"type\":\"result\",\"subtype\":\"success\",\"structured_output\":{\"relationship\":\"policy\"}}'\n")
	result := invokeAgent(context.Background(), PilotOptions{Agent: agent, Turns: 3, Timeout: time.Second}, before, "review", "Review the source.", filepath.Join(root, "review"), assessmentPhase().schema)
	require.True(t, commandPassed(result))
	arg := func(name string) string {
		for i, value := range result.Args {
			if value == name && i+1 < len(result.Args) {
				return result.Args[i+1]
			}
		}
		return ""
	}
	require.Equal(t, "json", arg("--output-format"))
	require.Equal(t, assessmentPhase().schema, arg("--json-schema"))
	allowed := arg("--allowedTools")
	require.Contains(t, allowed, "Bash(git -C "+quoteShell(before)+" log:*)")
	require.Contains(t, allowed, "Bash(git -C "+quoteShell(actual)+" log:*)")
	require.Contains(t, allowed, "Bash(git -C "+quoteShell(actual)+" diff:*)")
	require.NotContains(t, allowed, "Bash(git -C:*)")
	require.Contains(t, strings.Join(result.Args, " "), "--safe-mode --restricted")
	var value DesignAssessment
	require.NoError(t, readAgentJSON(result.Stdout, &value))
	require.Equal(t, "policy", value.Relationship)
	require.Contains(t, readOnlyGitCommands(before, actual), "git -C "+quoteShell(actual)+" log -5 --oneline")
}
