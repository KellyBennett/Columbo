package feedback

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/KellyBennett/Columbo/internal/feedbackdb"
	"github.com/stretchr/testify/require"
)

func TestParallelPilotMatchesSequentialPilotAndResumes(t *testing.T) {
	source, before, after := newHistory(t)
	root := t.TempDir()
	client, err := newGitHubClient("https://api.github.com", "")
	require.NoError(t, err)
	c := collector{repository: "o/r", github: client, options: CollectOptions{Checkout: source, EvidencePath: filepath.Join(root, "evidence")}}
	flagged := "f.go"
	o := Observation{ID: "observation", ObservedAt: "2026-01-01T00:00:00Z", Record: PullRequestRecord{PullRequest: PullRequest{Number: 1}}, Revisions: []Revision{{SHA: before, Roles: []string{"original-review-anchor"}}, {SHA: after, Roles: []string{"pr-head"}}}}
	for i, content := range []string{"-1", "+1", ""} {
		id := int64(i + 1)
		o.Record.Comments = append(o.Record.Comments, Comment{Path: &flagged, Kind: ReviewComment, ID: id, CommitID: &before, Body: "Investigate ownership", Marker: &Marker{CaseID: fmt.Sprintf("C-%d", id), Role: "finding", Rule: "feature-envy"}})
		if content != "" {
			o.Record.Reactions = append(o.Record.Reactions, Reaction{CommentKind: ReviewComment, CommentID: id, Content: content})
		}
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
	ready := filepath.Join(root, "ready")
	agent := executable(t, filepath.Join(root, "agent"), `
prompt=$(cat)
case "$1" in
assess)
 case "$prompt" in *C-2*) [ -f `+shellQuote(ready)+` ] || exit 7;; esac
 echo '{"relationship":"shared policy","opportunity":"one owner","future_change":"add an operation","change_basis":"hypothetical","costs":"extra call","evidence":["f.go:2"]}'
 ;;
refactor)
 printf 'package p\nfunc Sum(a, b int) int { return add(a,b) }\nfunc add(a,b int) int { return a+b }\n' > f.go
 ;;
review)
 candidate=tradeoff
 case "$prompt" in *"Do not invent a candidate"*) candidate=not-applicable;; esac
 echo '{"finding":{"outcome":"tradeoff","reason":"policy","evidence":["f.go:2"]},"direction":{"outcome":"tradeoff","reason":"abstraction","evidence":["f.go:3"]},"candidate":{"outcome":"'$candidate'","reason":"ownership","evidence":["f.go:2-3"]},"actual":{"outcome":"no-demonstrated-benefit","reason":"same operation","evidence":["f.go:2"]},"limitations":["synthetic test"],"reported_model":"test"}'
 ;;
esac
`)
	run := func(parallel int) (string, map[string]TrialResult) {
		require.NoError(t, os.RemoveAll(ready))
		options := PilotOptions{Repository: "o/r", Store: storePath, Out: filepath.Join(root, fmt.Sprintf("pilot-%d", parallel)), Agent: agent, AgentArgs: stringList{"{phase}"}, Columbo: analyzer, Limit: 3, Turns: 3, Attempts: 1, Timeout: 30 * time.Second, CheckTimeout: time.Minute, AllowReconstructed: true, Module: ".", Parallel: parallel}
		var out strings.Builder
		require.ErrorContains(t, Pilot(context.Background(), options, &out), "1 trials incomplete", out.String())
		require.Contains(t, out.String(), "C-2 (thumbs-up): incomplete")
		require.NoError(t, os.WriteFile(ready, nil, 0o600))
		require.NoError(t, Pilot(context.Background(), options, &out), out.String())
		var plan PilotPlan
		data, err := os.ReadFile(filepath.Join(options.Out, "plan.json"))
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(data, &plan))
		require.Len(t, plan.Cases, 3)
		results := map[string]TrialResult{}
		for _, finding := range plan.Cases {
			var result TrialResult
			data, err := os.ReadFile(filepath.Join(options.Out, "trials", finding.ID, "result.json"))
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(data, &result))
			executions, err := os.ReadDir(filepath.Join(options.Out, "trials", finding.ID, "executions"))
			require.NoError(t, err)
			require.Len(t, executions, map[bool]int{true: 2, false: 1}[finding.CaseID == "C-2"], finding.CaseID)
			results[finding.CaseID] = TrialResult{Complete: result.Complete, Review: result.Review, BehaviorChecksPassed: result.BehaviorChecksPassed, ActualBehaviorChecksPassed: result.ActualBehaviorChecksPassed, ChangedPaths: result.ChangedPaths, BaselineTestScope: result.BaselineTestScope}
		}
		report, err := os.ReadFile(filepath.Join(options.Out, "report.md"))
		require.NoError(t, err)
		return string(report), results
	}
	sequentialReport, sequential := run(1)
	parallelReport, parallel := run(2)
	require.Equal(t, sequential, parallel)
	require.Equal(t, sequentialReport, parallelReport)
	require.True(t, sequential["C-1"].Complete && sequential["C-2"].Complete && sequential["C-3"].Complete)
	require.True(t, sequential["C-1"].BehaviorChecksPassed)
	require.Equal(t, "tradeoff", sequential["C-1"].Review.Candidate.Outcome)
}
