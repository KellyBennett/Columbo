package columbo

import (
	"database/sql"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

type sqliteHarness struct {
	*testing.T
	path string
}

func newSQLiteHarness(t *testing.T) *sqliteHarness {
	return &sqliteHarness{T: t, path: filepath.Join(t.TempDir(), "report ?#%.sqlite")}
}
func (h *sqliteHarness) writeEmpty() { h.write(Report{}) }
func (h *sqliteHarness) write(report Report) {
	h.Helper()
	require.NoError(h.T, WriteSnapshot(h.path, report, "sqlite-test"))
}
func (h *sqliteHarness) open() *sql.DB {
	h.Helper()
	db, err := OpenSnapshot(h.path)
	require.NoError(h.T, err)
	h.Cleanup(func() { require.NoError(h.T, db.Close()) })
	return db
}
func (h *sqliteHarness) bytes() []byte {
	h.Helper()
	data, err := os.ReadFile(h.path)
	require.NoError(h.T, err)
	return data
}
func (h *sqliteHarness) mutable() *sql.DB {
	h.Helper()
	db, err := openSnapshotDatabase(h.path, false)
	require.NoError(h.T, err)
	h.Cleanup(func() { require.NoError(h.T, db.Close()) })
	return db
}
func (h *sqliteHarness) mutation(statement string) {
	h.Helper()
	db := h.mutable()
	_, err := db.Exec(statement)
	require.NoError(h.T, err)
	require.NoError(h.T, db.Close())
}
func (h *sqliteHarness) preserved(report Report) {
	h.Helper()
	before := h.bytes()
	require.Error(h.T, WriteSnapshot(h.path, report, "replacement"))
	require.Equal(h.T, before, h.bytes())
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(h.path), ".columbo-*"))
	require.NoError(h.T, err)
	require.Empty(h.T, matches)
}
func TestSQLiteEmptyAndReadOnly(t *testing.T) { newSQLiteHarness(t).checkEmptyAndReadOnly() }
func (h *sqliteHarness) checkEmptyAndReadOnly() {
	h.writeEmpty()
	db := h.open()
	var failed, warned, suppressed int
	require.NoError(h.T, db.QueryRow("SELECT * FROM summary").Scan(&failed, &warned, &suppressed))
	require.Equal(h.T, []int{0, 0, 0}, []int{failed, warned, suppressed})
	_, err := db.Exec("DELETE FROM report")
	require.Error(h.T, err)
	require.NoError(h.T, snapshotNoSidecars(h.path))
}
func TestSQLiteExistingSnapshotRefused(t *testing.T) {
	newSQLiteHarness(t).checkExistingSnapshotRefused()
}
func (h *sqliteHarness) checkExistingSnapshotRefused() {
	h.writeEmpty()
	h.preserved(sqliteTestReport())
	db := h.open()
	var count int
	require.NoError(h.T, db.QueryRow("SELECT COUNT(*) FROM cases").Scan(&count))
	require.Zero(h.T, count)
}
func TestSQLiteInvalidDestinationsPreserved(t *testing.T) {
	statements := []string{"PRAGMA user_version=999", "PRAGMA application_id=0", "DROP VIEW summary", "CREATE TABLE unrelated (id INTEGER)", "PRAGMA foreign_keys=OFF; INSERT INTO case_policy_reviews VALUES ('orphan',0,'unknown')"}
	for _, statement := range statements {
		t.Run(statement, func(t *testing.T) {
			h := newSQLiteHarness(t)
			h.writeEmpty()
			h.mutation(statement)
			h.invalidSnapshot()
			h.preserved(Report{})
		})
	}
}
func TestSQLiteCorruptAndUnrelatedPreserved(t *testing.T) {
	newSQLiteHarness(t).checkCorruptAndUnrelatedPreserved()
}

