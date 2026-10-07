package columbo

import (
	"context"
	"github.com/KellyBennett/Columbo/internal/snapshotdb"
)

func (report Report) writeStages(w *snapshotWriter) {
	for ordinal, stage := range report.Stages {
		stage.writeSnapshot(w, ordinal)
	}
}
func (stage stageResult) writeSnapshot(w *snapshotWriter, ordinal int) {
	definition := stage.definition
	params := snapshotdb.InsertStageParams{ID: definition.id, Ordinal: int64(ordinal), Name: definition.name, Task: definition.task, State: stage.state, PendingDefinition: snapshotFlag(definition.pending), IssueCount: int64(stage.issueCount())}
	w.exec(func() error { return w.queries.InsertStage(context.Background(), params) })
	stage.writeCollectors(w)
}
func (stage stageResult) writeCollectors(w *snapshotWriter) {
	for index, collector := range stage.definition.collectors {
		member := snapshotdb.InsertStageCollectorParams{StageID: stage.definition.id, Ordinal: int64(index), Collector: collector, IssueCount: int64(stage.issues[collector])}
		w.exec(func() error { return w.queries.InsertStageCollector(context.Background(), member) })
	}
}
func (r *snapshotRenderer) staged(stages []snapshotdb.RefactoringStage) error {
	r.emit("Refactoring mode: staged (legacy verdicts retained in SQLite; not enforced)\n")
	for _, stage := range stages {
		r.stageHeader(stage)
		if err := r.stageCollectors(stage.ID); err != nil {
			return err
		}
	}
	if err := r.stageIssues(); err != nil {
		return err
	}
	return r.warnings()
}
func (r *snapshotRenderer) stageHeader(stage snapshotdb.RefactoringStage) {
	r.emit("Stage %s: %s\n", stage.Name, stage.State)
	if stage.State != "active" {
		return
	}
	r.failed = stage.IssueCount
	r.emit("  Task: %s\n", stage.Task)
	if stage.PendingDefinition != 0 {
		r.emit("  Pending definition; this is not completion.\n")
	}
}
func (r *snapshotRenderer) stageIssues() error {
	groups, err := r.queries.ActiveStageAdvisories(context.Background())
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
func (r *snapshotRenderer) stageCollectors(id string) error {
	members, err := r.queries.StageCollectors(context.Background(), snapshotdb.StageCollectorsParams{StageID: id})
	if err != nil {
		return err
	}
	for _, member := range members {
		r.emit("  Collector %s: %d issues\n", member.Collector, member.IssueCount)
	}
	return nil
}
