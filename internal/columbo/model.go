package columbo

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

var smells = []string{"long-function", "long-parameter-list", "high-cognitive-complexity", "excessive-dependencies", "feature-envy", "data-clump", "cosmetic-extraction"}

type Summary struct {
	Failed     int `json:"failed"`
	Warned     int `json:"warned"`
	Suppressed int `json:"suppressed"`
}
type Report struct {
	Version      int           `json:"version"`
	Summary      Summary       `json:"summary"`
	Cases        []Case        `json:"cases"`
	Suppressions []Suppression `json:"suppressions"`
	Warnings     []Warning     `json:"warnings"`
}
type Case struct {
	ID            string         `json:"id"`
	Smell         string         `json:"smell"`
	Verdict       string         `json:"verdict"`
	Symbol        string         `json:"symbol"`
	File          string         `json:"file"`
	StartLine     int            `json:"start_line"`
	EndLine       int            `json:"end_line"`
	Suppressed    bool           `json:"suppressed"`
	Clues         []Clue         `json:"clues"`
	Clusters      []Cluster      `json:"clusters"`
	Why           string         `json:"why"`
	Diagnosis     string         `json:"diagnosis"`
	Leads         []string       `json:"leads"`
	Avoid         []string       `json:"avoid"`
	Receipts      []any          `json:"receipts"`
	PolicyReviews []PolicyReview `json:"policy_reviews"`
}
type Clue struct {
	Kind     string `json:"kind"`
	Subject  string `json:"subject"`
	Value    any    `json:"value"`
	Limit    any    `json:"limit"`
	Operator any    `json:"operator"`
}
type Cluster struct {
	Key     string   `json:"key"`
	Owner   string   `json:"owner"`
	File    string   `json:"file"`
	Members []Member `json:"members"`
}
type Member struct {
	Helper     string `json:"helper"`
	CallOffset int    `json:"call_offset"`
}
type Site struct {
	File       string `json:"file"`
	CallOffset int    `json:"call_offset"`
}
type Detail struct {
	Subject        string   `json:"subject"`
	Value          any      `json:"value"`
	Nesting        any      `json:"nesting"`
	Expansion      []string `json:"expansion"`
	ExpansionSites []Site   `json:"expansion_sites"`
}
type Source struct {
	Kind        string `json:"kind"`
	File        string `json:"file"`
	StartLine   int    `json:"start_line"`
	EndLine     int    `json:"end_line"`
	StartOffset int    `json:"start_offset"`
	EndOffset   int    `json:"end_offset"`
	Detail      Detail `json:"detail"`
}
type History struct {
	Kind        string   `json:"kind"`
	Commit      string   `json:"commit"`
	CommittedAt int64    `json:"committed_at"`
	Files       []string `json:"files"`
}
type Suppression struct {
	Smell         string `json:"smell"`
	Symbol        string `json:"symbol"`
	File          string `json:"file"`
	Line          int    `json:"line"`
	Justification string `json:"justification"`
	Applied       bool   `json:"applied"`
	CaseID        any    `json:"case_id"`
}
type Warning struct {
	Code    string `json:"code"`
	File    string `json:"file"`
	Line    int    `json:"line"`
	Message string `json:"message"`
}
type PolicyReview struct {
	ID           string `json:"id"`
	Note         string `json:"note"`
	ReviewPrompt string `json:"review_prompt"`
}

var policy = PolicyReview{"CE-001", "This rule deliberately rejects some decompositions that may represent meaningful responsibilities. This is a provisional dogfooding policy; its review note does not change the case verdict.", "Show the human the parent and qualifying helpers, forwarded inputs, shared dependencies, overlap values and thresholds, and original versus expanded line and complexity metrics. Discuss whether the failure reflects the intended policy before changing code or requesting a suppression."}

const policyStatus = "provisional"

func ValidatePublicRelease() error {
	if policyStatus == "provisional" {
		return fmt.Errorf("public v1 release blocked by provisional policy %s", policy.ID)
	}
	return nil
}
func canonical(v any) string {
	var b bytes.Buffer
	e := json.NewEncoder(&b)
	e.SetEscapeHTML(false)
	_ = e.Encode(v)
	var normalized any
	d := json.NewDecoder(bytes.NewReader(b.Bytes()))
	d.UseNumber()
	_ = d.Decode(&normalized)
	b.Reset()
	e = json.NewEncoder(&b)
	e.SetEscapeHTML(false)
	_ = e.Encode(normalized)
	return strings.TrimSuffix(b.String(), "\n")
}

