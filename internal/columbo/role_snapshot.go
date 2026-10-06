package columbo

import (
	"context"
	"github.com/KellyBennett/Columbo/internal/snapshotdb"
)

func (report Report) writeRoles(w *snapshotWriter) {
	for _, role := range report.Roles {
		role.writeSnapshot(w)
	}
}
func (r RoleCandidate) writeSnapshot(w *snapshotWriter) {
	w.exec(func() error {
		return w.queries.InsertRoleCandidate(context.Background(), snapshotdb.InsertRoleCandidateParams{ID: r.ID, CanonicalInterface: r.Interface, Confidence: r.Confidence, Classification: r.Classification})
	})
	r.writeSurface(w)
	r.writeInterfaces(w)
	for ordinal, receipt := range r.Receipts {
		r.writeReceipt(w, ordinal, receipt)
	}
	r.writeCaseLinks(w)
}
func (r RoleCandidate) writeCaseLinks(w *snapshotWriter) {
	for _, id := range r.CaseIDs {
		w.exec(func() error {
			return w.queries.LinkRoleCase(context.Background(), snapshotdb.LinkRoleCaseParams{CandidateID: r.ID, CaseID: id})
		})
	}
}
func (r RoleCandidate) writeSurface(w *snapshotWriter) {
	for _, implementation := range r.Implementations {
		w.exec(func() error {
			return w.queries.InsertRoleImplementation(context.Background(), snapshotdb.InsertRoleImplementationParams{CandidateID: r.ID, Identity: implementation})
		})
	}
	for _, message := range r.Messages {
		w.exec(func() error {
			return w.queries.InsertRoleMessage(context.Background(), snapshotdb.InsertRoleMessageParams{CandidateID: r.ID, Identity: message})
		})
	}
}
func (r RoleCandidate) writeInterfaces(w *snapshotWriter) {
	groups := []struct {
		kind  string
		names []string
	}{{"exact", r.ExactInterfaces}, {"compatible", r.CompatibleInterfaces}, {"used", r.UsedInterfaces}}
	for _, group := range groups {
		for _, name := range group.names {
			w.exec(func() error {
				return w.queries.InsertRoleInterface(context.Background(), snapshotdb.InsertRoleInterfaceParams{CandidateID: r.ID, Identity: name, Relationship: group.kind})
			})
		}
	}
}
func (r RoleCandidate) writeReceipt(w *snapshotWriter, ordinal int, source Source) {
	params := source.roleSnapshotReceipt(w)
	params.CandidateID, params.Ordinal = r.ID, int64(ordinal)
	w.exec(func() error { return w.queries.InsertRoleReceipt(context.Background(), params) })
}
func (source Source) roleSnapshotReceipt(w *snapshotWriter) snapshotdb.InsertRoleReceiptParams {
	w.require(source.Declaration != nil, "role receipt requires a declaration")
	if source.Declaration == nil {
		return snapshotdb.InsertRoleReceiptParams{}
	}
	declaration := w.declaration(*source.Declaration)
	return snapshotdb.InsertRoleReceiptParams{DeclarationID: declaration, Kind: source.Kind, Subject: source.Detail.Subject, Message: source.RoleMessage, StartLine: int64(source.StartLine), EndLine: int64(source.EndLine), StartOffset: int64(source.StartOffset), EndOffset: int64(source.EndOffset)}
}
func (r *snapshotRenderer) roles() error {
	roles, err := r.queries.SummaryRoles(context.Background())
	if err != nil {
		return err
	}
	for _, role := range roles {
		r.emit("Role evidence %s (%s, %s): %s\n", role.ID, role.Confidence, role.Classification, role.CanonicalInterface)
		if err := r.roleInterfaces(role.ID); err != nil {
			return err
		}
	}
	return nil
}
func (r *snapshotRenderer) roleInterfaces(id string) error {
	rows, err := r.queries.SummaryRoleInterfaces(context.Background(), snapshotdb.SummaryRoleInterfacesParams{CandidateID: id})
	if err != nil {
		return err
	}
	for _, row := range rows {
		r.emit("  %s interface: %s\n", row.Relationship, row.Identity)
	}
	return nil
}
