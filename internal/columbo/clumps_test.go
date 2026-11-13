package columbo

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// clumpSource gives each parameter its own named type, shared by three functions.
func clumpSource(size int) string {
	var source strings.Builder
	source.WriteString("package fixture\n")
	params := clumpParameters(&source, size)
	for _, name := range []string{"One", "Two", "Three"} {
		fmt.Fprintf(&source, "func %s(%s){}\n", name, strings.Join(params, ","))
	}
	return source.String()
}
func clumpParameters(source *strings.Builder, size int) []string {
	params := []string{}
	for i := 0; i < size; i++ {
		name := fmt.Sprintf("A%02d", i)
		fmt.Fprintf(source, "type %s struct{}\n", name)
		params = append(params, fmt.Sprintf("p%d %s", i, name))
	}
	return params
}

const contextClumpSource = `package fixture
import "context"
func Enroll(ctx context.Context, orgID string, sectionID string, studentIDs []string) error { return nil }
func Drop(ctx context.Context, orgID string, sectionID string, studentIDs []string) error { return nil }
func Notify(ctx context.Context, orgID, sectionID string, studentIDs []string) {}
func Rename(ctx context.Context, a string, b string) {}
func Move(ctx context.Context, from string, to string) {}
func Tag(ctx context.Context, name, path, kind string) {}
func Label(ctx context.Context, key, value string) {}
`

func TestClumpIgnoresCommonSignatures(t *testing.T) {
	h := &testHarness{T: t}
	c := h.one(h.investigate(h.fixture(contextClumpSource), clumpConfig()), "data-clump")
	require.Equal(t, []string{"org string", "section string", "studentids []string"}, h.clueValue(c, "clump-parameters"))
	require.Equal(t, []string{"fixture.Drop", "fixture.Enroll", "fixture.Notify"}, supportSymbols(c))
}

func TestClumpMatchesLooselySpelledNames(t *testing.T) {
	h := &testHarness{T: t}
	source := `package fixture
func Enroll(orgID, sectionID, termID string) {}
func Drop(orgId, sectionId, termId string) {}
func Notify(org, section, term string) {}
func Audit(OrgID int, section, term string) {}
`
	c := h.one(h.investigate(h.fixture(source), clumpConfig()), "data-clump")
	require.Equal(t, []string{"org string", "section string", "term string"}, h.clueValue(c, "clump-parameters"))
	require.Equal(t, []string{"fixture.Drop", "fixture.Enroll", "fixture.Notify"}, supportSymbols(c))
}

func TestClumpKeepsBareIDName(t *testing.T) {
	h := &testHarness{T: t}
	source := "package fixture\nfunc A(id, name, kind string){}\nfunc B(ID, name, kind string){}\nfunc C(Id, name, kind string){}\n"
	c := h.one(h.investigate(h.fixture(source), clumpConfig()), "data-clump")
	require.Equal(t, []string{"id string", "kind string", "name string"}, h.clueValue(c, "clump-parameters"))
}

func TestClumpTrimsOnlyACamelCaseIDSuffix(t *testing.T) {
	h := &testHarness{T: t}
	source := "package fixture\nfunc A(avoid, valid, uuid, userUUID string){}\nfunc B(avoid, valid, uuid, userUUID string){}\nfunc C(avoid, valid, uuid, userUUID string){}\n"
	c := h.one(h.investigate(h.fixture(source), clumpConfig()), "data-clump")
	require.Equal(t, []string{"avoid string", "useruuid string", "uuid string", "valid string"}, h.clueValue(c, "clump-parameters"))
}

func TestClumpMatchesNameAndType(t *testing.T) {
	h := &testHarness{T: t}
	source := "package fixture\nfunc A(a,b,c int){}\nfunc B(x,y,z int){}\nfunc C(p,q,r int){}\nfunc D(a,b,c string){}\nfunc E(a,b,c bool){}\n"
	require.Empty(t, h.investigate(h.fixture(source), clumpConfig()).Cases)
}

func TestClumpIgnoresUninformativeParameters(t *testing.T) {
	h := &testHarness{T: t}
	source := "package fixture\nimport \"context\"\nfunc A(ctx context.Context, _ int, a string, b bool){}\nfunc B(ctx context.Context, _ int, a string, b bool){}\nfunc C(ctx context.Context, _ int, a string, b bool){}\nfunc D(int, string, bool){}\nfunc E(int, string, bool){}\nfunc F(int, string, bool){}\n"
	config := clumpConfig()
	config.Counts["data-clump-size"] = 2
	c := h.one(h.investigate(h.fixture(source), config), "data-clump")
	require.Equal(t, []string{"a string", "b bool"}, h.clueValue(c, "clump-parameters"))
}

func TestClumpFamilyReportsSharedCoreOnce(t *testing.T) {
	h := &testHarness{T: t}
	source := `package fixture
func A(a int, b bool, c string, d float64) {}
func B(a int, b bool, c string, d float64) {}
func C(a int, b bool, c string, d float64) {}
func D(a int, b bool, c string, e byte) {}
func E(a int, b bool, c string, e byte) {}
func F(a int, b bool, c string, e byte) {}
`
	c := h.one(h.investigate(h.fixture(source), clumpConfig()), "data-clump")
	require.Equal(t, []string{"a int", "b bool", "c string"}, h.clueValue(c, "clump-parameters"))
	require.Equal(t, []string{"fixture.A", "fixture.B", "fixture.C", "fixture.D", "fixture.E", "fixture.F"}, supportSymbols(c))
}

