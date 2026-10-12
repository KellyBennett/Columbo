package columbo

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/KellyBennett/Columbo/internal/snapshotdb"
	"github.com/stretchr/testify/require"
)

func TestInspectionSnapshotRejectsUnboundIdentity(t *testing.T) {
	root := inspectionTestModule(t)
	inspectionSQLWriteFile(t, root, ".columbo.yml", "history:\n  enabled: false\n")
	report := inspectionSQLAnalyze(t, root, "")
	for name, modify := range map[string]func(*inspectionFacts){
		"changed identity": func(facts *inspectionFacts) { facts.SourceSHA = strings.Repeat("0", 64) },
		"missing inventory file": func(facts *inspectionFacts) {
			facts.Files = slices.DeleteFunc(facts.Files, func(file inspectionFile) bool { return file.Path == "go.mod" })
		},
		"unbound configuration":    func(facts *inspectionFacts) { facts.ConfigSHA = "" },
		"mismatched configuration": func(facts *inspectionFacts) { facts.ConfigSHA = strings.Repeat("0", 64) },
	} {
		t.Run(name, func(t *testing.T) {
			facts := *report.inspection
			facts.Files = append([]inspectionFile{}, facts.Files...)
			modify(&facts)
			changed := report
			changed.inspection = &facts
			path := filepath.Join(t.TempDir(), "rejected.sqlite")
			require.Error(t, WriteSnapshot(path, changed, "test"))
			_, err := os.Stat(path)
			require.ErrorIs(t, err, os.ErrNotExist, "invalid inspection snapshot was published")
		})
	}
}

func TestInspectionSnapshotCanonicalInventoryOrder(t *testing.T) {
	report := inspectionSQLAnalyze(t, inspectionTestModule(t), "")
	original := inspectionSQLReader(t, report)
	want, err := original.InspectionFiles(context.Background())
	require.NoError(t, err)
	facts := *report.inspection
	facts.Files = append([]inspectionFile{}, facts.Files...)
	slices.Reverse(facts.Files)
	report.inspection = &facts
	shuffled := inspectionSQLReader(t, report)
	got, err := shuffled.InspectionFiles(context.Background())
	require.NoError(t, err)
	require.Equal(t, want, got)
	identity, err := shuffled.InspectionIdentity(context.Background())
	require.NoError(t, err)
	require.Equal(t, facts.SourceSHA, identity.SourceSha256)
	require.Nil(t, identity.ConfigSha256)
}

func TestInspectionSnapshotPreservesRepeatedBlankFieldNames(t *testing.T) {
	root := inspectionTestModule(t)
	inspectionSQLWriteFile(t, root, "padding.go", "package fixture\ntype Padding struct { _, _ int }\n")
	q := inspectionSQLReader(t, inspectionSQLAnalyze(t, root, ""))
	types, err := q.InspectionTypes(context.Background())
	require.NoError(t, err)
	index := slices.IndexFunc(types, func(typ snapshotdb.InspectionTypesRow) bool { return typ.TypeKey == "fixture.Padding" })
	require.NotEqual(t, -1, index, "missing Padding source type")
	fields, err := q.InspectionFields(context.Background(), snapshotdb.InspectionFieldsParams{TypeID: types[index].ID})
	require.NoError(t, err)
	require.Len(t, fields, 1)
	names, err := q.InspectionFieldNames(context.Background(), snapshotdb.InspectionFieldNamesParams{FieldID: fields[0].ID})
	require.NoError(t, err)
	require.Equal(t, []snapshotdb.InspectionFieldNamesRow{{Ordinal: 0, Name: "_"}, {Ordinal: 1, Name: "_"}}, names)
}
