package columbo

import (
	"github.com/stretchr/testify/require"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const mprPrelude = rolePrelude + `
type Kind int
const (Mail Kind=iota; Text; Push)
type Sender interface{Send(string)error}
`
const mprSelection = `var s Sender; switch k {case Mail:s=Email{};case Text:s=SMS{}};return s.Send("x")`

func mprSource(bodies ...string) string {
	source := mprPrelude
	for i, body := range bodies {
		source += "\nfunc " + string(rune('a'+i)) + "(k Kind)error{" + body + "}\n"
	}
	return source
}
func mprConfig() Config {
	c := selectionConfig()
	c.Severity[variantSmell] = "fail"
	return c
}
func mprFixture(t *testing.T, src string) Report { t.Helper(); return roleFixture(t, src, mprConfig()) }
func TestMissingRoleAcceptance(t *testing.T) {
	tests := []struct {
		name, source, confidence string
		cases                    int
	}{
		{"mpr-strong", mprSource(mprSelection, mprSelection, mprSelection), "strong", 4},
		{"mpr-stable-mapping", mprSource(mprSelection, mprSelection, mprSelection), "strong", 4},
		{"mpr-domain-join", strings.ReplaceAll(mprSource(mprSelection, mprSelection, mprSelection), "k Kind", "renamed Kind"), "", 0},
		{"mpr-partial-suc-role", mprSource(mprSelection), "partial", 1},
		{"mpr-incomplete-default", mprSource(strings.ReplaceAll(mprSelection, "case Text:", "default:")), "partial", 1},
	}
	// Renaming the discriminator must not change the canonical domain.
	tests[2].source = strings.ReplaceAll(tests[2].source, "switch k", "switch renamed")
	tests[2].confidence = "strong"
	tests[2].cases = 4
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := mprFixture(t, tt.source)
			require.Len(t, r.Correlations, 1)
			c := r.Correlations[0]
			require.Equal(t, tt.confidence, c.Confidence)
			require.Len(t, c.CaseIDs, tt.cases)
			require.Equal(t, "value:fixture.Kind", c.Domain)
			require.Equal(t, []string{"Send(string) error"}, c.Messages)
		})
	}
}
func TestMissingRoleConflict(t *testing.T) {
	src := mprSource(mprSelection, mprSelection, strings.ReplaceAll(mprSelection, "Email{}", "Legacy{}")) + `type Legacy struct{};func(Legacy)Send(string)error{return nil}`
	r := mprFixture(t, src)
	require.Len(t, r.Correlations, 1)
	require.Equal(t, "partial", r.Correlations[0].Confidence)
	require.Contains(t, r.Correlations[0].Diagnosis, "incompatible")
	require.Len(t, r.Correlations[0].CaseIDs, 4)
}
func TestMissingRoleSubset(t *testing.T) {
	extra := strings.ReplaceAll(mprSelection, "case Text:s=SMS{}", "case Text:s=SMS{};case Push:s=Mobile{}")
	r := mprFixture(t, mprSource(mprSelection, extra, mprSelection)+`type Mobile struct{};func(Mobile)Send(string)error{return nil}`)
	require.Len(t, r.Correlations, 1)
	require.Equal(t, "strong", r.Correlations[0].Confidence)
	require.Len(t, r.Correlations[0].Players, 3)
}
func TestMissingRoleIdentityAndEnforcement(t *testing.T) {
	src := mprSource(mprSelection, mprSelection, mprSelection)
	r := mprFixture(t, src)
	before := r.Correlations[0].ID
	summary := r.Summary
	slices.Reverse(r.Cases)
	slices.Reverse(r.Selections)
	slices.Reverse(r.Roles)
	r.Cases[0].Suppressed = true
	r.Cases[0].Verdict = "WARN"
	r.correlate()
	require.Equal(t, before, r.Correlations[0].ID)
	require.Equal(t, summary, r.Summary)
	moved := mprFixture(t, strings.ReplaceAll(src, "\n", "\n\n"))
	require.Equal(t, before, moved.Correlations[0].ID)
	withoutRole := r
	withoutRole.Roles = nil
	withoutRole.correlate()
	require.Len(t, withoutRole.Correlations, 1)
	require.Equal(t, "partial", withoutRole.Correlations[0].Confidence)
	require.Contains(t, withoutRole.Correlations[0].Diagnosis, "cannot establish a common")
}
func TestMissingRoleNoProximityJoin(t *testing.T) {
	unrelated := `switch other {case Mail:println(1);case Text:println(2)};`
	src := mprSource(mprSelection) + `func x(other Kind){` + unrelated + `};func y(other Kind){` + unrelated + `};func z(other Kind){` + unrelated + `}`
	src = strings.Replace(src, "func a(k Kind)", "type Other int;const(One Other=iota;Two);func a(k Other)", 1)
	src = strings.Replace(src, "switch k {case Mail:", "switch k {case One:", 1)
	src = strings.Replace(src, "case Text:s=SMS{}", "case Two:s=SMS{}", 1)
	r := mprFixture(t, src)
	require.Len(t, r.Correlations, 1)
	require.Len(t, r.Correlations[0].CaseIDs, 1)
	require.Equal(t, "partial", r.Correlations[0].Confidence)
}
func TestMissingRoleFactoryAndInterface(t *testing.T) {
	r := mprFixture(t, mprSource(mprSelection, mprSelection, mprSelection))
	require.Equal(t, "strong", r.Correlations[0].Confidence) // named Sender already exists
	source := mprPrelude + `func factory(k Kind)Sender{var s Sender;switch k{case Mail:s=Email{};case Text:s=SMS{}};return s}
 func a(k Kind)error{return factory(k).Send("x")};func b(k Kind)error{return factory(k).Send("x")};func c(k Kind)error{return factory(k).Send("x")}`
	after := mprFixture(t, source)
	require.Empty(t, after.Cases)
	require.Empty(t, after.Correlations)
}
func TestMissingRoleSnapshot(t *testing.T) {
	r := mprFixture(t, mprSource(mprSelection, mprSelection, mprSelection))
	r.Cases[0].Suppressed = true
	path := filepath.Join(t.TempDir(), "correlations.sqlite")
	require.NoError(t, WriteSnapshot(path, r, "test"))
	reader, err := OpenSnapshot(path)
	require.NoError(t, err)
	defer reader.Close()
	text, code, err := RenderSnapshot(reader.Queries(), path)
	require.NoError(t, err)
	require.Equal(t, 1, code)
	require.Contains(t, string(text), "CORRELATION MPR-")
	require.Contains(t, string(text), "SUPPRESSED (original: FAIL)")
	require.Contains(t, string(text), "MPR-001 (provisional)")
	require.Contains(t, string(text), "Mapping: value:fixture.Kind=0 → fixture.Email")
	r.Correlations = nil
	second := filepath.Join(t.TempDir(), "without.sqlite")
	require.NoError(t, WriteSnapshot(second, r, "test"))
	other, err := OpenSnapshot(second)
	require.NoError(t, err)
	defer other.Close()
	_, withoutCode, err := RenderSnapshot(other.Queries(), second)
	require.NoError(t, err)
	require.Equal(t, code, withoutCode)
}

