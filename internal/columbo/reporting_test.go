package columbo

import (
	"bytes"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func (t *testHarness) sqlStrings(db *sql.DB, query string, args ...any) []string {
	t.Helper()
	rows, err := db.Query(query, args...)
	t.noError(err)
	defer rows.Close()
	return (stringRows{test: t, rows: rows}).values()
}

type stringRows struct {
	test *testHarness
	rows *sql.Rows
}

func (cursor stringRows) values() []string {
	values := []string{}
	for cursor.rows.Next() {
		var value string
		cursor.test.noError(cursor.rows.Scan(&value))
		values = append(values, value)
	}
	cursor.test.noError(cursor.rows.Err())
	return values
}
func (t *testHarness) sqlCount(db *sql.DB, query string, args ...any) int {
	t.Helper()
	var count int
	t.noError(db.QueryRow(query, args...).Scan(&count))
	return count
}
func (t *testHarness) allSmellsReport() Report {
	t.Helper()
	dir, err := filepath.Abs("testdata/all")
	t.noError(err)
	config := Defaults()
	config.History = false
	return t.investigate(dir, config)
}

func TestSnapshotCanonicalCaseOrder(t *testing.T) {
	(&testHarness{T: t}).TestSnapshotCanonicalCaseOrder()
}
func (h *testHarness) TestSnapshotCanonicalCaseOrder() {
	db := h.snapshot(h.allSmellsReport())
	got := h.sqlStrings(db, `SELECT c.id || '|' || c.smell || '|' || d.symbol FROM cases c JOIN declarations d ON d.id = c.primary_declaration_id ORDER BY c.ordinal`)
	h.equal(canonicalAllCases, got)
}

// These IDs and their order were pinned before replacing the reporting encoder.
var canonicalAllCases = []string{
	"C-b10bccb46b|long-function|fixture.LongFunction",
	"C-ad03ea5957|long-parameter-list|fixture.Parameters",
	"C-3d6d2adfd3|high-cognitive-complexity|fixture.Complex",
	"C-4fb5041cb7|excessive-dependencies|fixture.Dependencies",
	"C-cf8ec1ee12|long-parameter-list|fixture.Dependencies",
	"C-4063b2b57c|feature-envy|fixture.(Own).Envy",
	"C-47c5c497d6|data-clump|fixture.ClumpA",
	"C-5522b4859c|cosmetic-extraction|fixture.Parent",
}

func TestSnapshotLogicalDeterminism(t *testing.T) {
	(&testHarness{T: t}).TestSnapshotLogicalDeterminism()
}
func (h *testHarness) TestSnapshotLogicalDeterminism() {
	first := h.logicalRows(h.snapshot(h.allSmellsReport()))
	second := h.logicalRows(h.snapshot(h.allSmellsReport()))
	h.equal(first, second, "same evidence must produce identical logical rows and ordinals")
}
func TestEmptySnapshotGoldens(t *testing.T) { (&testHarness{T: t}).TestEmptySnapshotGoldens() }
func (h *testHarness) TestEmptySnapshotGoldens() {
	report := h.investigate(h.fixture("package fixture\nfunc F(){}\n"), quiet())
	h.empty(report.Cases)
	h.snapshotGoldens(report, filepath.Join("testdata", "empty"))
}
func TestSnapshotContributionReconciliation(t *testing.T) {
	(&testHarness{T: t}).TestSnapshotContributionReconciliation()
}
func (h *testHarness) TestSnapshotContributionReconciliation() {
	db := h.snapshot(h.allSmellsReport())
	h.checkStoredContributions(db)
	h.equal(6, h.sqlCount(db, `SELECT COUNT(*) FROM clues WHERE kind IN ('function-lines','cognitive-complexity','expanded-lines','expanded-complexity')`), "ordinary and original/expanded cosmetic metrics")
}

const contributionReconciliationQuery = `SELECT q.case_id, q.kind, q.numeric_value, COALESCE(SUM(r.value), 0)
FROM clues q
LEFT JOIN clue_receipts link ON link.clue_id = q.id AND link.case_id = q.case_id
LEFT JOIN source_receipts r ON r.id = link.receipt_id AND r.case_id = link.case_id AND r.kind = 'metric-contribution'
WHERE q.kind IN ('function-lines', 'cognitive-complexity', 'expanded-lines', 'expanded-complexity')
GROUP BY q.id ORDER BY q.case_id, q.ordinal`

func TestSnapshotScoredDependenciesAndInventory(t *testing.T) {
	(&testHarness{T: t}).TestSnapshotScoredDependenciesAndInventory()
}
func (h *testHarness) TestSnapshotScoredDependenciesAndInventory() {
	config := dependencyConfig()
	config.Counts["dependencies"] = 1
	report := h.investigate(h.fixture(dependencyScenarios[0].source), config)
	db := h.snapshot(report)
	h.equal(3, h.sqlCount(db, `SELECT COUNT(*) FROM declaration_dependencies WHERE scored = 1`))
	inventory := h.sqlStrings(db, inventoryDependenciesQuery)
	h.contains(inventory, "type:error")
	h.contains(inventory, "interface:interface{}")
	h.contains(inventory, "package:go/parser")
	h.zero(h.sqlCount(db, `SELECT COUNT(*) FROM source_receipts r JOIN dependency_receipts dr ON dr.receipt_id=r.id JOIN declaration_dependencies dd ON dd.declaration_id=dr.declaration_id AND dd.dependency_id=dr.dependency_id WHERE r.kind='dependency-inventory' AND dd.scored<>0`))
}

const inventoryDependenciesQuery = `SELECT DISTINCT dep.identity FROM declaration_dependencies dd
JOIN dependencies dep ON dep.id=dd.dependency_id JOIN dependency_receipts dr ON dr.declaration_id=dd.declaration_id AND dr.dependency_id=dd.dependency_id
JOIN source_receipts r ON r.id=dr.receipt_id WHERE dd.scored=0 AND r.kind='dependency-inventory' ORDER BY dep.identity`

func TestSnapshotClumpMultiplicity(t *testing.T) {
	(&testHarness{T: t}).TestSnapshotClumpMultiplicity()
}
func (h *testHarness) TestSnapshotClumpMultiplicity() {
	source := "package fixture\nfunc A(a,b int,c string){}\nfunc B(a,b int,c string){}\nfunc C(a,b int,c string){}\n"
	config := quiet()
	config.Severity["data-clump"] = "fail"
	db := h.snapshot(h.investigate(h.fixture(source), config))
	types := h.sqlStrings(db, `SELECT v.value FROM clue_values v JOIN clues q ON q.id=v.clue_id WHERE q.kind='clump-types' ORDER BY v.ordinal`)
	h.equal([]string{"int", "int", "string"}, types)
	support := h.sqlStrings(db, `SELECT DISTINCT d.symbol FROM case_declarations cd JOIN declarations d ON d.id=cd.declaration_id ORDER BY d.symbol`)
	h.equal([]string{"fixture.A", "fixture.B", "fixture.C"}, support)
}
func TestSnapshotEmptyDependencySets(t *testing.T) {
	(&testHarness{T: t}).TestSnapshotEmptyDependencySets()
}
func (h *testHarness) TestSnapshotEmptyDependencySets() {
	source := "package fixture\nfunc Parent(a,b int){one(a,b);two(a,b)}\n" + helpers
	db := h.snapshot(h.investigate(h.fixture(source), cosmeticConfig()))
	h.equal(2, h.sqlCount(db, `SELECT COUNT(*) FROM clues q WHERE q.kind='dependency-set' AND q.value_type='list' AND q.numeric_value IS NULL AND NOT EXISTS(SELECT 1 FROM clue_values v WHERE v.clue_id=q.id)`))
}
func TestSnapshotInitAndBlankIdentities(t *testing.T) {
	(&testHarness{T: t}).TestSnapshotInitAndBlankIdentities()
}
func (h *testHarness) TestSnapshotInitAndBlankIdentities() {
	source := "package fixture\nfunc init(){println(1)\nprintln(2)}\nfunc init(){println(1)\nprintln(2)}\nfunc _(a,b,c,d,e int){}\nfunc _(a,b,c,d,e int){}\n"
	config := longParameterConfig()
	config.Severity["long-function"] = "fail"
	config.Counts["function-lines"] = 1
	db := h.snapshot(h.investigate(h.fixture(source), config))
	symbols := h.sqlStrings(db, `SELECT d.symbol FROM cases c JOIN declarations d ON d.id=c.primary_declaration_id ORDER BY c.ordinal`)
	h.equal([]string{"fixture.init#1", "fixture.init#2", "fixture._#1", "fixture._#2"}, symbols)
	h.equal(4, h.sqlCount(db, `SELECT COUNT(DISTINCT id) FROM cases`))
}

func TestCLISnapshotPublication(t *testing.T) {
	(&testHarness{T: t}).checkCLISnapshotPublication()
}

const cliFindingSource = "package fixture\nfunc F(a,b,c,d,e int){}\n"

// commandResult separates invocation and observable delivery from filesystem
// assertions, so CLI cases share the same capture and failure contract.
type commandResult struct {
	code   int
	stdout []byte
	stderr string
}

func (t *testHarness) runCLI(dir string, args ...string) commandResult {
	var stdout, stderr bytes.Buffer
	code := Run(args, Invocation{Dir: dir, Version: "acceptance", Stdout: &stdout, Stderr: &stderr})
	return commandResult{code, stdout.Bytes(), stderr.String()}
}
func (result commandResult) requireFailure(t *testHarness) {
	t.equal(2, result.code)
	t.empty(result.stdout)
	t.notEmpty(result.stderr)
}
func (t *testHarness) checkCLISnapshotPublication() {
	dir := t.fixture(cliFindingSource)
	for _, name := range []string{"", "chosen.sqlite"} {
		t.checkCLIOutputSelection(dir, name)
	}
}
func (t *testHarness) checkCLIOutputSelection(dir, name string) {
	args := []string{"--no-history"}
	if name != "" {
		args = append(args, "--output="+name)
	}
	result := t.runCLI(dir, args...)
	t.equal(1, result.code, result.stderr)
	path := filepath.Join(dir, name)
	if name == "" {
		path = t.onlyDefaultSnapshot(dir)
	}
	t.checkPublishedSummary(path, result.stdout)
	t.empty(result.stderr)
}
func (t *testHarness) defaultSnapshots(dir string) []string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(dir, "columbo-*.sqlite"))
	t.noError(err)
	for _, path := range paths {
		require.Regexp(t.T, `^columbo-[A-Z2-7]{26}\.sqlite$`, filepath.Base(path))
	}
	return paths
}
func (t *testHarness) onlyDefaultSnapshot(dir string) string {
	t.Helper()
	paths := t.defaultSnapshots(dir)
	t.length(paths, 1)
	return paths[0]
}
func TestCLIRepeatedRunsCreateFreshSnapshots(t *testing.T) {
	(&testHarness{T: t}).checkRepeatedRunsCreateFreshSnapshots()
}
func (t *testHarness) checkRepeatedRunsCreateFreshSnapshots() {
	dir := t.fixture(cliFindingSource)
	first := t.runCLI(dir, "--no-history")
	t.equal(1, first.code, first.stderr)
	previous := t.onlyDefaultSnapshot(dir)
	before := t.read(previous)
	second := t.runCLI(dir, "--no-history")
	t.equal(1, second.code, second.stderr)
	t.checkNewDefaultSnapshot(dir, previous, second.stdout)
	t.equal(before, t.read(previous), "earlier snapshots must remain unchanged")
	t.checkPublishedSummary(previous, first.stdout)
}
func (t *testHarness) checkNewDefaultSnapshot(dir, previous string, stdout []byte) {
	paths := t.defaultSnapshots(dir)
	t.length(paths, 2)
	for _, path := range paths {
		if path != previous {
			t.checkPublishedSummary(path, stdout)
		}
	}
}
func TestCLIExplicitExistingSnapshotRefused(t *testing.T) {
	(&testHarness{T: t}).checkExplicitExistingSnapshotRefused()
}
func (t *testHarness) checkExplicitExistingSnapshotRefused() {
	dir := t.fixture(cliFindingSource)
	path := filepath.Join(dir, "chosen.sqlite")
	t.equal(1, t.runCLI(dir, "--no-history", "--output="+path).code)
	before := t.read(path)
	result := t.runCLI(dir, "--no-history", "--output="+path)
	result.requireFailure(t)
	t.contains(result.stderr, "choose a fresh output path")
	t.equal(before, t.read(path))
}
func (t *testHarness) checkPublishedSummary(path string, stdout []byte) {
	db, err := openTestSnapshot(path)
	t.noError(err)
	defer db.Close()
	summary, exitCode, err := RenderSnapshot(testDatabaseQueries(db), path)
	t.noError(err)
	t.equal(1, exitCode)
	t.equal(summary, stdout)
	t.contains(string(stdout), path)
	t.checkPublishedManifest(db)
}
func (t *testHarness) checkPublishedManifest(db *sql.DB) {
	t.equal([]string{"acceptance"}, t.sqlStrings(db, `SELECT columbo_version FROM report`))
	_, err := db.Exec(`DELETE FROM cases`)
	t.hasError(err, "snapshot readers must be read-only")
}
func TestCLIPrepublicationFailure(t *testing.T) { (&testHarness{T: t}).checkCLIPrepublicationFailure() }
func (t *testHarness) checkCLIPrepublicationFailure() {
	dir := t.fixture(cliFindingSource)
	path := filepath.Join(dir, "reserved.sqlite")
	original := "unrelated data must remain untouched"
	t.write(dir, "reserved.sqlite", original)
	t.runCLI(dir, "--no-history", "--output="+path).requireFailure(t)
	t.equal([]byte(original), t.read(path))
}
func TestCLISummaryWriteFailureKeepsSnapshot(t *testing.T) {
	(&testHarness{T: t}).checkCLISummaryWriteFailure()
}
func (t *testHarness) checkCLISummaryWriteFailure() {
	dir := t.fixture(cliFindingSource)
	t.checkFailingSummarySink(dir)
	db, err := openTestSnapshot(filepath.Join(dir, "summary.sqlite"))
	t.noError(err, "summary delivery failure occurs after complete publication")
	defer db.Close()
	t.equal(1, t.sqlCount(db, `SELECT failed FROM summary`))
	t.equal([]string{"dev"}, t.sqlStrings(db, `SELECT columbo_version FROM report`))
}
func (t *testHarness) checkFailingSummarySink(dir string) {
	var stderr bytes.Buffer
	sink := &brokenSink{}
	code := Run([]string{"--no-history", "--output=summary.sqlite"}, Invocation{Dir: dir, Stdout: sink, Stderr: &stderr})
	t.equal(2, code)
	t.equal(3, sink.accepted)
	t.contains(stderr.String(), "sink failure")
}
func TestCLIFailureLeavesPriorSnapshot(t *testing.T) {
	(&testHarness{T: t}).checkCLIFailureLeavesPriorSnapshot()
}
func (t *testHarness) checkCLIFailureLeavesPriorSnapshot() {
	dir := t.fixture(cliFindingSource)
	t.equal(1, t.runCLI(dir, "--no-history").code)
	path := t.onlyDefaultSnapshot(dir)
	before := t.read(path)
	t.write(dir, "bad.go", "package fixture\nvar X=missing\n")
	t.runCLI(dir, "--no-history").requireFailure(t)
	t.equal(before, t.read(path))
	t.length(t.defaultSnapshots(dir), 1)
}

