package columbo

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func (t *testHarness) requireCosmeticPolicy(c Case) {
	t.Helper()
	require.Equal(t.T, "WARN", c.Verdict)
	require.True(t.T, c.Suppressed)
	require.Equal(t.T, []PolicyReview{policy}, c.PolicyReviews)
}

func TestPolicyReviewPreservesCase(t *testing.T) {
	(&testHarness{T: t}).TestPolicyReviewPreservesCase()
}
func (t *testHarness) TestPolicyReviewPreservesCase() {
	dir := t.fixture("package fixture\nfunc Parent(a,b int){one(a,b);two(a,b)}\n" + helpers)
	want := t.one(t.investigate(dir, cosmeticConfig()), "cosmetic-extraction")
	require.Equal(t, "FAIL", want.Verdict)
	before := canonical(want)
	got := want
	got.PolicyReviews = []PolicyReview{}
	got.addPolicyReview()
	require.Equal(t, before, canonical(got), "policy annotation changed case evidence or enforcement")
}
func (t *testHarness) requireDisabledCosmeticSuppression(dir string, config Config) {
	t.Helper()
	config.Severity["cosmetic-extraction"] = "off"
	report := t.investigate(dir, config)
	t.require(len(report.Cases) == 0 && len(report.Warnings) == 1, report)
}
func (t *testHarness) requireReportSchema(r Report) {
	t.Helper()
	db := t.snapshot(r)
	for _, table := range snapshotEvidenceTables {
		var count int
		err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_schema WHERE name = ? AND type IN ('table','view')`, table).Scan(&count)
		t.require(err == nil && count == 1, "missing relational group", table, err)
	}
	var pragma, report int
	t.require(db.QueryRow(`PRAGMA user_version`).Scan(&pragma) == nil, "schema PRAGMA")
	t.require(db.QueryRow(`SELECT schema_version FROM report`).Scan(&report) == nil, "schema report")
	t.require(pragma == SchemaVersion && report == pragma, "schema version disagreement", pragma, report)
}

var snapshotEvidenceTables = []string{
	"report", "summary", "files", "declarations", "cases", "case_declarations", "case_guidance",
	"clues", "clue_values", "dependencies", "declaration_dependencies", "dependency_receipts",
	"clusters", "cluster_members", "source_receipts", "receipt_expansion_declarations", "receipt_expansion_sites",
	"clue_receipts", "commits", "case_history", "case_history_files", "suppressions", "warnings", "policy_reviews", "case_policy_reviews",
}

func (t *testHarness) requireIdentityDelimiterBoundaries() {
	t.Helper()
	first, _ := identity("x", "a|b", "c", "")
	second, _ := identity("x", "a", "b|c", "")
	t.require(first != second, "identity delimiter collision")
}
func (t *testHarness) requireOverlapComparison(clue Clue) {
	t.Helper()
	if strings.HasSuffix(clue.Subject, ":mean") {
		t.require(clue.Limit == .75 && clue.Operator == ">=", clue)
	} else {
		t.require(clue.Limit == nil && clue.Operator == nil, clue)
	}
}

type thresholdCase struct {
	kind, smell, source string
	value               int64
}

var thresholdCases = []thresholdCase{
	{"function-lines", "long-function", "func F(){println(1)\nprintln(2)}", 2},
	{"parameters", "long-parameter-list", "func F(a,b int){}", 2},
	{"cognitive-complexity", "high-cognitive-complexity", "func F(a bool){if a {if a {}}}", 3},
	{"dependencies", "excessive-dependencies", "type A struct{};type B struct{};func F(a A,b B){}", 2},
}

func (t *testHarness) checkThresholdBoundary(scenario thresholdCase) {
	t.Helper()
	dir := t.fixture("package fixture\n" + scenario.source + "\n")
	config := quiet()
	config.Severity[scenario.smell] = "fail"
	config.Counts[scenario.kind] = scenario.value
	t.require(len(t.investigate(dir, config).Cases) == 0, "finding at threshold", scenario)
	config.Counts[scenario.kind] = scenario.value - 1
	t.one(t.investigate(dir, config), scenario.smell)
}
