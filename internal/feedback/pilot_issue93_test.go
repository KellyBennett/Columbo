package feedback

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

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