func TestStoredSummaryWithoutAnalysis(t *testing.T) {
	(&testHarness{T: t}).checkStoredSummaryWithoutAnalysis()
}
func (t *testHarness) checkStoredSummaryWithoutAnalysis() {
	// This report is a saved-result fixture, with no source files or analyzer.
	// Deliberately contradictory comparisons and stale in-memory totals prove
	// that the reader trusts stored rule outcomes rather than rerunning rules.
	report := savedSummaryFixture()
	report.Summary = Summary{Failed: 999, Warned: 999, Suppressed: 999}
	db := t.snapshot(report)
	summary, failed, err := RenderSnapshot(testDatabaseQueries(db), "saved.sqlite")
	t.noError(err)
	t.equal(1, failed)
	t.checkSavedSummaryText(summary)
}
func (t *testHarness) checkSavedSummaryText(summary []byte) {
	for _, text := range []string{"FAIL", "WARN", "SUPPRESSED", "1 (limit > 99)", "3 (limit > 4)", "accepted policy fixture", policy.Note, policy.ReviewPrompt, historyWarning, "saved.sqlite"} {
		t.contains(string(summary), text)
	}
	t.notContains(string(summary), "999")
	t.notContains(string(summary), "full receipt only")
	t.goldenBytes(filepath.Join("testdata", "saved-summary.golden"), summary)
}

