package feedback

import (
	"cmp"
	"context"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/KellyBennett/Columbo/internal/columbo"
	"github.com/KellyBennett/Columbo/internal/feedbackdb"
)

type ruleTally struct {
	rule                 string
	thumbsUp, thumbsDown int
	missed               int
}

func Status(ctx context.Context, storePath string, out io.Writer) error {
	store, err := feedbackdb.Open(storePath)
	if err != nil {
		return err
	}
	defer store.Close()
	q := store.Queries()
	rules, err := q.FindingRules(ctx)
	if err != nil {
		return err
	}
	reactions, err := q.FindingReactions(ctx)
	if err != nil {
		return err
	}
	candidates, err := q.MissCandidates(ctx)
	if err != nil {
		return err
	}
	writeTable(out, tally(rules, reactions, misses(candidates, columbo.SmellIDs())))
	return nil
}

func tally(rules []*string, reactions []feedbackdb.FindingReactionsRow, missed []string) []ruleTally {
	byRule := map[string]*ruleTally{}
	get := func(rule string) *ruleTally {
		if byRule[rule] == nil {
			byRule[rule] = &ruleTally{rule: rule}
		}
		return byRule[rule]
	}
	for _, rule := range rules {
		get(*rule)
	}
	for _, row := range reactions {
		get(*row.Rule).add(row.Content, int(row.Reactions))
	}
	for _, rule := range missed {
		get(rule).missed++
	}
	tallies := make([]ruleTally, 0, len(byRule))
	for _, t := range byRule {
		tallies = append(tallies, *t)
	}
	slices.SortFunc(tallies, func(a, b ruleTally) int {
		return cmp.Or(cmp.Compare(a.lastIfNoRule(), b.lastIfNoRule()), cmp.Compare(a.rule, b.rule))
	})
	return tallies
}

func (t ruleTally) lastIfNoRule() int {
	if t.rule == "" {
		return 1
	}
	return 0
}

func (t *ruleTally) add(content string, count int) {
	switch content {
	case "+1":
		t.thumbsUp += count
	case "-1":
		t.thumbsDown += count
	}
}

func misses(bodies []string, known []string) []string {
	var rules []string
	for _, body := range bodies {
		if rule, ok := classifyMiss(body, known); ok {
			rules = append(rules, rule)
		}
	}
	return rules
}

func classifyMiss(body string, known []string) (rule string, isMiss bool) {
	text, isMiss := missText(body)
	if !isMiss {
		return "", false
	}
	quoted := make([]string, len(known))
	for i, id := range known {
		quoted[i] = regexp.QuoteMeta(id)
	}
	wholeWord := regexp.MustCompile(`(?:^|[^\w-])(` + strings.Join(quoted, "|") + `)(?:$|[^\w-])`)
	if match := wholeWord.FindStringSubmatch(strings.ToLower(text)); match != nil {
		return match[1], true
	}
	return "", true
}

func writeTable(out io.Writer, tallies []ruleTally) {
	rows := [][]string{{"rule", "👍", "👎", "missed"}}
	for _, t := range tallies {
		rows = append(rows, []string{cmp.Or(t.rule, "no rule"), strconv.Itoa(t.thumbsUp), strconv.Itoa(t.thumbsDown), strconv.Itoa(t.missed)})
	}
	widths := make([]int, len(rows[0]))
	for _, row := range rows {
		for i, cell := range row {
			widths[i] = max(widths[i], displayWidth(cell))
		}
	}
	for _, row := range rows {
		line := row[0] + strings.Repeat(" ", widths[0]-displayWidth(row[0]))
		for i, cell := range row[1:] {
			line += "  " + strings.Repeat(" ", widths[i+1]-displayWidth(cell)) + cell
		}
		fmt.Fprintln(out, line)
	}
}

func displayWidth(text string) int {
	width := utf8.RuneCountInString(text)
	for _, r := range text {
		if r >= 0x1F000 {
			width++
		}
	}
	return width
}
