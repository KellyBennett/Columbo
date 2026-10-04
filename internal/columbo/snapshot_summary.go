package columbo

import (
	"bytes"
	"database/sql"
	"fmt"
	"strconv"
)

// RenderSnapshot reads an already completed snapshot. It never evaluates rules
// or accepts an analyzer report; the stored outcomes also determine its exit code.
func RenderSnapshot(db *sql.DB, path string) ([]byte, int, error) {
	renderer := &snapshotRenderer{db: db}
	if err := renderer.render(path); err != nil {
		return nil, 2, err
	}
	return renderer.buffer.Bytes(), renderer.exitCode(), nil
}

type snapshotRenderer struct {
	db     *sql.DB
	buffer bytes.Buffer
	failed int
}

func (r *snapshotRenderer) emit(format string, args ...any) {
	fmt.Fprintf(&r.buffer, format, args...)
}
func (r *snapshotRenderer) render(path string) error {
	for _, stage := range []func() error{r.cases, r.warnings, r.totals} {
		if err := stage(); err != nil {
			return err
		}
	}
	r.emit("Database: %s\n", path)
	return nil
}
func (r *snapshotRenderer) exitCode() int {
	if r.failed > 0 {
		return 1
	}
	return 0
}

type summaryCase struct {
	id, smell, symbol, file, verdict string
	start, end, suppressed           int
}

const summaryCasesSQL = `SELECT c.id,c.smell,d.symbol,f.path,c.start_line,c.end_line,c.verdict,c.suppressed
FROM cases c JOIN declarations d ON d.id=c.primary_declaration_id
JOIN files f ON f.id=d.file_id ORDER BY c.ordinal`

func (r *snapshotRenderer) cases() error {
	entries, err := r.readCases()
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := r.caseEntry(&entry); err != nil {
			return err
		}
	}
	return nil
}
func (r *snapshotRenderer) readCases() ([]summaryCase, error) {
	rows, err := r.db.Query(summaryCasesSQL)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return r.caseRows(rows)
}
func (r *snapshotRenderer) caseRows(rows *sql.Rows) ([]summaryCase, error) {
	entries := []summaryCase{}
	for rows.Next() {
		entry := &summaryCase{}
		if err := entry.scan(rows); err != nil {
			return nil, err
		}
		entries = append(entries, *entry)
	}
	return entries, rows.Err()
}
func (c *summaryCase) scan(rows *sql.Rows) error {
	return rows.Scan(&c.id, &c.smell, &c.symbol, &c.file, &c.start, &c.end, &c.verdict, &c.suppressed)
}
func (r *snapshotRenderer) caseEntry(c *summaryCase) error {
	c.writeHeader(r)
	for _, stage := range []func(string) error{r.suppressions, r.metrics, r.policyReviews} {
		if err := stage(c.id); err != nil {
			return err
		}
	}
	return nil
}
func (c *summaryCase) writeHeader(r *snapshotRenderer) {
	r.emit("CASE %s %s %s:%d-%d\n  %s: %s\n", c.id, c.symbol, c.file, c.start, c.end, c.smell, c.outcome())
}
func (c *summaryCase) outcome() string {
	if c.suppressed != 0 {
		return "SUPPRESSED (original: " + c.verdict + ")"
	}
	return c.verdict
}

const summarySuppressionsSQL = `SELECT f.path,s.line,s.justification FROM suppressions s
JOIN files f ON f.id=s.file_id WHERE s.case_id=? ORDER BY s.ordinal`

func (r *snapshotRenderer) suppressions(id string) error {
	rows, err := r.db.Query(summarySuppressionsSQL, id)
	if err != nil {
		return err
	}
	defer rows.Close()
	return r.suppressionRows(rows)
}
func (r *snapshotRenderer) suppressionRows(rows *sql.Rows) error {
	for rows.Next() {
		var file, reason string
		var line int
		if err := rows.Scan(&file, &line, &reason); err != nil {
			return err
		}
		r.emit("  suppression %s:%d: %s\n", file, line, reason)
	}
	return rows.Err()
}

type summaryMetric struct {
	kind, subject string
	value, limit  any
	operator      sql.NullString
}