// savedSummaryBuilder owns a completed report, while savedDeclaration owns its
// physical identity and the cases that use that source declaration.
type savedSummaryBuilder struct{ report Report }
type savedDeclaration struct {
	ref    DeclarationRef
	source Source
}

func savedSummaryFixture() Report {
	builder := savedSummaryBuilder{report: Report{Version: 1, Warnings: []Warning{{Code: "history-unavailable", Message: historyWarning}}}}
	for i, verdict := range []string{"FAIL", "WARN", "WARN"} {
		builder.add(i, verdict)
	}
	builder.report.finish()
	return builder.report
}
func savedSummaryDeclaration(i int) savedDeclaration {
	ref := DeclarationRef{File: "saved.go", Symbol: fmt.Sprintf("saved.F%d", i)}
	source := Source{Kind: "declaration", File: ref.File, StartLine: i + 1, EndLine: i + 1, StartOffset: i * 10, EndOffset: i*10 + 9, Declaration: &ref, Detail: Detail{Subject: ref.Symbol}}
	return savedDeclaration{ref, source}
}
func (d savedDeclaration) evidence() DeclarationEvidence {
	return DeclarationEvidence{Ref: d.ref, Source: d.source}
}
func (d savedDeclaration) parameterCase(verdict string, value int) Case {
	id, _ := identity("long-parameter-list", d.ref.File, d.ref.Symbol, "")
	clue := metric("parameters", d.ref.Symbol, value).compare(99, ">").forDeclaration(&d.ref)
	return Case{ID: id, Smell: "long-parameter-list", Verdict: verdict, Symbol: d.ref.Symbol, File: d.ref.File, StartLine: d.source.StartLine, EndLine: d.source.EndLine, PrimaryDeclaration: &d.ref, Clues: []Clue{clue}, Receipts: []any{d.source}, Why: "full receipt only", Diagnosis: "stored diagnosis", Leads: []string{"stored lead"}}
}
func (d savedDeclaration) policyCase() Case {
	c := d.parameterCase("WARN", 3)
	c.Smell = "cosmetic-extraction"
	c.ID, _ = identity(c.Smell, d.ref.File, d.ref.Symbol, "")
	c.Clues = []Clue{metric("expanded-lines", d.ref.Symbol, 3).compare(4, ">").forDeclaration(&d.ref)}
	c.Suppressed = true
	c.PolicyReviews = []PolicyReview{policy}
	return c
}
func (builder *savedSummaryBuilder) add(i int, verdict string) {
	d := savedSummaryDeclaration(i)
	builder.report.Declarations = append(builder.report.Declarations, d.evidence())
	c := d.parameterCase(verdict, i+1)
	if i == 2 {
		c = d.policyCase()
		builder.suppress(c)
	}
	builder.report.Cases = append(builder.report.Cases, c)
}
func (builder *savedSummaryBuilder) suppress(c Case) {
	builder.report.Suppressions = append(builder.report.Suppressions, c.fixtureSuppression())
}

