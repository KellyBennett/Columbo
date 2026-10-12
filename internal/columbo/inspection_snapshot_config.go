package columbo

import (
	"context"
	"fmt"

	"github.com/KellyBennett/Columbo/internal/snapshotdb"
)

func (facts inspectionFacts) snapshotConfig(file inspectionFile, fileID int64) (snapshotdb.InsertSourceConfigParams, error) {
	if facts.ConfigSHA != file.SHA {
		return snapshotdb.InsertSourceConfigParams{}, fmt.Errorf("inspection configuration does not match source inventory")
	}
	params := snapshotdb.InsertSourceConfigParams{Staged: snapshotFlag(facts.Config.Staged), History: snapshotFlag(facts.Config.History), MaxCommits: facts.Config.MaxCommits}
	if facts.ConfigFile != "" {
		params.ConfigPath = &facts.ConfigFile
	}
	if facts.ConfigSHA != "" {
		params.ConfigFileID, params.ConfigSha256 = &fileID, &facts.ConfigSHA
	}
	return params, nil
}

func (w *inspectionSnapshotWriter) writeConfig(facts inspectionFacts) {
	params, err := facts.snapshotConfig(w.files[facts.ConfigFile], w.snapshotWriter.files[facts.ConfigFile])
	w.record(err)
	w.exec(func() error { return w.queries.InsertSourceConfig(context.Background(), params) })
	facts.Config.writeInspectionSettings(w)
}

func (config Config) writeInspectionSettings(w *inspectionSnapshotWriter) {
	w.writeSeverities(config.Severity)
	w.writeCounts(config.Counts)
	w.writeRatios(config.Ratios)
	w.writeExclusions(config.Exclude)
}

func writeInspectionMap[T any](w *inspectionSnapshotWriter, values map[string]T, insert func(string, T) error) {
	for _, key := range inspectionSortedKeys(values) {
		w.exec(func() error { return insert(key, values[key]) })
	}
}

func (w *inspectionSnapshotWriter) writeSeverities(severities map[string]string) {
	writeInspectionMap(w, severities, func(smell, severity string) error {
		return w.queries.InsertSourceConfigSeverity(context.Background(), snapshotdb.InsertSourceConfigSeverityParams{Smell: smell, Severity: severity})
	})
}

func (w *inspectionSnapshotWriter) writeCounts(counts map[string]int64) {
	writeInspectionMap(w, counts, func(name string, value int64) error {
		return w.queries.InsertSourceConfigCount(context.Background(), snapshotdb.InsertSourceConfigCountParams{Name: name, Value: value})
	})
}

func (w *inspectionSnapshotWriter) writeRatios(ratios map[string]float64) {
	writeInspectionMap(w, ratios, func(name string, value float64) error {
		return w.queries.InsertSourceConfigRatio(context.Background(), snapshotdb.InsertSourceConfigRatioParams{Name: name, Value: value})
	})
}

func (w *inspectionSnapshotWriter) writeExclusions(patterns []string) {
	for ordinal, pattern := range patterns {
		w.exec(func() error {
			return w.queries.InsertSourceConfigExclusion(context.Background(), snapshotdb.InsertSourceConfigExclusionParams{Ordinal: int64(ordinal), Pattern: pattern})
		})
	}
}
