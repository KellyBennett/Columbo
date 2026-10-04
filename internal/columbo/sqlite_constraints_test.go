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