func (c Case) fixtureSuppression() Suppression {
	return Suppression{Smell: c.Smell, Symbol: c.Symbol, File: c.File, Line: c.StartLine, Justification: "accepted policy fixture", Applied: true, CaseID: c.ID}
}

func TestSnapshotHistoryCaseSubsets(t *testing.T) {
	(&testHarness{T: t}).checkSnapshotHistoryCaseSubsets()
}
func (t *testHarness) checkSnapshotHistoryCaseSubsets() {
	fixture := t.caseSubsetHistoryFixture()
	config := longParameterConfig()
	config.History = true
	report := t.investigate(fixture.dir, config)
	db := t.snapshot(report)
	for _, c := range report.Cases {
		t.checkStoredCaseHistory(db, c)
	}
}
func (t *testHarness) caseSubsetHistoryFixture() gitFixture {
	fixture := t.committedFixture("package fixture\nfunc F(a,b,c,d,e int){}\n")
	t.write(fixture.dir, "other.go", "package fixture\nfunc G(a,b,c,d,e int){}\n")
	fixture.commit(t, "second file")
	t.write(fixture.dir, "source.go", "package fixture\nfunc F(a,b,c,d,e int){println(a)}\n")
	t.write(fixture.dir, "other.go", "package fixture\nfunc G(a,b,c,d,e int){println(a)}\n")
	fixture.commit(t, "both files")
	return fixture
}
func (fixture gitFixture) commit(t *testHarness, message string) {
	fixture.run(t, "add", ".")
	fixture.run(t, "commit", "-m", message)
}
func (t *testHarness) checkStoredCaseHistory(db *sql.DB, c Case) {
	expected := []string{}
	for i, history := range historyReceipts(c) {
		t.length(history.Files, 1, "case keeps exact matching file subset")
		expected = append(expected, fmt.Sprintf("%d|%s|%d|%s", i, history.Commit, history.CommittedAt, history.Files[0]))
	}
	got := t.sqlStrings(db, `SELECT ch.ordinal || '|' || ch.commit_hash || '|' || co.committed_at || '|' || f.path FROM case_history ch JOIN commits co ON co.hash=ch.commit_hash JOIN case_history_files hf ON hf.case_id=ch.case_id AND hf.history_ordinal=ch.ordinal JOIN files f ON f.id=hf.file_id WHERE ch.case_id=? ORDER BY ch.ordinal,hf.ordinal`, c.ID)
	t.equal(expected, got)
	t.length(got, 2)
}

