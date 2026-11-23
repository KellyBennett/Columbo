package feedback

import (
	"context"
	"fmt"
	"slices"
	"strings"
)

type actualChangeStatus string

const (
	actualChanged     actualChangeStatus = "changed"
	actualNoResponse  actualChangeStatus = "no-response"
	actualUnavailable actualChangeStatus = "unavailable"
)

type ActualChange struct {
	Status actualChangeStatus `json:"status"`
	Reason string             `json:"reason"`
}

type ActualChangeCounts struct {
	Changed     int `json:"changed"`
	NoResponse  int `json:"no_response"`
	Unavailable int `json:"unavailable"`
}

func (c PilotCase) inspectActualChange(ctx context.Context) ActualChange {
	switch {
	case c.After == "":
		return ActualChange{actualUnavailable, "No later PR revision is available."}
	case c.Before == c.After:
		return ActualChange{actualNoResponse, "The starting and later PR revisions are identical: " + c.Before}
	}
	paths, err := git(ctx, c.Latest.GitDir, "diff", "--no-ext-diff", "--no-textconv", "--no-renames", "--name-only", "-z", c.Before, c.After, "--")
	if err != nil {
		return ActualChange{actualUnavailable, "Later diff unavailable: " + err.Error()}
	}
	return classifyLaterPaths(paths, c.Finding.Path)
}

func (c PilotCase) comparesActual() bool {
	return c.After != "" && c.ActualChange.Status != actualNoResponse
}

func (c PilotCase) actualAncestry(ctx context.Context) string {
	if _, err := git(ctx, c.Initial.GitDir, "merge-base", "--is-ancestor", c.Before, c.After); err == nil {
		return "later PR head descends from starting revision"
	}
	return "not established; revisions may have been rewritten"
}

func (c PilotCase) actualPatch(ctx context.Context) (string, error) {
	return git(ctx, c.Latest.GitDir, "diff", "--binary", "--no-ext-diff", "--no-textconv", c.Before, c.After)
}

func classifyLaterPaths(paths string, flagged *string) ActualChange {
	switch {
	case paths == "":
		return ActualChange{actualNoResponse, "The starting and later PR revisions have identical files."}
	case flagged == nil || *flagged == "":
		return ActualChange{actualUnavailable, "The finding has no flagged file, so the later diff's relevance is unknown."}
	case slices.Contains(strings.Split(paths, "\x00"), *flagged):
		return ActualChange{actualChanged, "The later diff touches flagged file " + *flagged}
	}
	return ActualChange{actualNoResponse, "The later diff does not touch flagged file " + *flagged}
}

func (change ActualChange) judgment() Judgment {
	outcome := string(change.Status)
	if change.Status == actualUnavailable {
		outcome = "inconclusive"
	}
	return Judgment{Outcome: outcome, Reason: change.Reason, Evidence: []string{change.Reason}}
}

func (counts *ActualChangeCounts) add(change ActualChange) {
	switch change.Status {
	case actualChanged:
		counts.Changed++
	case actualNoResponse:
		counts.NoResponse++
	default:
		counts.Unavailable++
	}
}

func actualChangeReport(counts map[string]ActualChangeCounts) string {
	if len(counts) == 0 {
		return ""
	}
	var report strings.Builder
	report.WriteString("## Actual-change candidates\n\nCounts include all eligible candidates before the sample limit. Changed findings take priority within each actual-changes reaction group. No-response records inaction, not a design judgment.\n\n| Reactions | Changed flagged file | No response | Unavailable evidence |\n| --- | --- | --- | --- |\n")
	for _, group := range reactionGroupOrder {
		if count, ok := counts[group]; ok {
			fmt.Fprintf(&report, "| %s | %d | %d | %d |\n", group, count.Changed, count.NoResponse, count.Unavailable)
		}
	}
	report.WriteByte('\n')
	return report.String()
}
