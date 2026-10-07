package columbo

import (
	"context"
	"github.com/KellyBennett/Columbo/internal/snapshotdb"
)

func (r *snapshotRenderer) advisories() error {
	groups, err := r.queries.SummaryAdvisories(context.Background())
	if err != nil {
		return err
	}
	for _, group := range groups {
		if err := summaryAdvisory(group).render(r); err != nil {
			return err
		}
	}
	return nil
}

type summaryAdvisory snapshotdb.AdvisoryGroup

func (group summaryAdvisory) render(r *snapshotRenderer) error {
	r.emit("Evidence %s %s: %s\n  Lead: %s\n  Scope: %s\n", group.ID, group.Kind, group.Subject, group.Lead, group.Limits)
	if err := r.advisoryValues(group.ID); err != nil {
		return err
	}
	return r.advisorySites(group.ID)
}
func (r *snapshotRenderer) advisoryValues(id string) error {
	values, err := r.queries.AdvisoryValues(context.Background(), snapshotdb.AdvisoryValuesParams{GroupID: id})
	if err != nil {
		return err
	}
	for _, value := range values {
		r.emit("  %s: %s %s\n", value.Kind, value.Identity, value.Value)
	}
	return nil
}
func (r *snapshotRenderer) advisorySites(id string) error {
	sites, err := r.queries.AdvisorySites(context.Background(), snapshotdb.AdvisorySitesParams{GroupID: id})
	if err != nil {
		return err
	}
	for _, site := range sites {
		r.emit("  %s %s %s\n", site.Symbol, site.Representation, site.InputExpression)
		if err := r.advisoryReceipts(id, site); err != nil {
			return err
		}
	}
	return nil
}
func (r *snapshotRenderer) advisoryReceipts(id string, site snapshotdb.AdvisorySitesRow) error {
	params := snapshotdb.AdvisoryReceiptsParams{GroupID: id, SiteOrdinal: site.Ordinal}
	rows, err := r.queries.AdvisoryReceipts(context.Background(), params)
	if err != nil {
		return err
	}
	for _, row := range rows {
		summaryAdvisoryReceipt(row).render(r, site.Path)
	}
	return nil
}

type summaryAdvisoryReceipt snapshotdb.AdvisoryReceiptsRow

func (row summaryAdvisoryReceipt) render(r *snapshotRenderer, path string) {
	r.emit("    %s %s:%d-%d [%s]: %s\n", row.Kind, path, row.StartLine, row.EndLine, row.Subject, row.Spelling)
}