func TestSnapshotClusterAndMemberOrder(t *testing.T) {
	(&testHarness{T: t}).TestSnapshotClusterAndMemberOrder()
}
func (h *testHarness) TestSnapshotClusterAndMemberOrder() {
	report := h.investigate(h.fixture(orderedClusterSource()), cosmeticConfig())
	db := h.snapshot(report)
	for _, c := range report.Cases {
		h.equal(orderedClusterRows(c), h.sqlStrings(db, clusterOrderQuery, c.ID))
	}
	h.zero(h.sqlCount(db, `SELECT COUNT(*) FROM clues WHERE kind='dependency-overlap' AND operator IS NULL AND (pair_left_member_id IS NULL OR pair_right_member_id IS NULL OR cluster_id IS NULL)`))
	h.zero(h.sqlCount(db, `SELECT COUNT(*) FROM clues WHERE (kind='forwarded-input-set' OR (kind='parameter-overlap' AND operator IS NULL)) AND (member_id IS NULL OR cluster_id IS NULL)`))
}
func orderedClusterRows(c Case) []string {
	want := []string{}
	for ci, cluster := range c.Clusters {
		for mi, member := range cluster.Members {
			want = append(want, fmt.Sprintf("%d|%s|%s|%d|%s|%d", ci, cluster.Key, cluster.Owner, mi, member.Helper, member.CallOffset))
		}
	}
	return want
}