func TestPrimitiveClumpLeadsTowardNamedTypes(t *testing.T) {
	h := &testHarness{T: t}
	c := h.one(h.investigate(h.fixture(contextClumpSource), clumpConfig()), "data-clump")
	require.Contains(t, c.Leads, primitiveClumpLead)
}

func TestPrimitiveClumpLeadCoversCompositesOfPrimitives(t *testing.T) {
	h := &testHarness{T: t}
	source := "package fixture\nfunc A(a *int, b map[string][]byte, c [2]rune){}\nfunc B(a *int, b map[string][]byte, c [2]rune){}\nfunc C(a *int, b map[string][]byte, c [2]rune){}\n"
	c := h.one(h.investigate(h.fixture(source), clumpConfig()), "data-clump")
	require.Contains(t, c.Leads, primitiveClumpLead)
}

func TestNamedTypeClumpGetsNoPrimitiveLead(t *testing.T) {
	h := &testHarness{T: t}
	source := "package fixture\ntype OrgID string\nfunc A(org OrgID, a, b string){}\nfunc B(org OrgID, a, b string){}\nfunc C(org OrgID, a, b string){}\n"
	c := h.one(h.investigate(h.fixture(source), clumpConfig()), "data-clump")
	require.NotContains(t, c.Leads, primitiveClumpLead)
	require.Equal(t, adviceCatalog["data-clump"].leads, c.Leads)
}

const fieldClumpSource = `package fixture
import "time"
type ContentCatalog struct{}
type sectionGradesLocker interface{ Lock() }
type calcLoader interface{ Load() }
type Recalculator struct {
	repo    sectionGradesLocker
	catalog ContentCatalog
	now     func() time.Time
}
type sectionCalculator struct {
	repo    calcLoader
	catalog ContentCatalog
	now     func() time.Time
}
func NewRecalculator(repo sectionGradesLocker, catalog ContentCatalog, now func() time.Time) *Recalculator {
	return &Recalculator{repo: repo, catalog: catalog, now: now}
}
`

func TestClumpCountsStructFieldsAndMatchesInterfaces(t *testing.T) {
	h := &testHarness{T: t}
	c := h.one(h.investigate(h.fixture(fieldClumpSource), clumpConfig()), "data-clump")
	require.Equal(t, "fixture.Recalculator", c.Symbol)
	require.Equal(t, []string{"catalog fixture.ContentCatalog", "now func() time.Time", "repo interface"}, h.clueValue(c, "clump-parameters"))
	require.Equal(t, []string{"fixture.NewRecalculator", "fixture.Recalculator", "fixture.sectionCalculator"}, supportSymbols(c))
	require.NotContains(t, c.Leads, primitiveClumpLead)
}

func TestSnapshotStoresStructAnchoredClump(t *testing.T) {
	h := &testHarness{T: t}
	db := h.snapshot(h.investigate(h.fixture(fieldClumpSource), clumpConfig()))
	primary := h.sqlStrings(db, `SELECT d.symbol FROM cases c JOIN declarations d ON d.id=c.primary_declaration_id WHERE c.smell='data-clump'`)
	require.Equal(t, []string{"fixture.Recalculator"}, primary)
	fields := h.sqlStrings(db, `SELECT d.symbol || '|' || r.subject FROM source_receipts r JOIN declarations d ON d.id=r.source_declaration_id WHERE r.kind='field' ORDER BY d.symbol, r.ordinal`)
	require.Equal(t, []string{"fixture.Recalculator|0:fixture.sectionGradesLocker", "fixture.Recalculator|1:fixture.ContentCatalog", "fixture.Recalculator|2:func() time.Time", "fixture.sectionCalculator|0:fixture.calcLoader", "fixture.sectionCalculator|1:fixture.ContentCatalog", "fixture.sectionCalculator|2:func() time.Time"}, fields)
}

func TestClumpDoesNotMatchInterfaceWithConcreteType(t *testing.T) {
	h := &testHarness{T: t}
	source := `package fixture
type Loader interface{ Load() }
type Saver interface{ Save() }
type Store struct{}
func A(repo Loader, a, b int) {}
func B(repo Saver, a, b int) {}
func C(repo *Store, a, b int) {}
`
	require.Empty(t, h.investigate(h.fixture(source), clumpConfig()).Cases)
}

func TestClumpIgnoresEmbeddedBlankAndContextFields(t *testing.T) {
	h := &testHarness{T: t}
	source := `package fixture
import "context"
type Base struct{}
type A struct {
	Base
	_   string
	ctx context.Context
	a   int
	b   bool
}
type B struct {
	Base
	_   string
	ctx context.Context
	a   int
	b   bool
}
type C struct {
	Base
	_   string
	ctx context.Context
	a   int
	b   bool
}
`
	require.Empty(t, h.investigate(h.fixture(source), clumpConfig()).Cases)
}

func clumpConfig() Config {
	config := quiet()
	config.Severity["data-clump"] = "fail"
	return config
}

func supportSymbols(c Case) []string {
	out := []string{}
	for _, d := range c.SupportingDeclarations {
		if d.Role == "clump-support" {
			out = append(out, d.Declaration.Symbol)
		}
	}
	sort.Strings(out)
	return out
}
