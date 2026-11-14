package trials

import (
	"fmt"
	"strings"

	"github.com/KellyBennett/Columbo/calibration/harness"
)

// PromptVersion changes whenever the prompt text does, so trials under different wording never pool.
const PromptVersion = "v1"

// Prompt hands the agent Columbo's output for the cases the mutant added and nothing about how they arose.
func Prompt(mutant harness.Snapshot, added []harness.Finding) string {
	ids := map[string]bool{}
	for _, f := range added {
		ids[f.CaseID] = true
	}
	var b strings.Builder
	b.WriteString("Columbo is a static checker for architectural smells in Go. Running `columbo --staged=false --no-history ./...` in this module reports these cases:\n\n")
	b.WriteString(caseBlocks(mutant.Output, ids))
	b.WriteString("\nColumbo's guidance for each case:\n")
	for _, f := range added {
		g := mutant.Guidance[f.CaseID]
		fmt.Fprintf(&b, "\nCASE %s\nWhy: %s\nDiagnosis: %s\n", f.CaseID, g.Why, g.Diagnosis)
		writeList(&b, "Leads", g.Leads)
		writeList(&b, "Avoid", g.Avoid)
	}
	b.WriteString("\nChange the code so Columbo no longer reports these cases and reports no new ones, while `go vet ./...` and `go test ./...` keep passing. ")
	b.WriteString("Do not add suppression directives, Columbo configuration, or generated-code markers, and do not delete or weaken test assertions; updating test call sites is fine.\n")
	if others := len(mutant.Findings) - len(added); others > 0 {
		fmt.Fprintf(&b, "\nColumbo reports %d other case(s) in this module. They are out of scope; leave that code alone.\n", others)
	}
	b.WriteString("\nYour shell runs `go build`, `go vet`, `go test`, `columbo`, and read-only commands such as `ls`, `cat`, and `grep`; anything else is denied, so make every edit with your file tools. ")
	b.WriteString("Check your work with `go vet ./...`, `go test ./...`, and `columbo --staged=false --no-history ./...` in this directory. ")
	b.WriteString("Columbo exits 1 while any FAIL case remains, out-of-scope ones included, so read its output for the cases above rather than relying on its exit code.\n")
	b.WriteString("\nWhen you are done, reply with a short summary of what you changed.\n")
	return b.String()
}

func writeList(b *strings.Builder, title string, items []string) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(b, "%s:\n", title)
	for _, it := range items {
		fmt.Fprintf(b, "- %s\n", it)
	}
}

// caseBlocks keeps each selected CASE header and the indented lines under it, in Columbo's order.
func caseBlocks(output string, ids map[string]bool) string {
	var b strings.Builder
	keep := false
	for _, line := range strings.SplitAfter(output, "\n") {
		if fields := strings.Fields(line); strings.HasPrefix(line, "CASE ") && len(fields) > 1 {
			keep = ids[fields[1]]
		} else if !strings.HasPrefix(line, "  ") {
			keep = false
		}
		if keep {
			b.WriteString(line)
		}
	}
	return b.String()
}
