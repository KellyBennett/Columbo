package columbo

import (
	"context"
	"github.com/KellyBennett/Columbo/internal/snapshotdb"
)

func (r *snapshotRenderer) correlations() error {
	entries, err := r.queries.SummaryCorrelations(context.Background())
	if err != nil {
		return err
	}
	for _, c := range entries {
		if err := r.correlation(c); err != nil {
			return err
		}
	}
	return nil
}
func (r *snapshotRenderer) correlation(c snapshotdb.Correlation) error {
	r.emit("CORRELATION %s\n  Missing Polymorphic Role — %s\n  %s\n", c.ID, c.Confidence, c.Diagnosis)
	if c.VariantDomain != "" {
		r.emit("  Variant domain: %s\n", c.VariantDomain)
	}
	for _, stage := range []func(string) error{r.correlationMembers, r.correlationRoles, r.correlationPlayers, r.correlationMappings, r.correlationGuidance} {
		if err := stage(c.ID); err != nil {
			return err
		}
	}
	r.emit("  Policy %s (%s): %s\n  Review: %s\n", c.PolicyID, c.PolicyStatus, c.PolicyNote, c.ReviewPrompt)
	return nil
}
func (r *snapshotRenderer) correlationMembers(id string) error {
	entries, err := r.queries.SummaryCorrelationCases(context.Background(), snapshotdb.SummaryCorrelationCasesParams{CorrelationID: id})
	if err != nil {
		return err
	}
	for _, entry := range entries {
		status := entry.Verdict
		if entry.Suppressed != 0 {
			status = "SUPPRESSED (original: " + status + ")"
		}
		r.emit("  Connects: %s [%s]\n", entry.ID, status)
	}
	return nil
}
func (r *snapshotRenderer) correlationRoles(id string) error {
	entries, err := r.queries.SummaryCorrelationRoles(context.Background(), snapshotdb.SummaryCorrelationRolesParams{CorrelationID: id})
	if err != nil {
		return err
	}
	for _, entry := range entries {
		r.emit("  Observed role %s: %s\n", entry.ID, entry.CanonicalInterface)
	}
	return nil
}

func (r *snapshotRenderer) correlationPlayers(id string) error {
	entries, err := r.queries.SummaryCorrelationPlayers(context.Background(), snapshotdb.SummaryCorrelationPlayersParams{CorrelationID: id})
	if err != nil {
		return err
	}
	for _, entry := range entries {
		r.emit("  Player: %s\n", entry)
	}
	return nil
}
func (r *snapshotRenderer) correlationMappings(id string) error {
	entries, err := r.queries.SummaryCorrelationMappings(context.Background(), snapshotdb.SummaryCorrelationMappingsParams{CorrelationID: id})
	if err != nil {
		return err
	}
	for _, entry := range entries {
		r.emit("  Mapping: %s → %s\n", entry.Variant, entry.Implementation)
	}
	return nil
}

func (r *snapshotRenderer) correlationGuidance(id string) error {
	entries, err := r.queries.SummaryCorrelationGuidance(context.Background(), snapshotdb.SummaryCorrelationGuidanceParams{CorrelationID: id})
	if err != nil {
		return err
	}
	for _, entry := range entries {
		r.emit("  %s: %s\n", entry.Kind, entry.Text)
	}
	return nil
}