func identity(smell, file, symbol, key string) (string, string) {
	a := []string{smell, file, symbol, key}
	for i, s := range a {
		a[i] = strings.NewReplacer("%", "%25", "|", "%7C", "\r", "%0D", "\n", "%0A").Replace(s)
	}
	raw := strings.Join(a, "|")
	h := sha256.Sum256([]byte(raw))
	return "C-" + hex.EncodeToString(h[:])[:10], raw
}
func metric(kind, subject string, value, limit any, op any) Clue {
	return Clue{kind, subject, value, limit, op}
}
func sourceLess(a, b Source) bool {
	if a.File != b.File {
		return a.File < b.File
	}
	if a.StartLine != b.StartLine {
		return a.StartLine < b.StartLine
	}
	if a.EndLine != b.EndLine {
		return a.EndLine < b.EndLine
	}
	if a.StartOffset != b.StartOffset {
		return a.StartOffset < b.StartOffset
	}
	if a.EndOffset != b.EndOffset {
		return a.EndOffset < b.EndOffset
	}
	if a.Kind != b.Kind {
		return a.Kind < b.Kind
	}
	return canonical(a.Detail) < canonical(b.Detail)
}
func normalize(c *Case) {
	sort.Slice(c.Clues, func(i, j int) bool {
		a, b := c.Clues[i], c.Clues[j]
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.Subject != b.Subject {
			return a.Subject < b.Subject
		}
		return canonical([]any{a.Value, a.Limit, a.Operator}) < canonical([]any{b.Value, b.Limit, b.Operator})
	})
	var sources []Source
	var history []any
	aggregated := map[string]Source{}
	for _, r := range c.Receipts {
		if s, ok := r.(Source); ok {
			if s.Kind == "metric-contribution" && strings.Contains(s.Detail.Subject, "complexity") {
				base := s
				base.Detail.Value = nil
				k := canonical(base)
				if prev, yes := aggregated[k]; yes {
					s.Detail.Value = prev.Detail.Value.(int) + s.Detail.Value.(int)
				}
				aggregated[k] = s
			} else {
				sources = append(sources, s)
			}
		} else {
			history = append(history, r)
		}
	}
	for _, s := range aggregated {
		sources = append(sources, s)
	}
	sort.Slice(sources, func(i, j int) bool { return sourceLess(sources[i], sources[j]) })
	c.Receipts = []any{}
	last := ""
	for _, s := range sources {
		k := canonical(s)
		if k != last {
			c.Receipts = append(c.Receipts, s)
			last = k
		}
	}
	c.Receipts = append(c.Receipts, history...)
}
func (r *Report) finish() {
	sort.Slice(r.Cases, func(i, j int) bool {
		a, b := r.Cases[i], r.Cases[j]
		if a.File != b.File {
			return a.File < b.File
		}
		if a.StartLine != b.StartLine {
			return a.StartLine < b.StartLine
		}
		if a.Smell != b.Smell {
			return a.Smell < b.Smell
		}
		return a.ID < b.ID
	})
	for i := range r.Cases {
		c := &r.Cases[i]
		normalize(c)
		if c.Suppressed {
			r.Summary.Suppressed++
		} else if c.Verdict == "FAIL" {
			r.Summary.Failed++
		} else {
			r.Summary.Warned++
		}
	}
	sort.Slice(r.Suppressions, func(i, j int) bool {
		a, b := r.Suppressions[i], r.Suppressions[j]
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Smell < b.Smell
	})
	sort.Slice(r.Warnings, func(i, j int) bool {
		a, b := r.Warnings[i], r.Warnings[j]
		return fmt.Sprintf("%s\x00%s\x00%09d\x00%s", a.Code, a.File, a.Line, a.Message) < fmt.Sprintf("%s\x00%s\x00%09d\x00%s", b.Code, b.File, b.Line, b.Message)
	})
}
func Serialize(r Report, format string) ([]byte, error) {
	var b bytes.Buffer
	if format == "json" {
		e := json.NewEncoder(&b)
		e.SetEscapeHTML(false)
		if err := e.Encode(r); err != nil {
			return nil, err
		}
		return b.Bytes(), nil
	}
	for _, c := range r.Cases {
		fmt.Fprintf(&b, "CASE %s — %s\nVERDICT\n  ", c.ID, c.Symbol)
		if c.Suppressed {
			fmt.Fprintf(&b, "SUPPRESSED (original: %s)\n", c.Verdict)
			for _, s := range r.Suppressions {
				if s.CaseID == c.ID {
					fmt.Fprintf(&b, "  %s:%d — %s\n", s.File, s.Line, s.Justification)
				}
			}
		} else {
			fmt.Fprintln(&b, c.Verdict)
		}
		fmt.Fprintf(&b, "SMELL\n  %s\nCLUES\n", c.Smell)
		for _, cl := range c.Clusters {
			fmt.Fprintf(&b, "  cluster %s owner=%s file=%s\n", cl.Key, cl.Owner, cl.File)
			for _, m := range cl.Members {
				fmt.Fprintf(&b, "    %s call_offset=%d\n", m.Helper, m.CallOffset)
			}
		}
		for _, q := range c.Clues {
			fmt.Fprintf(&b, "  %s %s: %s", q.Kind, q.Subject, canonical(q.Value))
			if q.Limit != nil {
				fmt.Fprintf(&b, " (limit %v %s)", q.Operator, canonical(q.Limit))
			}
			fmt.Fprintln(&b)
		}
		fmt.Fprintf(&b, "WHY THIS MATTERS\n  %s\nDIAGNOSIS\n  %s\nLEADS\n", c.Why, c.Diagnosis)
		for _, s := range c.Leads {
			fmt.Fprintf(&b, "  → %s\n", s)
		}
		fmt.Fprintln(&b, "AVOID")
		for _, s := range c.Avoid {
			fmt.Fprintf(&b, "  ✗ %s\n", s)
		}
		fmt.Fprintln(&b, "RECEIPTS")
		for _, s := range c.Receipts {
			fmt.Fprintf(&b, "  %s\n", canonical(s))
		}
		if len(c.PolicyReviews) > 0 {
			fmt.Fprintln(&b, "DOGFOODING POLICY REVIEW")
			for _, p := range c.PolicyReviews {
				fmt.Fprintf(&b, "  %s\n  %s\n  %s\n", p.ID, p.Note, p.ReviewPrompt)
			}
		}
		fmt.Fprintln(&b)
	}
	if len(r.Cases) == 0 {
		fmt.Fprintln(&b, "Columbo: no cases")
	} else {
		fmt.Fprintf(&b, "Columbo: %d failed, %d warned, %d suppressed\n", r.Summary.Failed, r.Summary.Warned, r.Summary.Suppressed)
	}
	return b.Bytes(), nil
}
