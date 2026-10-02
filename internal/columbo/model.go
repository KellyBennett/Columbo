package columbo

import (
	"bytes"
	"cmp"
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
	b, _ := serializeJSON(v)
	normalized := decodeCanonical(b)
	b, _ = serializeJSON(normalized)
	return strings.TrimSuffix(string(b), "\n")
}
func decodeCanonical(b []byte) any {
	d := newJSONDecoder(b)
	d.UseNumber()
	var v any
	_ = d.Decode(&v)
	return v
}
func newJSONDecoder(b []byte) *json.Decoder { return json.NewDecoder(bytes.NewReader(b)) }
func identity(smell, file, symbol, key string) (string, string) {
	a := []string{smell, file, symbol, key}
	for i, s := range a {
		a[i] = strings.NewReplacer("%", "%25", "|", "%7C", "\r", "%0D", "\n", "%0A").Replace(s)
	}
	raw := strings.Join(a, "|")
	h := sha256.Sum256([]byte(raw))
	return "C-" + hex.EncodeToString(h[:])[:10], raw
}
func metric(kind, subject string, value any) Clue {
	return Clue{Kind: kind, Subject: subject, Value: value}
}
func (c Clue) compare(limit any, op any) Clue { c.Limit = limit; c.Operator = op; return c }
func sourceLess(a, b Source) bool {
	return cmp.Or(
		cmp.Compare(a.File, b.File),
		cmp.Compare(a.StartLine, b.StartLine),
		cmp.Compare(a.EndLine, b.EndLine),
		cmp.Compare(a.StartOffset, b.StartOffset),
		cmp.Compare(a.EndOffset, b.EndOffset),
		cmp.Compare(a.Kind, b.Kind),
		cmp.Compare(canonical(a.Detail), canonical(b.Detail)),
	) < 0
}
func normalize(c *Case) { c.sortClues(); c.normalizeReceipts() }
func (c *Case) sortClues() {
	sort.Slice(c.Clues, func(i, j int) bool { return c.Clues[i].before(c.Clues[j]) })
}
func (q Clue) before(other Clue) bool {
	if q.Kind != other.Kind {
		return q.Kind < other.Kind
	}
	if q.Subject != other.Subject {
		return q.Subject < other.Subject
	}
	return canonical([]any{q.Value, q.Limit, q.Operator}) < canonical([]any{other.Value, other.Limit, other.Operator})
}

type receiptCollection struct {
	sources    []Source
	history    []any
	aggregated map[string]Source
}

func (c *Case) normalizeReceipts() {
	rs := &receiptCollection{aggregated: map[string]Source{}}
	for _, r := range c.Receipts {
		rs.add(r)
	}
	c.Receipts = rs.finish()
}
func (rs *receiptCollection) add(r any) {
	s, ok := r.(Source)
	if !ok {
		rs.history = append(rs.history, r)
		return
	}
	if s.Kind == "metric-contribution" && strings.Contains(s.Detail.Subject, "complexity") {
		rs.aggregate(s)
		return
	}
	rs.sources = append(rs.sources, s)
}
func (rs *receiptCollection) aggregate(s Source) {
	base := s
	base.Detail.Value = nil
	key := canonical(base)
	if prev, ok := rs.aggregated[key]; ok {
		s.Detail.Value = prev.Detail.Value.(int) + s.Detail.Value.(int)
	}
	rs.aggregated[key] = s
}
func (rs *receiptCollection) finish() []any {
	for _, s := range rs.aggregated {
		rs.sources = append(rs.sources, s)
	}
	sort.Slice(rs.sources, func(i, j int) bool { return sourceLess(rs.sources[i], rs.sources[j]) })
	return append(uniqueSources(rs.sources), rs.history...)
}
func uniqueSources(sources []Source) []any {
	out := []any{}
	last := ""
	for _, s := range sources {
		key := canonical(s)
		if key != last {
			out = append(out, s)
			last = key
		}
	}
	return out
}
func (c Case) before(other Case) bool {
	return cmp.Or(cmp.Compare(c.File, other.File), cmp.Compare(c.StartLine, other.StartLine), cmp.Compare(c.Smell, other.Smell), cmp.Compare(c.ID, other.ID)) < 0
}
func (s Suppression) before(other Suppression) bool {
	return cmp.Or(cmp.Compare(s.File, other.File), cmp.Compare(s.Line, other.Line), cmp.Compare(s.Smell, other.Smell)) < 0
}
func (w Warning) sortKey() string {
	return fmt.Sprintf("%s\x00%s\x00%09d\x00%s", w.Code, w.File, w.Line, w.Message)
}
func (r *Report) finish() {
	r.sort()
	for i := range r.Cases {
		r.finishCase(&r.Cases[i])
	}
}
func (r *Report) sort() {
	sort.Slice(r.Cases, func(i, j int) bool { return r.Cases[i].before(r.Cases[j]) })
	sort.Slice(r.Suppressions, func(i, j int) bool { return r.Suppressions[i].before(r.Suppressions[j]) })
	sort.Slice(r.Warnings, func(i, j int) bool { return r.Warnings[i].sortKey() < r.Warnings[j].sortKey() })
}
func (r *Report) finishCase(c *Case) {
	normalize(c)
	if c.Suppressed {
		r.Summary.Suppressed++
		return
	}
	if c.Verdict == "FAIL" {
		r.Summary.Failed++
		return
	}
	r.Summary.Warned++
}
func textClueNumber(kind string, v any) string {
	if kind == "parameter-overlap" || kind == "dependency-overlap" || kind == "foreign-own-ratio" {
		switch n := v.(type) {
		case float64:
			return fmt.Sprintf("%.6f", n)
		case int:
			return fmt.Sprintf("%.6f", float64(n))
		}
	}
	return canonical(v)
}