const clusterOrderQuery = `SELECT cl.ordinal || '|' || cl.cluster_key || '|' || owner.symbol || '|' || m.ordinal || '|' || helper.symbol || '|' || m.call_offset
FROM clusters cl JOIN declarations owner ON owner.id=cl.owner_declaration_id JOIN cluster_members m ON m.cluster_id=cl.id
JOIN declarations helper ON helper.id=m.helper_declaration_id WHERE cl.case_id=? ORDER BY cl.ordinal,m.ordinal`

func TestSnapshotCaseScopedReachableClusters(t *testing.T) {
	(&testHarness{T: t}).TestSnapshotCaseScopedReachableClusters()
}
func (h *testHarness) TestSnapshotCaseScopedReachableClusters() {
	source := "package fixture\nfunc Ancestor(a,b int){middle(a,b)}\nfunc middle(x,y int){one(x,y);two(x,y)}\n" + helpers
	report := h.investigate(h.fixture(source), cosmeticConfig())
	h.length(report.Cases, 2)
	db := h.snapshot(report)
	h.equal(2, h.sqlCount(db, `SELECT COUNT(*) FROM clusters`))
	h.equal(1, h.sqlCount(db, `SELECT COUNT(DISTINCT cluster_key) FROM clusters`))
	h.equal(4, h.sqlCount(db, `SELECT COUNT(*) FROM cluster_members`))
	h.equal([]string{"fixture.middle", "fixture.middle"}, h.sqlStrings(db, `SELECT d.symbol FROM clusters cl JOIN declarations d ON d.id=cl.owner_declaration_id ORDER BY cl.case_id,cl.ordinal`))
}

const distinctExpansionSource = "package fixture\nfunc Parent(a,b int){outer(inner(a,b),b);two(a,b)}\nfunc outer(a,b int){_=func(){if a>0{println(a)}}}\nfunc inner(a,b int)int{if a>0{return a};return b}\n"

func TestSnapshotDistinctExpansionCopies(t *testing.T) {
	(&testHarness{T: t}).TestSnapshotDistinctExpansionCopies()
}
func (h *testHarness) TestSnapshotDistinctExpansionCopies() {
	report := h.analyzedExpansionReport()
	db := h.snapshot(report)
	h.checkCaseExpansions(db, h.one(report, "cosmetic-extraction"))
	h.checkStoredContributions(db)
}
func (h *testHarness) analyzedExpansionReport() Report {
	source := distinctExpansionSource + strings.ReplaceAll(helpers, "func one(a,b int)", "func unused(a,b int)")
	config := cosmeticConfig()
	config.Counts["function-lines"] = 1
	return h.investigate(h.fixture(source), config)
}
func (h *testHarness) checkCaseExpansions(db *sql.DB, c Case) {
	copies := 0
	for ordinal, receipt := range c.Receipts {
		if source, ok := receipt.(Source); ok && len(source.Detail.Expansion) > 0 {
			h.checkStoredExpansion(db, c.ID, ordinal, source)
			copies++
		}
	}
	h.greater(copies, 0, "expanded contribution evidence remains stored")
}
func (t *testHarness) checkStoredExpansion(db *sql.DB, caseID string, ordinal int, source Source) {
	names := t.sqlStrings(db, `SELECT e.symbol FROM receipt_expansion_declarations e JOIN source_receipts r ON r.id=e.receipt_id WHERE r.case_id=? AND r.ordinal=? ORDER BY e.ordinal`, caseID, ordinal)
	t.equal(source.Detail.Expansion, names)
	expectedSites := []string{}
	for _, site := range source.Detail.ExpansionSites {
		expectedSites = append(expectedSites, fmt.Sprintf("%s|%d", site.File, site.CallOffset))
	}
	sites := t.sqlStrings(db, `SELECT f.path || '|' || e.call_offset FROM receipt_expansion_sites e JOIN source_receipts r ON r.id=e.receipt_id JOIN files f ON f.id=e.file_id WHERE r.case_id=? AND r.ordinal=? ORDER BY e.ordinal`, caseID, ordinal)
	t.equal(expectedSites, sites)
	t.length(sites, len(names)-1)
}
func (t *testHarness) checkStoredContributions(db *sql.DB) {
	rows, err := db.Query(contributionReconciliationQuery)
	t.noError(err)
	defer rows.Close()
	for rows.Next() {
		var id, kind string
		var value, sum int
		t.noError(rows.Scan(&id, &kind, &value, &sum))
		t.equal(value, sum, "%s %s copied contributions", id, kind)
	}
	t.noError(rows.Err())
}