// sqliteX is an ordinary user table: only the literal sqlite_ prefix is reserved.
// Its contents must remain visible to readers and protected from publication.
func TestSQLiteUserTableWithSQLitePrefixPreserved(t *testing.T) {
	newSQLiteHarness(t).checkUserTableWithSQLitePrefixPreserved()
}
func (h *sqliteHarness) checkUserTableWithSQLitePrefixPreserved() {
	h.writeEmpty()
	h.mutation("CREATE TABLE sqliteX (value TEXT); INSERT INTO sqliteX VALUES ('unrelated user data')")
	h.invalidSnapshot()
	h.preserved(Report{})
	require.Equal(h.T, "unrelated user data", h.userTableValue())
	h.userTableDiscovered()
}
func (h *sqliteHarness) invalidSnapshot() {
	db, err := OpenSnapshot(h.path)
	require.Error(h.T, err)
	require.Nil(h.T, db)
}
func (h *sqliteHarness) userTableValue() string {
	db := h.mutable()
	var value string
	require.NoError(h.T, db.QueryRow("SELECT value FROM sqliteX").Scan(&value))
	return value
}
func (h *sqliteHarness) userTableDiscovered() {
	rows := (&testHarness{h.T}).logicalRows(h.mutable())
	require.Contains(h.T, string(rows), "[sqliteX] value\n\"unrelated user data\"")
}
func (h *sqliteHarness) checkCorruptAndUnrelatedPreserved() {
	for _, content := range []string{"", "not sqlite", "SQLite format 3\x00truncated"} {
		h.Run(content, func(t *testing.T) {
			h := newSQLiteHarness(t)
			require.NoError(h.T, os.WriteFile(h.path, []byte(content), 0600))
			h.preserved(Report{})
		})
	}
}
func TestSQLiteSymlinkAndSidecarsRefused(t *testing.T) {
	newSQLiteHarness(t).checkSymlinkAndSidecarsRefused()
}
func (h *sqliteHarness) checkSymlinkAndSidecarsRefused() {
	h.writeEmpty()
	link := h.path + ".link"
	require.NoError(h.T, os.Symlink(h.path, link))
	require.Error(h.T, WriteSnapshot(link, Report{}, "test"))
	require.NoError(h.T, os.WriteFile(h.path+"-wal", []byte("sidecar"), 0600))
	h.preserved(Report{})
}
func TestSQLiteAbsentDestinationWithSidecarsRefused(t *testing.T) {
	for _, suffix := range []string{"-journal", "-wal", "-shm"} {
		t.Run(suffix, func(t *testing.T) { newSQLiteHarness(t).checkAbsentSidecar(suffix) })
	}
}
func (h *sqliteHarness) checkAbsentSidecar(suffix string) {
	sidecar := &sqliteHarness{T: h.T, path: h.path + suffix}
	require.NoError(h.T, os.WriteFile(sidecar.path, []byte("existing sidecar"), 0600))
	h.rejectedFresh(Report{})
	require.Equal(h.T, []byte("existing sidecar"), sidecar.bytes())
}
func TestSQLitePrepublicationFailuresPreservePrevious(t *testing.T) {
	newSQLiteHarness(t).checkInvalidReportsPreservePrevious()
}
func (h *sqliteHarness) checkInvalidReportsPreservePrevious() {
	previous := h.previousSnapshot()
	before := previous.bytes()
	report := sqliteTestReport()
	report.Cases[0].Receipts = append(report.Cases[0].Receipts, struct{}{})
	h.rejectedFresh(report)
	report.Cases[0].Receipts = nil
	report.Cases[0].Clues[0].SupportingReceipts = []string{"unknown"}
	h.rejectedFresh(report)
	require.Equal(h.T, before, previous.bytes())
}
func (h *sqliteHarness) rejectedFresh(report Report) {
	h.Helper()
	require.Error(h.T, WriteSnapshot(h.path, report, "test"))
	h.noOutput()
	h.noTemporarySnapshots()
}
func TestSQLiteNumericTypes(t *testing.T) {
	h := newSQLiteHarness(t)
	sqliteCheckNumericRows(t, h.numericDatabase())
}
func TestSQLiteListMultiplicity(t *testing.T) {
	h := newSQLiteHarness(t)
	sqliteCheckListRows(t, h.numericDatabase())
}
func (h *sqliteHarness) numericDatabase() *sql.DB {
	report := sqliteTestReport()
	report.Cases[0].Clues = sqliteNumericClues()
	h.write(report)
	return h.open()
}
func sqliteNumericClues() []Clue {
	return []Clue{{Kind: "integer", Value: int64(9007199254740993), Limit: int64(9007199254740992), Operator: ">"}, {Kind: "real", Value: float64(1), Limit: float64(.75), Operator: ">="}, {Kind: "empty", Value: []string{}}, {Kind: "repeat", Value: []string{"T", "T", "U"}}}
}
func sqliteCheckNumericRows(t *testing.T, db *sql.DB) {
	var integer int64
	var actualType, limitType string
	require.NoError(t, db.QueryRow("SELECT numeric_value,typeof(numeric_value),typeof(limit_value) FROM clues WHERE kind='integer'").Scan(&integer, &actualType, &limitType))
	require.Equal(t, int64(9007199254740993), integer)
	require.Equal(t, "integer", actualType)
	require.Equal(t, "integer", limitType)
	require.NoError(t, db.QueryRow("SELECT typeof(numeric_value),typeof(limit_value) FROM clues WHERE kind='real'").Scan(&actualType, &limitType))
	require.Equal(t, "real", actualType)
	require.Equal(t, "real", limitType)
}
func sqliteCheckListRows(t *testing.T, db *sql.DB) {
	var kind string
	var count int
	require.NoError(t, db.QueryRow("SELECT value_type,(SELECT COUNT(*) FROM clue_values v WHERE v.clue_id=q.id) FROM clues q WHERE kind='empty'").Scan(&kind, &count))
	require.Equal(t, "list", kind)
	require.Zero(t, count)
	var values string
	require.NoError(t, db.QueryRow("SELECT group_concat(value,',') FROM (SELECT v.value FROM clue_values v JOIN clues q ON q.id=v.clue_id WHERE q.kind='repeat' ORDER BY v.ordinal)").Scan(&values))
	require.Equal(t, "T,T,U", values)
}
func TestSQLiteNonfiniteAndMalformedValuesRejected(t *testing.T) {
	newSQLiteHarness(t).checkNonfiniteAndMalformedValuesRejected()
}
func (h *sqliteHarness) checkNonfiniteAndMalformedValuesRejected() {
	for _, value := range []any{nil, math.NaN(), math.Inf(1), "string", []int{1}} {
		h.Run("invalid numeric", func(t *testing.T) {
			h := newSQLiteHarness(t)
			report := sqliteTestReport()
			report.Cases[0].Clues[0].Value = value
			h.rejectedFresh(report)
		})
	}
}
func TestSQLiteOrphanAndCrossCaseConstraints(t *testing.T) {
	h := newSQLiteHarness(t)
	report := sqliteTestReport()
	second := report.Cases[0]
	second.ID = "C-second"
	report.Cases = append(report.Cases, second)
	h.write(report)
	sqliteCheckForbiddenLinks(t, h.mutable())
}
func sqliteCheckForbiddenLinks(t *testing.T, db *sql.DB) {
	statements := []string{"INSERT INTO case_policy_reviews VALUES ('missing',0,'missing')", "INSERT INTO clue_receipts SELECT 'C-first',c.id,s.id FROM clues c JOIN source_receipts s WHERE c.case_id='C-second' AND s.case_id='C-first'", "INSERT INTO clue_values SELECT id,0,'invalid' FROM clues WHERE value_type='integer'"}
	for _, statement := range statements {
		_, err := db.Exec(statement)
		require.Error(t, err, statement)
	}
}

