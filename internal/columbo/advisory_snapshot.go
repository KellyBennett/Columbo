package columbo

import (
	"context"
	"github.com/KellyBennett/Columbo/internal/snapshotdb"
)

func (report Report) writeAdvisories(w *snapshotWriter) {
	w.advisoryCoverage(overwriteKind, report.Tangles.Files, report.Tangles.Declarations)
	w.advisoryCoverage(categorySplitKind, report.Tangles.Files, report.Tangles.Declarations)
	w.advisoryCoverage(guardedUpdateKind, report.Tangles.Files, report.Tangles.Declarations)
	w.advisoryCoverage("choice-set", report.Choices.Files, report.Choices.Declarations)
	w.advisoryCoverage("nested-field-decision", report.Tangles.Files, report.Tangles.Declarations)
	if report.Stages != nil {
		w.advisoryCoverage("variant-coordination", report.Tangles.Files, report.Tangles.Declarations)
		w.advisoryCoverage(categoryBehaviorKind, report.Tangles.Files, report.Tangles.Declarations)
	}
	for _, group := range report.advisories() {
		group.writeSnapshot(w)
	}
}
func (w *snapshotWriter) advisoryCoverage(kind string, files, declarations int) {
	params := snapshotdb.InsertAdvisoryCollectorParams{Kind: kind, FilesAnalyzed: int64(files), DeclarationsAnalyzed: int64(declarations)}
	w.exec(func() error { return w.queries.InsertAdvisoryCollector(context.Background(), params) })
}
func (g advisoryGroup) writeSnapshot(w *snapshotWriter) {
	params := snapshotdb.InsertAdvisoryGroupParams{ID: g.id, Kind: g.kind, Subject: g.subject, Lead: g.lead, Limits: g.limits}
	w.exec(func() error { return w.queries.InsertAdvisoryGroup(context.Background(), params) })
	for ordinal, value := range g.values {
		value.writeSnapshot(w, g.id, ordinal)
	}
	for ordinal, site := range g.sites {
		site.writeSnapshot(w, g.id, ordinal)
	}
}
func (v advisoryValue) writeSnapshot(w *snapshotWriter, group string, ordinal int) {
	params := snapshotdb.InsertAdvisoryValueParams{GroupID: group, Kind: v.kind, Ordinal: int64(ordinal), Identity: v.identity, Value: v.value}
	w.exec(func() error { return w.queries.InsertAdvisoryValue(context.Background(), params) })
}

type advisorySiteWriter struct {
	writer  *snapshotWriter
	site    advisorySite
	group   string
	ordinal int64
}

func (site advisorySite) writeSnapshot(w *snapshotWriter, group string, ordinal int) {
	record := advisorySiteWriter{w, site, group, int64(ordinal)}
	record.write()
}
func (record advisorySiteWriter) write() {
	w := record.writer
	params := snapshotdb.InsertAdvisorySiteParams{GroupID: record.group, Ordinal: record.ordinal, DeclarationID: w.declaration(record.site.declaration()), Representation: record.site.representation, InputExpression: record.site.input}
	w.exec(func() error { return w.queries.InsertAdvisorySite(context.Background(), params) })
	record.details()
}
func (record advisorySiteWriter) details() {
	for ordinal, seed := range record.site.seeds {
		record.seed(ordinal, seed)
	}
	for ordinal, source := range record.site.receipts {
		record.receipt(ordinal, source)
	}
}
func (record advisorySiteWriter) seed(ordinal int, seed ChoiceConstant) {
	params := snapshotdb.InsertAdvisorySiteValueParams{GroupID: record.group, SiteOrdinal: record.ordinal, Ordinal: int64(ordinal), Identity: seed.Identity, Value: seed.Value}
	record.writer.exec(func() error { return record.writer.queries.InsertAdvisorySiteValue(context.Background(), params) })
}
func (record advisorySiteWriter) receipt(ordinal int, source Source) {
	record.writer.require(source.File == record.site.declaration().File, "advisory receipt file differs from declaration")
	params := advisoryReceipt(source)
	params.GroupID, params.SiteOrdinal, params.Ordinal = record.group, record.ordinal, int64(ordinal)
	record.writer.exec(func() error { return record.writer.queries.InsertAdvisoryReceipt(context.Background(), params) })
}
func advisoryReceipt(source Source) snapshotdb.InsertAdvisoryReceiptParams {
	return snapshotdb.InsertAdvisoryReceiptParams{Kind: source.Kind, Subject: source.Detail.Subject, StartLine: int64(source.StartLine), EndLine: int64(source.EndLine), StartOffset: int64(source.StartOffset), EndOffset: int64(source.EndOffset), Spelling: source.Spelling}
}
