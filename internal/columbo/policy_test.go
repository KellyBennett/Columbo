package columbo

import "strings"

// These test queries keep policy and identity checks with their case data.
func (c Case) hasSuppressedWarningPolicy() bool {
	return c.Verdict == "WARN" && c.Suppressed && len(c.PolicyReviews) == 1 && c.PolicyReviews[0] == policy
}
func (c Case) identityWithoutPolicyReviews() string {
	c.PolicyReviews = []PolicyReview{}
	id, _ := identity(c.Smell, c.File, c.Symbol, "")
	return id
}
func (t *testHarness) requireCosmeticPolicy(c Case) {
	t.Helper()
	t.require(c.hasSuppressedWarningPolicy(), c)
	t.require(c.identityWithoutPolicyReviews() == c.ID, "review changed identity")
}
func (t *testHarness) requireDisabledCosmeticSuppression(dir string, config Config) {
	t.Helper()
	config.Severity["cosmetic-extraction"] = "off"
	report := t.investigate(dir, config)
	t.require(len(report.Cases) == 0 && len(report.Warnings) == 1, report)
}
func (t *testHarness) requireReportSchema(r Report) {
	t.Helper()
	document := t.reportDocument(r)
	t.require(len(document) == 5, document)
	data, err := Serialize(r, "json")
	t.require(err == nil, err)
	t.require(!strings.Contains(string(data), `"clusters":null`) && !strings.Contains(string(data), `"policy_reviews":null`), string(data))
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
