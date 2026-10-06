package columbo

import (
	"context"
	"github.com/KellyBennett/Columbo/internal/snapshotdb"
	"strconv"
	"strings"
)

func (r *snapshotRenderer) variantSets(id string) error {
	rows, err := r.queries.SummaryVariantSets(context.Background(), snapshotdb.SummaryVariantSetsParams{CaseID: id})
	if err != nil {
		return err
	}
	sets := summaryVariantSets{rows: rows}
	for sets.more() {
		sets.next().write(r)
	}
	return nil
}

// The cursor owns grouping the ordered relational values into one displayed set.
type summaryVariantSets struct {
	rows     []snapshotdb.SummaryVariantSetsRow
	position int
}
type summaryVariantSet struct {
	kind, subject string
	values        []string
}

func (s *summaryVariantSets) more() bool { return s.position < len(s.rows) }
func (s *summaryVariantSets) next() summaryVariantSet {
	first := s.rows[s.position]
	set := summaryVariantSet{kind: first.Kind, subject: first.Subject}
	for s.more() && s.rows[s.position].ClueOrdinal == first.ClueOrdinal {
		set.values = append(set.values, s.rows[s.position].Value)
		s.position++
	}
	return set
}
func (s summaryVariantSet) write(r *snapshotRenderer) {
	r.emit("  %s %s: %s\n", s.kind, s.subject, strings.Join(s.values, ", "))
}
func (r *snapshotRenderer) variantDecisions(id string) error {
	rows, err := r.queries.SummaryVariantDecisions(context.Background(), snapshotdb.SummaryVariantDecisionsParams{CaseID: id})
	if err != nil {
		return err
	}
	for _, row := range rows {
		summaryVariantDecision(row).write(r)
	}
	return nil
}

type summaryVariantDecision snapshotdb.SummaryVariantDecisionsRow

func (d summaryVariantDecision) write(r *snapshotRenderer) {
	r.emit("  variant decision %s:%d-%d (bytes %d-%d) in %s\n", d.Path, d.StartLine, d.EndLine, d.StartOffset, d.EndOffset, d.Symbol)
}

func variantSiteSubjectLess(a, b string) bool {
	fileA, offsetA := variantSiteSubject(a)
	fileB, offsetB := variantSiteSubject(b)
	if fileA != fileB {
		return fileA < fileB
	}
	return offsetA < offsetB
}

func variantSiteSubject(subject string) (string, int) {
	index := strings.LastIndexByte(subject, ':')
	if index < 0 {
		return subject, 0
	}
	offset, _ := strconv.Atoi(subject[index+1:])
	return subject[:index], offset
}
