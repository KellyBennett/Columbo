package columbo

import (
	"bytes"
	"context"
	"fmt"
	"strconv"

	"github.com/KellyBennett/Columbo/internal/snapshotdb"
)

// RenderSnapshot reads an already completed snapshot. It never evaluates rules
// or accepts an analyzer report; the stored outcomes also determine its exit code.
func RenderSnapshot(queries snapshotdb.Querier, path string) ([]byte, int, error) {
	renderer := &snapshotRenderer{queries: queries}
	if err := renderer.render(path); err != nil {
		return nil, 2, err
	}
	return renderer.buffer.Bytes(), renderer.exitCode(), nil
}

type snapshotRenderer struct {
	queries snapshotdb.Querier
	buffer  bytes.Buffer
	failed  int64
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

type summaryCase snapshotdb.SummaryCasesRow

func (r *snapshotRenderer) cases() error {
	entries, err := r.queries.SummaryCases(context.Background())
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := r.caseEntry(summaryCase(entry)); err != nil {
			return err
		}
	}
	return nil
}
func (r *snapshotRenderer) caseEntry(c summaryCase) error {
	c.writeHeader(r)
	for _, stage := range []func(string) error{r.suppressions, r.metrics, r.duplicateFragments, r.clueSets, r.variantDecisions, r.selectionReceipts, r.policyReviews} {
		if err := stage(c.ID); err != nil {
			return err
		}
	}
	return nil
}
func (c summaryCase) writeHeader(r *snapshotRenderer) {
	r.emit("CASE %s %s %s:%d-%d\n  %s: %s\n", c.ID, c.Symbol, c.Path, c.StartLine, c.EndLine, c.Smell, c.outcome())
}
func (c summaryCase) outcome() string {
	if c.Suppressed != 0 {
		return "SUPPRESSED (original: " + c.Verdict + ")"
	}
	return c.Verdict
}

func (r *snapshotRenderer) suppressions(id string) error {
	entries, err := r.queries.SummarySuppressions(context.Background(), snapshotdb.SummarySuppressionsParams{CaseID: &id})
	if err != nil {
		return err
	}
	for _, entry := range entries {
		r.emit("  suppression %s:%d: %s\n", entry.Path, entry.Line, entry.Justification)
	}
	return nil
}

type summaryMetric snapshotdb.SummaryMetricsRow

func (r *snapshotRenderer) metrics(id string) error {
	entries, err := r.queries.SummaryMetrics(context.Background(), snapshotdb.SummaryMetricsParams{CaseID: id})
	if err != nil {
		return err
	}
	for _, entry := range entries {
		summaryMetric(entry).write(r)
	}
	return nil
}
func (m summaryMetric) write(r *snapshotRenderer) {
	r.emit("  %s %s: %s", m.Kind, m.Subject, m.number(m.NumericValue))
	if m.LimitValue != nil {
		r.emit(" (limit %s %s)", snapshotOperator(m.Operator), m.number(m.LimitValue))
	}
	r.emit("\n")
}
func (m summaryMetric) number(value any) string {
	if m.Kind == "parameter-overlap" || m.Kind == "dependency-overlap" || m.Kind == "foreign-own-ratio" {
		return fixedSnapshotNumber(value)
	}
	return snapshotNumber(value)
}
func snapshotOperator(operator *string) string {
	if operator == nil {
		return ""
	}
	return *operator
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

func (r *snapshotRenderer) policyReviews(id string) error {
	entries, err := r.queries.SummaryPolicy(context.Background(), snapshotdb.SummaryPolicyParams{CaseID: id})
	if err != nil {
		return err
	}
	for _, entry := range entries {
		r.emit("  Policy %s: %s\n  Review: %s\n", entry.ID, entry.Note, entry.ReviewPrompt)
	}
	return nil
}

func (r *snapshotRenderer) warnings() error {
	entries, err := r.queries.SummaryWarnings(context.Background())
	if err != nil {
		return err
	}
	for _, entry := range entries {
		r.emit("Warning %s %s:%d: %s\n", entry.Code, entry.Path, entry.Line, entry.Message)
	}
	return nil
}

func (r *snapshotRenderer) totals() error {
	totals, err := r.queries.SummaryTotals(context.Background())
	if err != nil {
		return err
	}
	r.failed = totals.Failed
	r.emit("Columbo: %d failed, %d warned, %d suppressed\n", totals.Failed, totals.Warned, totals.Suppressed)
	return nil
}

func (r *snapshotRenderer) duplicateFragments(id string) error {
	entries, err := r.queries.SummaryDuplicateFragments(context.Background(), snapshotdb.SummaryDuplicateFragmentsParams{CaseID: id})
	if err != nil {
		return err
	}
	for _, entry := range entries {
		summaryDuplicateFragment(entry).write(r)
	}
	return nil
}

type summaryDuplicateFragment snapshotdb.SummaryDuplicateFragmentsRow

func (fragment summaryDuplicateFragment) write(r *snapshotRenderer) {
	r.emit("  duplicate fragment %s:%d-%d (bytes %d-%d)\n", fragment.Path, fragment.StartLine, fragment.EndLine, fragment.StartOffset, fragment.EndOffset)
}