type sqliteFixture struct {
	ref    DeclarationRef
	report Report
}

func sqliteTestReport() Report {
	fixture := sqliteFixture{ref: DeclarationRef{File: "source.go", Symbol: "fixture.F"}}
	fixture.declaration()
	fixture.primaryCase()
	fixture.clue()
	return fixture.report
}
func (f *sqliteFixture) declaration() {
	source := sqliteTestSource(f.ref)
	f.report.Declarations = []DeclarationEvidence{{Ref: f.ref, Source: source}}
}
func (f *sqliteFixture) primaryCase() {
	c := Case{ID: "C-first", Smell: "long-function", Verdict: "FAIL", Symbol: f.ref.Symbol, File: f.ref.File, StartLine: 1, EndLine: 2, PrimaryDeclaration: &f.ref}
	c.Receipts = []any{sqliteTestSource(f.ref)}
	f.report.Cases = []Case{c}
}
func (f *sqliteFixture) clue() {
	f.report.Cases[0].Clues = []Clue{{Kind: "function-lines", Subject: "a display subject is not a relationship", Value: 1, Declaration: &f.ref, SupportingReceipts: []string{"typed-key"}}}
}
func sqliteTestSource(ref DeclarationRef) Source {
	return Source{Kind: "metric-contribution", File: ref.File, StartLine: 1, EndLine: 1, StartOffset: 0, EndOffset: 1, Detail: Detail{Subject: "function-lines", Value: 1}, Declaration: &ref, EvidenceKey: "typed-key"}
}
