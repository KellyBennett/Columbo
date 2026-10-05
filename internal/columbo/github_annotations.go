package columbo

import (
	"crypto/rand"
	"fmt"
	"strings"
)

// Workflow commands are appended to the existing snapshot summary. Neither
// the analyzer nor the rule thresholds are consulted by this presentation.
type githubAnnotation struct {
	severity, title, path, message string
	line, endLine                  int64
}

func (c summaryCase) annotate(r *snapshotRenderer, message string) {
	severity := "warning"
	if c.Verdict == "FAIL" {
		severity = "error"
	}
	githubAnnotation{severity: severity, title: "Columbo: " + c.Smell, path: c.Path, line: c.StartLine, endLine: c.EndLine, message: strings.TrimSpace(message)}.write(r)
}

func (a githubAnnotation) write(r *snapshotRenderer) {
	fmt.Fprintf(&r.github, "::%s title=%s", a.severity, escapeGitHubProperty(a.title))
	if a.path != "" {
		fmt.Fprintf(&r.github, ",file=%s,line=%d", escapeGitHubProperty(a.path), a.line)
		if a.endLine > a.line {
			fmt.Fprintf(&r.github, ",endLine=%d", a.endLine)
		}
	}
	fmt.Fprintf(&r.github, "::%s\n", escapeGitHubData(a.message))
}

func (r *snapshotRenderer) output() []byte {
	if !r.annotations {
		return r.buffer.Bytes()
	}
	// Disable command parsing for ordinary summary text, which may contain
	// repository-controlled paths or messages. Resume only for escaped commands.
	token := rand.Text()
	return []byte("::stop-commands::" + token + "\n" + r.buffer.String() + "::" + token + "::\n" + r.github.String())
}

func escapeGitHubData(value string) string {
	return strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A").Replace(value)
}

func escapeGitHubProperty(value string) string {
	return strings.NewReplacer(":", "%3A", ",", "%2C").Replace(escapeGitHubData(value))
}
