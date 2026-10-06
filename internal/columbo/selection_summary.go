package columbo

import (
	"context"
	"github.com/KellyBennett/Columbo/internal/snapshotdb"
	"strings"
)

func (r *snapshotRenderer) selectionReceipts(id string) error {
	rows, err := r.queries.SummarySelectionReceipts(context.Background(), snapshotdb.SummarySelectionReceiptsParams{CaseID: id})
	if err != nil {
		return err
	}
	for _, row := range rows {
		summarySelectionReceipt(row).write(r)
	}
	return nil
}

type summarySelectionReceipt snapshotdb.SummarySelectionReceiptsRow

func (s summarySelectionReceipt) write(r *snapshotRenderer) {
	r.emit("  %s %s:%d-%d (bytes %d-%d): %s\n", strings.ReplaceAll(s.Kind, "-", " "), s.Path, s.StartLine, s.EndLine, s.StartOffset, s.EndOffset, s.Subject)
}