func TestMissingRoleSnapshotRejectsForeignEvidence(t *testing.T) {
	for _, kind := range []string{"case", "role", "receipt"} {
		t.Run(kind, func(t *testing.T) {
			r := mprFixture(t, mprSource(mprSelection, mprSelection, mprSelection))
			c := &r.Correlations[0]
			switch kind {
			case "case":
				c.CaseIDs = append(c.CaseIDs, "C-other-snapshot")
			case "role":
				c.RoleID = "R-other-snapshot"
			case "receipt":
				c.Evidence[0].ReceiptKey = "receipt-from-other-snapshot"
			}
			require.Error(t, WriteSnapshot(filepath.Join(t.TempDir(), "invalid.sqlite"), r, "test"))
		})
	}
}
func TestMissingRoleEqualityChain(t *testing.T) {
	body := `var s Sender;if k==Mail {s=Email{}}else if k==Text{s=SMS{}};return s.Send("x")`
	r := mprFixture(t, mprSource(body, body, body))
	require.Len(t, r.Correlations, 1)
	require.Equal(t, "strong", r.Correlations[0].Confidence)
}
func TestMissingRoleUnmappedOrigin(t *testing.T) {
	body := `var s Sender=Email{};switch k{case Mail:s=Email{};case Text:s=SMS{}};return s.Send("x")`
	r := mprFixture(t, mprSource(body, body, body))
	require.Len(t, r.Correlations, 1)
	require.Equal(t, "partial", r.Correlations[0].Confidence)
	require.Contains(t, r.Correlations[0].Diagnosis, "complete variant-to-player mapping")
}
