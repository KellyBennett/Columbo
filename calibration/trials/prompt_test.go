package trials

import (
	"strings"
	"testing"

	"github.com/KellyBennett/Columbo/calibration/harness"
)

const output = `CASE C-aaaaaaaaaa example.com/p.Old p.go:1-20
  long-function: FAIL
  function-lines example.com/p.Old: 14 (limit > 10)
CASE C-bbbbbbbbbb example.com/p.Total p.go:22-40
  long-function: FAIL
  function-lines example.com/p.Total: 12 (limit > 10)
Columbo: 2 failed, 0 warned, 0 suppressed
Database: /tmp/x.sqlite
`

func mutantSnapshot() (harness.Snapshot, []harness.Finding) {
	planted := harness.Finding{Smell: "long-function", Symbol: "example.com/p.Total", CaseID: "C-bbbbbbbbbb"}
	snap := harness.Snapshot{
		Output:   output,
		Findings: []harness.Finding{{Smell: "long-function", Symbol: "example.com/p.Old", CaseID: "C-aaaaaaaaaa"}, planted},
		Guidance: map[string]harness.Guidance{"C-bbbbbbbbbb": {
			Why: "Long functions can obscure responsibility boundaries and ownership.", Diagnosis: "example.com/p.Total exceeds the configured source-line limit.",
			Leads: []string{"Look for cohesive responsibilities with different reasons to change."},
			Avoid: []string{"Extracting arbitrary helpers solely to reduce the line count."},
		}},
	}
	return snap, []harness.Finding{planted}
}

func TestPromptShowsOnlyTheAddedCasesWithTheirGuidance(t *testing.T) {
	snap, added := mutantSnapshot()
	p := Prompt(snap, added)
	for _, want := range []string{
		"CASE C-bbbbbbbbbb example.com/p.Total p.go:22-40\n  long-function: FAIL\n  function-lines example.com/p.Total: 12 (limit > 10)\n",
		"- Look for cohesive responsibilities with different reasons to change.",
		"- Extracting arbitrary helpers solely to reduce the line count.",
		"Columbo reports 1 other case(s) in this module.",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("prompt lacks %q:\n%s", want, p)
		}
	}
	for _, leak := range []string{"C-aaaaaaaaaa", "Database:", "Columbo: 2 failed"} {
		if strings.Contains(p, leak) {
			t.Errorf("prompt carries %q from outside the added cases", leak)
		}
	}
}

func TestPromptNeverNamesTheMutation(t *testing.T) {
	snap, added := mutantSnapshot()
	p := strings.ToLower(Prompt(snap, added))
	for _, leak := range []string{"mutant", "mutation", "inline", "inject", "planted", "original"} {
		if strings.Contains(p, leak) {
			t.Errorf("prompt says %q", leak)
		}
	}
}

func TestPromptOmitsTheOutOfScopeNoteWhenNothingElseIsReported(t *testing.T) {
	snap, added := mutantSnapshot()
	snap.Findings = added
	if p := Prompt(snap, added); strings.Contains(p, "other case") {
		t.Errorf("prompt mentions other cases when there are none:\n%s", p)
	}
}