func Serialize(r Report, format string) ([]byte, error) {
	if format == "json" {
		return serializeJSON(r)
	}
	w := &textWriter{}
	for _, c := range r.Cases {
		c.writeText(w, r.Suppressions)
	}
	r.writeSummary(w)
	return w.Bytes(), nil
}
func serializeJSON(v any) ([]byte, error) {
	var b bytes.Buffer
	e := json.NewEncoder(&b)
	e.SetEscapeHTML(false)
	if err := e.Encode(v); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

type textWriter struct{ bytes.Buffer }

func (w *textWriter) emit(format string, args ...any) { fmt.Fprintf(&w.Buffer, format, args...) }
func (c *Case) writeText(w *textWriter, suppressions []Suppression) {
	w.emit("CASE %s — %s\nVERDICT\n  ", c.ID, c.Symbol)
	c.writeVerdict(w, suppressions)
	w.emit("SMELL\n  %s\nCLUES\n", c.Smell)
	c.writeClusters(w)
	c.writeClues(w)
	c.writeGuidance(w)
	c.writeReceipts(w)
	c.writePolicy(w)
	w.emit("\n")
}
func (c *Case) writeVerdict(w *textWriter, suppressions []Suppression) {
	if !c.Suppressed {
		w.emit("%s\n", c.Verdict)
		return
	}
	w.emit("SUPPRESSED (original: %s)\n", c.Verdict)
	for _, s := range suppressions {
		if s.CaseID == c.ID {
			w.emit("  %s:%d — %s\n", s.File, s.Line, s.Justification)
		}
	}
}
func (c *Case) writeClusters(w *textWriter) {
	for _, cl := range c.Clusters {
		cl.writeText(w)
	}
}
func (cl Cluster) writeText(w *textWriter) {
	w.emit("  cluster %s owner=%s file=%s\n", cl.Key, cl.Owner, cl.File)
	for _, m := range cl.Members {
		w.emit("    %s call_offset=%d\n", m.Helper, m.CallOffset)
	}
}
func (c *Case) writeClues(w *textWriter) {
	for _, q := range c.Clues {
		q.writeText(w)
	}
}
func (q Clue) writeText(w *textWriter) {
	w.emit("  %s %s: %s", q.Kind, q.Subject, textClueNumber(q.Kind, q.Value))
	if q.Limit != nil {
		w.emit(" (limit %v %s)", q.Operator, textClueNumber(q.Kind, q.Limit))
	}
	w.emit("\n")
}
func (c *Case) writeGuidance(w *textWriter) {
	w.emit("WHY THIS MATTERS\n  %s\nDIAGNOSIS\n  %s\nLEADS\n", c.Why, c.Diagnosis)
	for _, s := range c.Leads {
		w.emit("  → %s\n", s)
	}
	w.emit("AVOID\n")
	for _, s := range c.Avoid {
		w.emit("  ✗ %s\n", s)
	}
}
func (c *Case) writeReceipts(w *textWriter) {
	w.emit("RECEIPTS\n")
	for _, s := range c.Receipts {
		w.emit("  %s\n", canonical(s))
	}
}
func (c *Case) writePolicy(w *textWriter) {
	if len(c.PolicyReviews) == 0 {
		return
	}
	w.emit("DOGFOODING POLICY REVIEW\n")
	for _, p := range c.PolicyReviews {
		w.emit("  %s\n  %s\n  %s\n", p.ID, p.Note, p.ReviewPrompt)
	}
}
func (r Report) writeSummary(w *textWriter) {
	if len(r.Cases) == 0 {
		w.emit("Columbo: no cases\n")
		return
	}
	w.emit("Columbo: %d failed, %d warned, %d suppressed\n", r.Summary.Failed, r.Summary.Warned, r.Summary.Suppressed)
}
func (c *Case) value(kind string, value any) {
	c.Clues = append(c.Clues, metric(kind, c.Symbol, value))
}
func (c *Case) threshold(kind string, value any, limit any, op string) {
	c.Clues = append(c.Clues, metric(kind, c.Symbol, value).compare(limit, op))
}
