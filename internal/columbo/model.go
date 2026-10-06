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

var smells = []string{"long-function", "long-parameter-list", "high-cognitive-complexity", "excessive-dependencies", "feature-envy", "data-clump", "cosmetic-extraction", "duplicate-code", "repeated-variant-decision", selectionSmell}

type Summary struct {
	Failed     int `json:"failed"`
	Warned     int `json:"warned"`
	Suppressed int `json:"suppressed"`
}

// DeclarationRef is a physical declaration identity. Display subjects are never
// used to reconstruct this identity or any evidence relationship.
type DeclarationRef struct {
	File   string
	Symbol string
}
type DependencyEvidence struct {
	Identity string
	Scored   bool
}
type DeclarationEvidence struct {
	Ref          DeclarationRef
	Source       Source
	Dependencies []DependencyEvidence
}
type CaseDeclaration struct {
	Declaration DeclarationRef
	Role        string
}
type MemberRef struct {
	ClusterKey string
	Helper     DeclarationRef
	CallOffset int
}

type Report struct {
	Version      int                   `json:"version"`
	Summary      Summary               `json:"summary"`
	Cases        []Case                `json:"cases"`
	Suppressions []Suppression         `json:"suppressions"`
	Warnings     []Warning             `json:"warnings"`
	Declarations []DeclarationEvidence `json:"-"`
}
type Case struct {
	ID                     string            `json:"id"`
	Smell                  string            `json:"smell"`
	Verdict                string            `json:"verdict"`
	Symbol                 string            `json:"symbol"`
	File                   string            `json:"file"`
	StartLine              int               `json:"start_line"`
	EndLine                int               `json:"end_line"`
	Suppressed             bool              `json:"suppressed"`
	Clues                  []Clue            `json:"clues"`
	Clusters               []Cluster         `json:"clusters"`
	Why                    string            `json:"why"`
	Diagnosis              string            `json:"diagnosis"`
	Leads                  []string          `json:"leads"`
	Avoid                  []string          `json:"avoid"`
	Receipts               []any             `json:"receipts"`
	PolicyReviews          []PolicyReview    `json:"policy_reviews"`
	PrimaryDeclaration     *DeclarationRef   `json:"-"`
	SupportingDeclarations []CaseDeclaration `json:"-"`
}
type Clue struct {
	Kind               string          `json:"kind"`
	Subject            string          `json:"subject"`
	Value              any             `json:"value"`
	Limit              any             `json:"limit"`
	Operator           any             `json:"operator"`
	Declaration        *DeclarationRef `json:"-"`
	ClusterKey         string          `json:"-"`
	Member             *MemberRef      `json:"-"`
	Pair               []MemberRef     `json:"-"`
	SupportingReceipts []string        `json:"-"`
}
type Cluster struct {
	Key              string          `json:"key"`
	Owner            string          `json:"owner"`
	File             string          `json:"file"`
	Members          []Member        `json:"members"`
	OwnerDeclaration *DeclarationRef `json:"-"`
}
type Member struct {
	Helper      string          `json:"helper"`
	CallOffset  int             `json:"call_offset"`
	Declaration *DeclarationRef `json:"-"`
}
type Site struct {
	File       string          `json:"file"`
	CallOffset int             `json:"call_offset"`
	Owner      *DeclarationRef `json:"-"`
}
type Detail struct {
	Subject               string           `json:"subject"`
	Value                 any              `json:"value"`
	Nesting               any              `json:"nesting"`
	Expansion             []string         `json:"expansion"`
	ExpansionSites        []Site           `json:"expansion_sites"`
	ExpansionDeclarations []DeclarationRef `json:"-"`
}
type Source struct {
	Spelling               string          `json:"spelling,omitempty"`
	Kind                   string          `json:"kind"`
	File                   string          `json:"file"`
	StartLine              int             `json:"start_line"`
	EndLine                int             `json:"end_line"`
	StartOffset            int             `json:"start_offset"`
	EndOffset              int             `json:"end_offset"`
	Detail                 Detail          `json:"detail"`
	Declaration            *DeclarationRef `json:"-"`
	DependencyIdentity     string          `json:"-"`
	DependencyScored       bool            `json:"-"`
	EvidenceKey            string          `json:"-"`
	AggregateContributions bool            `json:"-"`
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
	b, _ := encodeCanonicalJSON(v)
	normalized := decodeCanonical(b)
	b, _ = encodeCanonicalJSON(normalized)
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
	if q.Kind == "variant-set" && q.Subject != other.Subject {
		return variantSiteSubjectLess(q.Subject, other.Subject)
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
		if source, ok := r.(Source); ok {
			c.sourceDeclarations(source)
		}
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
	if s.AggregateContributions {
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
			s.EvidenceKey = sourceEvidenceKey(s)
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
func encodeCanonicalJSON(v any) ([]byte, error) {
	var b bytes.Buffer
	e := json.NewEncoder(&b)
	e.SetEscapeHTML(false)
	if err := e.Encode(v); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func (c *Case) metric(kind string, value any) Clue {
	return metric(kind, c.Symbol, value).forDeclaration(c.PrimaryDeclaration)
}

func (s Source) withExpansion(trace *expansion) Source {
	if trace != nil {
		s.Detail.Expansion = append([]string{}, trace.names...)
		s.Detail.ExpansionSites = append([]Site{}, trace.sites...)
		s.Detail.ExpansionDeclarations = append([]DeclarationRef{}, trace.declarations...)
	}
	return s
}

// Case construction belongs to the report model; physical coordinates arrive
// as an existing declaration receipt, preserving the same exclusive byte range.
func emptyCase() *Case {
	return &Case{Clues: []Clue{}, Clusters: []Cluster{}, Leads: []string{}, Avoid: []string{}, Receipts: []any{}, PolicyReviews: []PolicyReview{}}
}
func caseFromSource(smell, severity string, source Source) *Case {
	c := emptyCase()
	c.Smell, c.Verdict, c.Symbol = smell, strings.ToUpper(severity), source.Detail.Subject
	c.sourceIdentity(source)
	c.Receipts = append(c.Receipts, source)
	guidance(c)
	c.addPolicyReview()
	return c
}
func (c *Case) addPolicyReview() {
	if c.Smell == selectionSmell {
		c.PolicyReviews = append(c.PolicyReviews, selectionPolicy)
	}
	if c.Smell == variantSmell {
		c.PolicyReviews = append(c.PolicyReviews, variantPolicy)
	}
	if c.Smell == "cosmetic-extraction" && policyStatus == "provisional" {
		c.PolicyReviews = append(c.PolicyReviews, policy)
	}
}
func (d Detail) withoutExpansion() Detail {
	d.Expansion = []string{}
	d.ExpansionSites = []Site{}
	d.ExpansionDeclarations = []DeclarationRef{}
	return d
}

// sourceEvidenceKey uses the same private canonical encoding as receipt
// normalization. Only aggregate contributions omit their pre-summed value.
func sourceEvidenceKey(s Source) string {
	if s.AggregateContributions {
		s.Detail.Value = nil
	}
	return canonical(s)
}
func (s Source) forDeclaration(ref DeclarationRef) Source {
	s.Declaration = &ref
	return s
}
func (q Clue) forDeclaration(ref *DeclarationRef) Clue {
	q.Declaration = ref
	return q
}

// clueSupport owns one clue's explicit, de-duplicated receipt references.
type clueSupport struct {
	clue *Clue
	seen map[string]bool
}

func newClueSupport(q *Clue) *clueSupport {
	support := &clueSupport{clue: q, seen: map[string]bool{}}
	for _, key := range q.SupportingReceipts {
		support.seen[key] = true
	}
	return support
}
func (support *clueSupport) add(source Source) {
	key := sourceEvidenceKey(source)
	if !support.seen[key] {
		support.clue.SupportingReceipts = append(support.clue.SupportingReceipts, key)
		support.seen[key] = true
	}
}
func (q Clue) supportedBy(receipts []Source) Clue {
	support := newClueSupport(&q)
	for _, source := range receipts {
		support.add(source)
	}
	return q
}
func (c *Case) supportDeclaration(ref DeclarationRef, role string) {
	for _, existing := range c.SupportingDeclarations {
		if existing.Declaration == ref && existing.Role == role {
			return
		}
	}
	c.SupportingDeclarations = append(c.SupportingDeclarations, CaseDeclaration{ref, role})
}

func (c *Case) sourceDeclarations(source Source) {
	if source.Declaration != nil && !c.hasDeclaration(*source.Declaration) {
		c.supportDeclaration(*source.Declaration, "evidence")
	}
	for _, ref := range source.Detail.ExpansionDeclarations {
		c.supportDeclaration(ref, "expansion")
	}
}
func (c *Case) hasDeclaration(ref DeclarationRef) bool {
	for _, support := range c.SupportingDeclarations {
		if support.Declaration == ref {
			return true
		}
	}
	return false
}

func (c *Case) sourceIdentity(source Source) {
	c.File, c.StartLine, c.EndLine = source.File, source.StartLine, source.EndLine
	c.PrimaryDeclaration = source.Declaration
	if source.Declaration != nil {
		c.supportDeclaration(*source.Declaration, "primary")
	}
}

func (c *Case) includeDeclaration(d *declaration, role string) {
	c.Receipts = append(c.Receipts, d.declReceipt())
	c.supportDeclaration(d.ref(), role)
}

func (q Clue) forMember(ref MemberRef) Clue {
	q.Member = &ref
	return q
}