const summaryMetricsSQL = `SELECT kind,subject,numeric_value,limit_value,operator FROM clues
WHERE case_id=? AND numeric_value IS NOT NULL ORDER BY ordinal`

func (r *snapshotRenderer) metrics(id string) error {
	rows, err := r.db.Query(summaryMetricsSQL, id)
	if err != nil {
		return err
	}
	defer rows.Close()
	return r.metricRows(rows)
}
func (r *snapshotRenderer) metricRows(rows *sql.Rows) error {
	for rows.Next() {
		metric := &summaryMetric{}
		if err := metric.scan(rows); err != nil {
			return err
		}
		metric.write(r)
	}
	return rows.Err()
}
func (m *summaryMetric) scan(rows *sql.Rows) error {
	return rows.Scan(&m.kind, &m.subject, &m.value, &m.limit, &m.operator)
}
func (m *summaryMetric) write(r *snapshotRenderer) {
	r.emit("  %s %s: %s", m.kind, m.subject, m.number(m.value))
	if m.limit != nil {
		r.emit(" (limit %s %s)", m.operator.String, m.number(m.limit))
	}
	r.emit("\n")
}
func (m *summaryMetric) number(value any) string {
	if m.kind == "parameter-overlap" || m.kind == "dependency-overlap" || m.kind == "foreign-own-ratio" {
		return fixedSnapshotNumber(value)
	}
	return snapshotNumber(value)
}
func fixedSnapshotNumber(value any) string {
	if integer, ok := value.(int64); ok {
		return fmt.Sprintf("%.6f", float64(integer))
	}
	return fmt.Sprintf("%.6f", value)
}
func snapshotNumber(value any) string {
	if integer, ok := value.(int64); ok {
		return strconv.FormatInt(integer, 10)
	}
	return strconv.FormatFloat(value.(float64), 'g', -1, 64)
}

const summaryPolicySQL = `SELECT p.id,p.note,p.review_prompt FROM case_policy_reviews cp
JOIN policy_reviews p ON p.id=cp.policy_id WHERE cp.case_id=? ORDER BY cp.ordinal`

func (r *snapshotRenderer) policyReviews(id string) error {
	rows, err := r.db.Query(summaryPolicySQL, id)
	if err != nil {
		return err
	}
	defer rows.Close()
	return r.policyRows(rows)
}
func (r *snapshotRenderer) policyRows(rows *sql.Rows) error {
	for rows.Next() {
		var id, note, prompt string
		if err := rows.Scan(&id, &note, &prompt); err != nil {
			return err
		}
		r.emit("  Policy %s: %s\n  Review: %s\n", id, note, prompt)
	}
	return rows.Err()
}

const summaryWarningsSQL = `SELECT code,COALESCE(f.path,''),w.line,w.message FROM warnings w
LEFT JOIN files f ON f.id=w.file_id ORDER BY w.ordinal`

func (r *snapshotRenderer) warnings() error {
	rows, err := r.db.Query(summaryWarningsSQL)
	if err != nil {
		return err
	}
	defer rows.Close()
	return r.warningRows(rows)
}
func (r *snapshotRenderer) warningRows(rows *sql.Rows) error {
	for rows.Next() {
		var code, file, message string
		var line int
		if err := rows.Scan(&code, &file, &line, &message); err != nil {
			return err
		}
		r.emit("Warning %s %s:%d: %s\n", code, file, line, message)
	}
	return rows.Err()
}
func (r *snapshotRenderer) totals() error {
	var warned, suppressed int
	if err := r.db.QueryRow("SELECT failed,warned,suppressed FROM summary").Scan(&r.failed, &warned, &suppressed); err != nil {
		return err
	}
	r.emit("Columbo: %d failed, %d warned, %d suppressed\n", r.failed, warned, suppressed)
	return nil
}

func snapshotWarningMessages(db *sql.DB) ([]string, error) {
	rows, err := db.Query("SELECT message FROM warnings ORDER BY ordinal")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return warningMessages(rows)
}
func warningMessages(rows *sql.Rows) ([]string, error) {
	messages := []string{}
	for rows.Next() {
		var message string
		if err := rows.Scan(&message); err != nil {
			return nil, err
		}
		messages = append(messages, message)
	}
	return messages, rows.Err()
}
