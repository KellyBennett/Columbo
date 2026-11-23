package feedback

import (
	"context"
	"fmt"
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

func inspectActualChange(ctx context.Context, finding PilotCase) ActualChange {
	if finding.After == "" {
		return ActualChange{actualUnavailable, "No later PR revision is available."}
	}
	if finding.Before == finding.After {
		return ActualChange{actualNoResponse, "The starting and later PR revisions are identical: " + finding.Before}
	}
	paths, err := git(ctx, finding.Latest.GitDir, "diff", "--no-ext-diff", "--no-textconv", "--no-renames", "--name-only", "-z", finding.Before, finding.After, "--")
	if err != nil {
		return ActualChange{actualUnavailable, "Later diff unavailable: " + err.Error()}
	}
	if paths == "" {
		return ActualChange{actualNoResponse, "The starting and later PR revisions have identical files."}
	}
	if finding.Finding.Path == nil || *finding.Finding.Path == "" {
		return ActualChange{actualUnavailable, "The finding has no flagged file, so the later diff's relevance is unknown."}
	}
	for _, path := range strings.Split(paths, "\x00") {
		if path == *finding.Finding.Path {
			return ActualChange{actualChanged, "The later diff touches flagged file " + path}
		}
	}
	return ActualChange{actualNoResponse, "The later diff does not touch flagged file " + *finding.Finding.Path}
}

func (change ActualChange) judgment() Judgment {
	return Judgment{Outcome: string(change.Status), Reason: change.Reason, Evidence: []string{change.Reason}}
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

func (change ActualChange) priority() string {
	if change.Status == actualChanged {
		return "0"
	}
	return "1"
}

func actualChangeReport(counts map[string]ActualChangeCounts) string {
	if len(counts) == 0 {
		return ""
	}
	var report strings.Builder
	report.WriteString("## Actual-change candidates\n\nCounts include all eligible candidates before the sample limit. Changed findings take priority within each actual-changes reaction group. No-response records inaction, not a design judgment.\n\n| Reactions | Changed flagged file | No response | Unavailable evidence |\n| --- | --- | --- | --- |\n")
	for _, group := range []string{"thumbs-down", "thumbs-up", "unreacted", "mixed", "other"} {
		if count, ok := counts[group]; ok {
			fmt.Fprintf(&report, "| %s | %d | %d | %d |\n", group, count.Changed, count.NoResponse, count.Unavailable)
		}
	}
	report.WriteByte('\n')
	return report.String()
}