func TestCLIRejectsStdoutDestinations(t *testing.T) {
	(&testHarness{T: t}).TestCLIRejectsStdoutDestinations()
}
func (h *testHarness) TestCLIRejectsStdoutDestinations() {
	dir := h.fixture("package fixture\nfunc F(a,b,c,d,e int){}\n")
	for _, path := range []string{"-", "/dev/stdout", "/proc/self/fd/1"} {
		var stdout, stderr bytes.Buffer
		code := Run([]string{"--no-history", "--output=" + path}, Invocation{Dir: dir, Stdout: &stdout, Stderr: &stderr})
		h.equal(2, code, path)
		h.empty(stdout.String(), path)
		h.notEmpty(stderr.String(), path)
	}
}

func TestSavedSnapshotDistinctCopyPaths(t *testing.T) {
	(&testHarness{T: t}).TestSavedSnapshotDistinctCopyPaths()
}
func (h *testHarness) TestSavedSnapshotDistinctCopyPaths() {
	report := savedCopyFixture()
	db := h.snapshot(report)
	h.equal(2, h.sqlCount(db, `SELECT COUNT(*) FROM source_receipts WHERE subject='expanded-lines'`))
	h.equal(2, h.sqlCount(db, `SELECT COUNT(*) FROM source_receipts WHERE subject='expanded-complexity'`), "aggregation cannot cross copy-site paths")
	for ordinal, receipt := range report.Cases[0].Receipts {
		if source := receipt.(Source); len(source.Detail.Expansion) > 0 {
			h.checkStoredExpansion(db, report.Cases[0].ID, ordinal, source)
		}
	}
	h.checkStoredContributions(db)
}

// Two copy paths intentionally share their declaration chain and physical
// range. Equal complexity events aggregate only within the same complete path.
type savedCopyBuilder struct {
	root, helper savedDeclaration
	copies       []Source
}

func savedCopyFixture() Report {
	builder := newSavedCopyBuilder()
	report := Report{Version: 1, Cases: []Case{builder.rootCase()}, Declarations: []DeclarationEvidence{builder.root.evidence(), builder.helper.evidence()}}
	report.finish()
	return report
}
func newSavedCopyBuilder() savedCopyBuilder {
	root := DeclarationRef{File: "copies.go", Symbol: "saved.Parent"}
	helper := DeclarationRef{File: "copies.go", Symbol: "saved.helper"}
	primary := Source{Kind: "declaration", File: root.File, StartLine: 1, EndLine: 1, EndOffset: 40, Declaration: &root, Detail: Detail{Subject: root.Symbol}}
	help := Source{Kind: "declaration", File: helper.File, StartLine: 2, EndLine: 3, StartOffset: 41, EndOffset: 70, Declaration: &helper, Detail: Detail{Subject: helper.Symbol}}
	return savedCopyBuilder{root: savedDeclaration{root, primary}, helper: savedDeclaration{helper, help}, copies: savedCopyReceipts(root, helper)}
}
func (builder savedCopyBuilder) rootCase() Case {
	root := builder.root.ref
	id, _ := identity("cosmetic-extraction", root.File, root.Symbol, "")
	lines := metric("expanded-lines", root.Symbol, 2).compare(1, ">").forDeclaration(&root).supportedBy(builder.copies[:2])
	complexity := metric("expanded-complexity", root.Symbol, 6).compare(1, ">").forDeclaration(&root).supportedBy(builder.copies[2:])
	c := Case{ID: id, Smell: "cosmetic-extraction", Verdict: "FAIL", Symbol: root.Symbol, File: root.File, StartLine: 1, EndLine: 1, PrimaryDeclaration: &root, Clues: []Clue{lines, complexity}, Receipts: []any{builder.root.source}}
	for _, source := range builder.copies {
		c.Receipts = append(c.Receipts, source)
	}
	return c
}
func savedCopyReceipts(root, helper DeclarationRef) []Source {
	copies := []Source{}
	for _, offset := range []int{10, 20} {
		detail := Detail{Subject: "expanded-lines", Value: 1, Expansion: []string{root.Symbol, helper.Symbol}, ExpansionDeclarations: []DeclarationRef{root, helper}, ExpansionSites: []Site{{File: root.File, CallOffset: offset, Owner: &root}}}
		copies = append(copies, Source{Kind: "metric-contribution", File: helper.File, StartLine: 3, EndLine: 3, StartOffset: 50, EndOffset: 60, Detail: detail, Declaration: &helper})
	}
	for i, offset := range []int{10, 10, 20} {
		detail := Detail{Subject: "expanded-complexity", Value: i + 1, Nesting: 0, Expansion: []string{root.Symbol, helper.Symbol}, ExpansionDeclarations: []DeclarationRef{root, helper}, ExpansionSites: []Site{{File: root.File, CallOffset: offset, Owner: &root}}}
		copies = append(copies, Source{Kind: "metric-contribution", File: helper.File, StartLine: 3, EndLine: 3, StartOffset: 50, EndOffset: 52, Detail: detail, Declaration: &helper, AggregateContributions: true})
	}
	return copies
}

