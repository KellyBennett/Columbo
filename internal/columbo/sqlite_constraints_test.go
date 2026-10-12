package columbo

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSQLiteComparisonNullAlternatives(t *testing.T) {
	comparisons := []Clue{{Kind: "missing-limit", Value: 1, Operator: ">"}, {Kind: "missing-operator", Value: 1, Limit: 2}, {Kind: "list-limit", Value: 1, Limit: []string{"T"}, Operator: ">"}}
	for _, comparison := range comparisons {
		t.Run(comparison.Kind, func(t *testing.T) {
			h := newSQLiteHarness(t)
			report := sqliteTestReport()
			report.Cases[0].Clues = []Clue{comparison}
			h.rejectedFresh(report)
		})
	}
}
func TestSQLiteListUpdateAlternatives(t *testing.T) {
	h := newSQLiteHarness(t)
	h.numericDatabase().Close()
	h.forbiddenListUpdates()
}
func (h *sqliteHarness) forbiddenListUpdates() {
	db := h.mutable()
	for _, statement := range []string{"UPDATE clues SET value_type='integer',numeric_value=1 WHERE kind='repeat'", "UPDATE clue_values SET clue_id=(SELECT id FROM clues WHERE kind='integer')", "UPDATE clues SET limit_type=NULL,limit_value=1,operator='>' WHERE kind='integer'"} {
		_, err := db.Exec(statement)
		require.Error(h.T, err, statement)
	}
}
func TestSQLiteClusterMemberAndPairOwnership(t *testing.T) {
	h := newSQLiteHarness(t)
	fixture := sqliteFixture{ref: DeclarationRef{File: "source.go", Symbol: "fixture.F"}}
	h.write(fixture.clusteredReport())
	h.forbiddenClusterLinks()
}
func (f *sqliteFixture) clusteredReport() Report {
	f.declaration()
	f.primaryCase()
	f.helperDeclaration()
	f.cluster()
	f.pair()
	second := f.report.Cases[0]
	second.ID = "C-second"
	f.report.Cases = append(f.report.Cases, second)
	return f.report
}
func (f *sqliteFixture) helperDeclaration() {
	helper := DeclarationRef{File: f.ref.File, Symbol: "fixture.G"}
	f.report.Declarations = append(f.report.Declarations, DeclarationEvidence{Ref: helper, Source: sqliteTestSource(helper)})
}
func (f *sqliteFixture) cluster() {
	f.report.Cases[0].fixtureCluster(f.ref, f.report.Declarations[1].Ref)
}
func (c *Case) fixtureCluster(primary, helper DeclarationRef) {
	members := []Member{{Helper: primary.Symbol, CallOffset: 1, Declaration: &primary}, {Helper: helper.Symbol, CallOffset: 2, Declaration: &helper}}
	c.Clusters = []Cluster{{Key: "cluster", Owner: primary.Symbol, File: primary.File, Members: members, OwnerDeclaration: &primary}}
}
func (f *sqliteFixture) pair() {
	f.report.Cases[0].fixturePair(f.ref, f.report.Declarations[1].Ref)
}
func (c *Case) fixturePair(primary, helper DeclarationRef) {
	pair := []MemberRef{{ClusterKey: "cluster", Helper: primary, CallOffset: 1}, {ClusterKey: "cluster", Helper: helper, CallOffset: 2}}
	c.Clues = []Clue{{Kind: "dependency-overlap", Value: float64(1), ClusterKey: "cluster", Pair: pair}}
}
func (h *sqliteHarness) forbiddenClusterLinks() {
	db := h.mutable()
	for _, statement := range []string{"UPDATE clues SET case_id='C-second' WHERE case_id='C-first'", "UPDATE clues SET pair_left_member_id=(SELECT MIN(id) FROM cluster_members WHERE case_id='C-second') WHERE case_id='C-first'", "UPDATE cluster_members SET case_id='C-second' WHERE case_id='C-first'", "UPDATE clues SET pair_right_member_id=NULL WHERE pair_left_member_id IS NOT NULL", "UPDATE clues SET pair_right_member_id=pair_left_member_id"} {
		_, err := db.Exec(statement)
		require.Error(h.T, err, statement)
	}
}

