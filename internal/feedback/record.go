package feedback

import (
	"iter"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

type PullRequestRecord struct {
	PullRequest  PullRequest
	Comments     []Comment
	Reactions    []Reaction
	Commits      []Commit
	Suppressions []Suppression
}

type PullRequest struct {
	ID         int64
	Repository string
	Number     int64
	Title      string
	Author     string
	State      string
	Merged     bool
	CreatedAt  time.Time
	ClosedAt   *time.Time
}

type CommentKind string

const (
	ReviewComment CommentKind = "review"
	IssueComment  CommentKind = "issue"
)

type Comment struct {
	Kind      CommentKind
	ID        int64
	Author    string
	Body      string
	Path      *string
	Line      *int64
	CommitID  *string
	InReplyTo *int64
	CreatedAt time.Time
	UpdatedAt time.Time
	Marker    *Marker
}

type Marker struct {
	CaseID string
	Role   string
	State  string
	Rule   string
}

type Reaction struct {
	ID          int64
	CommentKind CommentKind
	CommentID   int64
	Author      string
	Content     string
	CreatedAt   time.Time
}

type Commit struct {
	SHA         string
	Author      *string
	Message     string
	CommittedAt time.Time
	Files       []CommitFile
}

type CommitFile struct {
	Path  string
	Patch *string
}

type Suppression struct {
	Path          string
	Line          int64
	Rule          string
	Justification string
}

var (
	markerPattern      = regexp.MustCompile(`\A<!-- columbo-feedback case=(\S+) role=(\S+) state=(open|cleared) -->`)
	ruleHeaderPattern  = regexp.MustCompile(`^\*\*Columbo: ([^\s*]+)\*\*`)
	suppressionPattern = regexp.MustCompile(`^\s*//[\t ]*columbo:ignore[\t ]+(\S+)[\t ]+--[\t ]+(.+)$`)
	hunkPattern        = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,\d+)? @@`)
)

func parseMarker(body string) *Marker {
	match := markerPattern.FindStringSubmatch(body)
	if match == nil {
		return nil
	}
	marker := &Marker{CaseID: match[1], Role: match[2], State: match[3]}
	if lines := strings.SplitN(body, "\n", 3); len(lines) > 1 {
		if rule := ruleHeaderPattern.FindStringSubmatch(lines[1]); rule != nil {
			marker.Rule = rule[1]
		}
	}
	return marker
}

func parseSuppressions(path, patch string) []Suppression {
	var found []Suppression
	for line, text := range addedLines(patch) {
		match := suppressionPattern.FindStringSubmatch(text)
		if match != nil && strings.TrimSpace(match[2]) != "" {
			found = append(found, Suppression{Path: path, Line: line, Rule: match[1], Justification: strings.TrimSpace(match[2])})
		}
	}
	return found
}

func addedLines(patch string) iter.Seq2[int64, string] {
	return func(yield func(int64, string) bool) {
		line := int64(0)
		for _, text := range strings.Split(patch, "\n") {
			hunk := hunkPattern.FindStringSubmatch(text)
			switch {
			case hunk != nil:
				line, _ = strconv.ParseInt(hunk[1], 10, 64)
			case strings.HasPrefix(text, "+"):
				if !yield(line, text[1:]) {
					return
				}
				line++
			case strings.HasPrefix(text, " "):
				line++
			}
		}
	}
}

const missPrefix = "columbo missed:"

func missText(body string) (string, bool) {
	body = strings.TrimLeftFunc(body, unicode.IsSpace)
	if len(body) < len(missPrefix) || !strings.EqualFold(body[:len(missPrefix)], missPrefix) {
		return "", false
	}
	return body[len(missPrefix):], true
}
