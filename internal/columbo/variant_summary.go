package columbo

import (
	"context"
	"github.com/KellyBennett/Columbo/internal/snapshotdb"
	"strconv"
	"strings"
)

func (r *snapshotRenderer) clueSets(id string) error {
	rows, err := r.queries.SummaryClueSets(context.Background(), snapshotdb.SummaryClueSetsParams{CaseID: id})
	if err != nil {
		return err
	}
	sets := summaryClueSets{rows: rows}
	for sets.more() {
		sets.next().write(r)
	}
	return nil
}

type summaryClueSets struct {
	rows     []snapshotdb.SummaryClueSetsRow
	position int
}
type summaryClueSet struct {
	kind, subject string
	values        []string
}

func (s *summaryClueSets) more() bool { return s.position < len(s.rows) }
func (s *summaryClueSets) next() summaryClueSet {
	first := s.rows[s.position]
	set := summaryClueSet{kind: first.Kind, subject: first.Subject}
	for s.more() && s.rows[s.position].ClueOrdinal == first.ClueOrdinal {
		set.values = append(set.values, s.rows[s.position].Value)
		s.position++
	}
	return set
}
func (s summaryClueSet) write(r *snapshotRenderer) {
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