func TestSQLiteInspectionSourceIntegrityConstraints(t *testing.T) {
	root := inspectionTestModule(t)
	inspectionSQLWriteFile(t, root, ".columbo.yml", "exclude: []\n")
	inspectionSQLWriteFile(t, root, "other.go", "package fixture\ntype Other struct { value int }\nfunc (*Other) Check() {}\n")
	h := newSQLiteHarness(t)
	h.write(inspectionSQLAnalyze(t, root, ""))
	db := h.mutable()
	for _, tc := range []struct {
		name, statement, failure string
	}{
		{"configuration-hash", "UPDATE source_config SET config_sha256=printf('%064d',0)", "FOREIGN KEY"},
		{"configuration-null-half", "UPDATE source_config SET config_file_id=NULL", "CHECK constraint"},
		{"scan-hash-shape", "UPDATE source_scan SET source_sha256='not-a-sha256'", "CHECK constraint"},
		{"file-selection", "UPDATE source_files SET selected_runtime=0 WHERE ordinary_target=1", "CHECK constraint"},
		{"file-shrink", "UPDATE source_files SET size=0 WHERE file_id=(SELECT id FROM files WHERE path='model.go')", "source file size excludes stored range"},
		{"range-past-file", "UPDATE source_ranges SET end_offset=999999 WHERE id=(SELECT MIN(id) FROM source_ranges)", "source range exceeds file size"},
		{"declaration-past-file", "UPDATE declarations SET end_offset=999999 WHERE symbol='fixture.(*Owner).Update'", "source declaration exceeds file size"},
		{"declaration-cross-file", "UPDATE source_declarations SET file_id=(SELECT id FROM files WHERE path='model.go') WHERE declaration_id=(SELECT id FROM declarations WHERE symbol='fixture.(*Other).Check')", "FOREIGN KEY"},
		{"method-without-receiver", "UPDATE source_declarations SET receiver_type_id=NULL WHERE kind='method'", "CHECK constraint"},
		{"function-with-receiver", "UPDATE source_declarations SET receiver_type_id=(SELECT id FROM source_types WHERE type_key='fixture.Owner') WHERE kind='function'", "CHECK constraint"},
		{"type-cross-file-range", "UPDATE source_types SET range_id=(SELECT id FROM source_ranges WHERE file_id=(SELECT id FROM files WHERE path='other.go') LIMIT 1) WHERE type_key='fixture.Owner'", "FOREIGN KEY"},
		{"field-cross-file-owner", "UPDATE source_fields SET type_id=(SELECT id FROM source_types WHERE type_key='fixture.Other') WHERE type_id=(SELECT id FROM source_types WHERE type_key='fixture.Owner') AND ordinal=3", "FOREIGN KEY"},
		{"field-target-identity", "UPDATE source_fields SET target_type_id=(SELECT id FROM source_types WHERE type_key='fixture.Other') WHERE type_key='fixture.Cell'", "FOREIGN KEY"},
		{"call-cross-file-range", "UPDATE source_calls SET range_id=(SELECT id FROM source_ranges WHERE file_id=(SELECT id FROM files WHERE path='other.go') LIMIT 1)", "FOREIGN KEY"},
		{"call-cross-file-receiver", "UPDATE source_calls SET receiver_range_id=(SELECT id FROM source_ranges WHERE file_id=(SELECT id FROM files WHERE path='other.go') LIMIT 1)", "FOREIGN KEY"},
		{"call-binding-without-receiver", "UPDATE source_calls SET receiver_range_id=NULL,receiver_form='' WHERE binding_range_id IS NOT NULL", "CHECK constraint"},
		{"call-target-orphan", "UPDATE source_calls SET target_declaration_id=999999", "FOREIGN KEY"},
		{"call-receiver-form", "UPDATE source_calls SET receiver_form='' WHERE receiver_range_id IS NOT NULL", "CHECK constraint"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := db.Begin()
			require.NoError(t, err)
			defer tx.Rollback()
			_, err = tx.Exec(tc.statement)
			require.ErrorContains(t, err, tc.failure)
		})
	}
}