// SQL acceptance assertions share the existing harness's fatal behavior while
// keeping test scenarios focused on their evidence rather than assertion APIs.
func (t *testHarness) equal(want, got any, message ...any) { require.Equal(t.T, want, got, message...) }
func (t *testHarness) noError(err error, message ...any)   { require.NoError(t.T, err, message...) }
func (t *testHarness) hasError(err error, message ...any)  { require.Error(t.T, err, message...) }
func (t *testHarness) contains(value any, part string, message ...any) {
	require.Contains(t.T, value, part, message...)
}
func (t *testHarness) notContains(value any, part string, message ...any) {
	require.NotContains(t.T, value, part, message...)
}
func (t *testHarness) empty(value any, message ...any)    { require.Empty(t.T, value, message...) }
func (t *testHarness) notEmpty(value any, message ...any) { require.NotEmpty(t.T, value, message...) }
func (t *testHarness) zero(value any, message ...any)     { require.Zero(t.T, value, message...) }
func (t *testHarness) greater(value, minimum int, message ...any) {
	require.Greater(t.T, value, minimum, message...)
}
func (t *testHarness) length(value any, count int, message ...any) {
	require.Len(t.T, value, count, message...)
}

func TestCLIUnusedSuppressionWarning(t *testing.T) {
	(&testHarness{T: t}).TestCLIUnusedSuppressionWarning()
}
func (t *testHarness) TestCLIUnusedSuppressionWarning() {
	source := "package fixture\n// columbo:ignore long-parameter-list -- kept for compatibility review\nfunc F(a,b int){}\n"
	dir := t.fixture(source)
	result := t.runCLI(dir, "--no-history")
	message := "Unused suppression for long-parameter-list on fixture.F."
	t.equal(0, result.code)
	t.equal(message+"\n", result.stderr)
	t.contains(string(result.stdout), message)
	t.checkUnusedSuppressionSnapshot(t.onlyDefaultSnapshot(dir))
}
func (t *testHarness) checkUnusedSuppressionSnapshot(path string) {
	db, err := openTestSnapshot(path)
	t.noError(err)
	defer db.Close()
	t.equal(1, t.sqlCount(db, `SELECT COUNT(*) FROM suppressions WHERE applied=0 AND case_id IS NULL`))
	t.equal(1, t.sqlCount(db, `SELECT COUNT(*) FROM warnings WHERE code='unused-suppression'`))
	t.zero(t.sqlCount(db, `SELECT failed+warned+suppressed FROM summary`))
}

func TestStoredSummaryReadFailure(t *testing.T) { (&testHarness{T: t}).TestStoredSummaryReadFailure() }
func (t *testHarness) TestStoredSummaryReadFailure() {
	db := t.snapshot(savedSummaryFixture())
	t.noError(db.Close())
	summary, exitCode, err := RenderSnapshot(testDatabaseQueries(db), "saved.sqlite")
	t.hasError(err)
	t.equal(2, exitCode)
	t.empty(summary, "failed readers must not return a partial analysis summary")
}
