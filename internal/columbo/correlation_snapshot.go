package columbo

import (
	"context"
	"github.com/KellyBennett/Columbo/internal/snapshotdb"
)

func (report Report) writeCorrelations(w *snapshotWriter) {
	for _, correlation := range report.Correlations {
		correlation.writeSnapshot(w)
	}
}
func (c Correlation) writeSnapshot(w *snapshotWriter) {
	c.writeHeader(w)
	c.writeMembers(w)
	c.writeRole(w)
	for _, evidence := range c.Evidence {
		c.writeEvidence(w, evidence)
	}
	c.writeGuidance(w, "lead", c.Leads)
	c.writeGuidance(w, "avoid", c.Avoid)
}
func (c Correlation) writeHeader(w *snapshotWriter) {
	w.exec(func() error {
		return w.queries.InsertCorrelation(context.Background(), snapshotdb.InsertCorrelationParams{
			ID: c.ID, Kind: c.Kind, Confidence: c.Confidence, VariantDomain: c.Domain, Diagnosis: c.Diagnosis,
			PolicyID: c.Policy.ID, PolicyStatus: "provisional", PolicyNote: c.Policy.Note, ReviewPrompt: c.Policy.ReviewPrompt,
		})
	})
}
func (c Correlation) writeMembers(w *snapshotWriter) {
	for _, id := range c.CaseIDs {
		w.exec(func() error {
			return w.queries.LinkCorrelationCase(context.Background(), snapshotdb.LinkCorrelationCaseParams{CorrelationID: c.ID, CaseID: id})
		})
	}
}
func (c Correlation) writeRole(w *snapshotWriter) {
	if c.RoleID == "" {
		return
	}
	w.exec(func() error {
		return w.queries.LinkCorrelationRole(context.Background(), snapshotdb.LinkCorrelationRoleParams{CorrelationID: c.ID, CandidateID: c.RoleID})
	})
}
func (c Correlation) writeEvidence(w *snapshotWriter, evidence CorrelationEvidence) {
	id, ok := w.receipts[snapshotReceiptKey{evidence.CaseID, evidence.ReceiptKey}]
	w.require(ok, "correlation references missing case receipt")
	w.exec(func() error {
		return w.queries.LinkCorrelationEvidence(context.Background(), snapshotdb.LinkCorrelationEvidenceParams{CorrelationID: c.ID, CaseID: evidence.CaseID, ReceiptID: id})
	})
}
func (c Correlation) writeGuidance(w *snapshotWriter, kind string, entries []string) {
	for ordinal, text := range entries {
		w.exec(func() error {
			return w.queries.InsertCorrelationGuidance(context.Background(), snapshotdb.InsertCorrelationGuidanceParams{CorrelationID: c.ID, Kind: kind, Ordinal: int64(ordinal), Text: text})
		})
	}
}
