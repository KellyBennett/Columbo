package columbo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const variantPrelude = `package fixture
type Kind string
const (A Kind = "a"; B Kind = "b"; C Kind = "c"; D Kind = "d"; Alias = A)
type Source struct { Kind Kind }
`

func variantConfig() Config {
	config := quiet()
	config.Severity[variantSmell] = "fail"
	return config
}

func variantFunction(name, body string) string {
	return "func " + name + "(x Kind) {" + body + "}\n"
}

func variantSwitch(values string) string {
	return "switch x {case " + values + ": println(x)}"
}

func TestVariantAcceptance(t *testing.T) {
	cases := []struct {
		name, source    string
		repeated, sites int
	}{
		{"variant-exact-switch-repeat", variantFunction("Fetch", variantSwitch("A,B")) + variantFunction("Check", variantSwitch("A,B")), 2, 2},
		{"variant-different-bodies", variantFunction("Fetch", "switch x {case A: println(1); case B: for range 5 {println(2)}}") + variantFunction("Check", "switch x {case A: _ = len(string(x)); case B: return}"), 2, 2},
		{"variant-partial-overlap", variantFunction("First", variantSwitch("A,B,C")) + variantFunction("Second", variantSwitch("A,B,D")), 2, 2},
		{"variant-insufficient-overlap", variantFunction("First", variantSwitch("A,B")) + variantFunction("Second", variantSwitch("B,C")), 0, 0},
		{"variant-three-way-support", variantFunction("First", variantSwitch("A,B")) + variantFunction("Second", variantSwitch("A,C")) + variantFunction("Third", variantSwitch("B,C")), 3, 3},
		{"variant-single-factory", variantFunction("Factory", variantSwitch("A,B,C,D")), 0, 0},
		{"variant-switch-if-equivalence", variantFunction("First", variantSwitch("A,B")) + variantFunction("Second", "if x == A {} else if x == B {} else {}"), 2, 2},
		{"variant-if-operand-order", variantFunction("First", "if x == A {} else if B == x {}") + variantFunction("Second", "if ((A)) == (x) {} else if (x) == B {}"), 2, 2},
		{"variant-if-or", variantFunction("First", "if x == A || B == x {}") + variantFunction("Second", "if x == B || A == x {}"), 2, 2},
		{"variant-if-mixed-condition", variantFunction("First", variantSwitch("A,B")) + variantFunction("Second", "if x == A {} else if x == B && len(x)>0 {}"), 0, 0},
		{"variant-if-mixed-tail", variantFunction("First", variantSwitch("A,B")) + variantFunction("Second", "if x != C {} else if x == A {} else if x == B {}"), 0, 0},
		{"variant-guards", variantFunction("First", variantSwitch("A,B")) + variantFunction("Second", "if x == A {return}; if x == B {return}"), 0, 0},
		{"variant-function-discriminant", variantFunction("First", variantSwitch("A,B")) + variantFunction("Second", "if func() Kind {return x}() == A {} else if x == B {}"), 0, 0},
		{"variant-constant-alias", variantFunction("First", variantSwitch("A,B")) + variantFunction("Second", variantSwitch("Alias,B")), 2, 2},
		{"variant-alias-not-distinct", variantFunction("First", "if x == A || x == Alias {}") + variantFunction("Second", variantSwitch("A,B")), 0, 0},
		{"variant-renamed-local", variantFunction("First", variantSwitch("A,B")) + "func Second(other Kind){switch other {case A,B:}}", 2, 2},
		{"variant-selector-chain", "func First(s Source){if s.Kind == A {} else if s.Kind == B {}}\nfunc Second(s Source){switch s.Kind {case A,B:}}", 2, 2},
		{"variant-shadowed-local", variantFunction("First", variantSwitch("A,B")) + variantFunction("Second", "if x == A {} else if x:=B; x == B {}"), 0, 0},
		{"variant-default", variantFunction("First", "switch x {case A:;default:}") + variantFunction("Second", variantSwitch("A,B")), 0, 0},
		{"variant-runtime-case", variantFunction("First", variantSwitch("A,B")) + variantFunction("Second", "y:=B; switch x {case A:;case y:}"), 0, 0},
		{"variant-raw-string", "func First(x string){switch x {case \"a\",\"b\":}}\nfunc Second(x string){switch x {case \"a\",\"b\":}}", 0, 0},
		{"variant-bool", "type Flag bool\nfunc First(x Flag){if x == true {} else if x == false {}}\nfunc Second(x Flag){switch x {case true,false:}}", 0, 0},
		{"variant-int", "type Code int\nconst(One Code=1; Two Code=2)\nfunc First(x Code){switch x {case One,Two:}}\nfunc Second(x Code){if x == 1 {} else if x == Two {}}", 2, 2},
		{"variant-float", "type Number float64\nfunc First(x Number){switch x {case 1,2:}}\nfunc Second(x Number){switch x {case 1,2:}}", 0, 0},
		{"variant-nested-literal", variantFunction("First", "_ = func(){"+variantSwitch("A,B")+"}") + variantFunction("Second", variantSwitch("A,B")), 2, 2},
		{"variant-reasonable-enum-mapping", "func Label(x Kind) string {switch x {case A:return \"First\";case B:return \"Second\"};return \"\"}\nfunc Rank(x Kind) int {switch x {case A:return 10;case B:return 20};return 0}", 2, 2},
	}
	for _, scenario := range cases {
		t.Run(scenario.name, func(t *testing.T) {
			h := &testHarness{T: t}
			report := h.investigate(h.fixture(variantPrelude+scenario.source), variantConfig())
			if scenario.repeated == 0 {
				require.Empty(t, report.Cases)
				return
			}
			c := h.one(report, variantSmell)
			require.Equal(t, scenario.repeated, h.clueValue(c, "repeated-variant-count"))
			require.Equal(t, scenario.sites, h.clueValue(c, "variant-decision-sites"))
			require.Equal(t, []PolicyReview{variantPolicy}, c.PolicyReviews)
			require.Equal(t, "FAIL", c.Verdict)
		})
	}
}

const typeVariantPrelude = `package fixture
type Payment interface{ Pay() }
type Card struct{}
func (Card) Pay(){}
type Cash struct{}
func (*Cash) Pay(){}
type Alias = Card
`

func TestVariantTypeSwitches(t *testing.T) {
	cases := []struct {
		name, role, arms string
		want             bool
	}{
		{"variant-type-switch-repeat", "Payment", "Card,*Cash", true},
		{"variant-type-nil", "Payment", "Card,*Cash,nil", true},
		{"variant-type-alias", "Payment", "Alias,*Cash", true},
		{"variant-type-any", "any", "Card,*Cash", false},
		{"variant-type-anonymous", "interface{Pay()}", "Card,*Cash", false},
		{"variant-type-interface-arm", "Payment", "Card,*Cash,Payment", false},
		{"variant-type-unnamed-arm", "any", "Card,*Cash,[]int", false},
	}
	for _, scenario := range cases {
		t.Run(scenario.name, func(t *testing.T) {
			h := &testHarness{T: t}
			source := typeVariantPrelude + "func First(p " + scenario.role + "){switch p.(type){case " + scenario.arms + ":}}\nfunc Second(p " + scenario.role + "){switch x:=p.(type){case " + scenario.arms + ": _ = x;default:}}"
			report := h.investigate(h.fixture(source), variantConfig())
			require.Len(t, report.Cases, map[bool]int{true: 1, false: 0}[scenario.want])
			if scenario.want {
				require.Equal(t, 2, h.clueValue(report.Cases[0], "repeated-variant-count"))
			}
		})
	}
}

func TestVariantThresholdsAndSeverity(t *testing.T) {
	h := &testHarness{T: t}
	dir := h.fixture(variantPrelude + variantFunction("First", variantSwitch("A,B")) + variantFunction("Second", variantSwitch("A,B")))
	config := variantConfig()
	require.Equal(t, "fail", Defaults().Severity[variantSmell])
	for _, key := range []string{"repeated-variant-sites", "repeated-variant-variants"} {
		config.Counts[key] = 3
		require.Empty(t, h.investigate(dir, config).Cases)
		config.Counts[key] = 2
		require.Len(t, h.investigate(dir, config).Cases, 1)
		for _, value := range []string{"0", "1", "2.5", "true"} {
			_, err := decodeConfig([]byte("thresholds:\n  "+key+": "+value+"\n"), Defaults())
			require.Error(t, err)
		}
	}
	original := h.one(h.investigate(dir, config), variantSmell)
	config.Severity[variantSmell] = "warn"
	warning := h.one(h.investigate(dir, config), variantSmell)
	require.Equal(t, original.ID, warning.ID)
	require.Equal(t, "WARN", warning.Verdict)
	config.Severity[variantSmell] = "off"
	require.Empty(t, h.investigate(dir, config).Cases)
}

func TestVariantIdentityAndCentralization(t *testing.T) {
	h := &testHarness{T: t}
	dir := h.fixture(variantPrelude + variantFunction("First", variantSwitch("A,B")) + variantFunction("Second", variantSwitch("A,B")))
	config := variantConfig()
	original := h.one(h.investigate(dir, config), variantSmell)
	h.write(dir, "source.go", variantPrelude)
	h.write(dir, "relocated.go", "package fixture\n\nfunc Renamed(other Kind){if other == B || Alias == other {}}\nfunc Another(source Kind){switch source {case B,A:}}")
	changed := h.one(h.investigate(dir, config), variantSmell)
	require.Equal(t, original.ID, changed.ID)
	require.Equal(t, h.clueValue(original, "repeated-variant-set"), h.clueValue(changed, "repeated-variant-set"))
	h.write(dir, "relocated.go", "package fixture\n"+variantFunction("First", variantSwitch("A,B,C"))+variantFunction("Second", variantSwitch("A,B,C")))
	require.Equal(t, original.ID, h.one(h.investigate(dir, config), variantSmell).ID)
	h.write(dir, "relocated.go", "package fixture\n"+variantFunction("Factory", variantSwitch("A,B,C")))
	require.Empty(t, h.investigate(dir, config).Cases)
	h.write(dir, "relocated.go", "package fixture\nvar labels = map[Kind]string{A: \"First\",B: \"Second\"}\nfunc Label(x Kind) string{return labels[x]}")
	require.Empty(t, h.investigate(dir, config).Cases)
}

func TestVariantScope(t *testing.T) {
	h := &testHarness{T: t}
	dir := h.fixture(variantPrelude + variantFunction("First", variantSwitch("A,B")))
	second := "package fixture\n" + variantFunction("Second", variantSwitch("A,B"))
	for _, excluded := range []struct{ path, prefix string }{
		{"extra_generated.go", ""}, {"second_test.go", ""}, {"generated.go", "// Code generated by fixture. DO NOT EDIT.\n"}, {"unselected.go", "//go:build never\n\n"},
	} {
		h.write(dir, excluded.path, excluded.prefix+strings.ReplaceAll(second, "Second", "Second"+strings.ReplaceAll(excluded.path, ".", "_")))
	}
	require.Empty(t, h.investigate(dir, variantConfig()).Cases)
	h.write(dir, "second.go", second)
	config := variantConfig()
	config.Exclude = append(config.Exclude, "second.go")
	require.Empty(t, h.investigate(dir, config).Cases)
	require.Len(t, h.investigate(dir, variantConfig()).Cases, 1)
}

func TestVariantSuppression(t *testing.T) {
	h := &testHarness{T: t}
	first := variantFunction("First", variantSwitch("A,B"))
	second := variantFunction("Second", variantSwitch("A,B"))
	directive := "//columbo:ignore repeated-variant-decision -- separate mappings retained for review\n"
	dir := h.fixture(variantPrelude + first + directive + second)
	config := variantConfig()
	report := h.investigate(dir, config)
	require.Equal(t, 1, report.Summary.Failed)
	require.False(t, report.Suppressions[0].Applied)
	h.write(dir, "source.go", variantPrelude+directive+first+second)
	suppressed := h.investigate(dir, config)
	require.Equal(t, 1, suppressed.Summary.Suppressed)
	require.Equal(t, report.Cases[0].ID, suppressed.Cases[0].ID)
}

func TestVariantEvidence(t *testing.T) {
	h := &testHarness{T: t}
	source := variantPrelude + variantFunction("First", "switch x {case A:;case B:}") + variantFunction("Second", "if x == Alias || x == B {}")
	dir := h.fixture(source)
	report := h.investigate(dir, variantConfig())
	c := h.one(report, variantSmell)
	require.Len(t, c.SupportingDeclarations, 2)
	require.Len(t, c.Receipts, 6)
	for _, receipt := range c.Receipts {
		s := receipt.(Source)
		require.NotNil(t, s.Declaration)
		if s.Kind == "variant-arm" {
			require.Equal(t, s.Spelling, string(h.read(filepath.Join(dir, s.File))[s.StartOffset:s.EndOffset]))
		}
	}
	require.Equal(t, canonical(report), canonical(h.investigate(dir, variantConfig())))
	db := h.snapshot(report)
	require.Equal(t, 6, h.sqlCount(db, "SELECT COUNT(*) FROM source_receipts WHERE case_id=?", c.ID))
	require.Equal(t, 4, h.sqlCount(db, "SELECT COUNT(*) FROM source_receipts WHERE case_id=? AND spelling<>''", c.ID))
	text, exit, err := RenderSnapshot(testDatabaseQueries(db), "report.sqlite")
	require.NoError(t, err)
	require.Equal(t, 1, exit)
	for _, snippet := range []string{"value:fixture.Kind", "variant decision source.go:", "variant-support", "repeated-variant-set", "RVD-001", "in fixture.Second"} {
		require.Contains(t, string(text), snippet)
	}
	require.Equal(t, 2, strings.Count(string(text), "variant decision "))
}

func TestVariantGoldens(t *testing.T) {
	for _, severity := range []string{"fail", "warn", "off", "suppressed"} {
		t.Run(severity, func(t *testing.T) {
			h := &testHarness{T: t}
			source := variantPrelude
			if severity == "suppressed" {
				source += "//columbo:ignore repeated-variant-decision -- independent mappings accepted for fixture\n"
			}
			source += variantFunction("First", variantSwitch("A,B,C"))
			source += strings.ReplaceAll(strings.TrimPrefix(typeVariantPrelude, "package fixture\n"), "type Alias = Card", "type CardAlias = Card") + "func FirstPayment(p Payment){switch p.(type){case Card,*Cash:}}\nfunc SecondPayment(p Payment){switch p.(type){case Card,*Cash:}}\n"
			dir := h.fixture(source)
			h.write(dir, "zsecond.go", "package fixture\n"+variantFunction("Second", "if x == Alias || x == B {}"))
			config := variantConfig()
			if severity != "suppressed" {
				config.Severity[variantSmell] = severity
			}
			report := h.investigate(dir, config)
			for i := range report.Cases {
				report.Cases[i].Receipts = append(report.Cases[i].Receipts, History{Kind: "history", Commit: strings.Repeat("a", 40), CommittedAt: 1, Files: sortedSet(report.Cases[i].sourceFiles())})
			}
			h.snapshotGoldens(report, filepath.Join("testdata", "variant-"+severity))
		})
	}
}

func TestVariantDuplicateIndependence(t *testing.T) {
	h := &testHarness{T: t}
	source := variantPrelude + variantFunction("First", "switch x {case A: println(1);case B: println(2)}") + variantFunction("Second", "if x==A {return} else if x==B {_=len(string(x))}")
	config := variantConfig()
	config.Severity["duplicate-code"] = "fail"
	config.Counts["duplicate-tokens"] = 50
	report := h.investigate(h.fixture(source), config)
	require.Len(t, report.Cases, 1)
	require.Equal(t, variantSmell, report.Cases[0].Smell)
	body := "switch x {case A: for i:=0;i<10;i++ {println(i);println(i+1);println(i+2)};case B: for i:=0;i<20;i++ {println(i);println(i-1);println(i-2)}}"
	report = h.investigate(h.fixture(variantPrelude+variantFunction("First", body)+variantFunction("Second", body)), config)
	require.NotEmpty(t, h.one(report, "duplicate-code"))
	require.NotEmpty(t, h.one(report, variantSmell))
}

func TestVariantGenericDomains(t *testing.T) {
	h := &testHarness{T: t}
	source := `package fixture
type Kind[T any] string
func First(x Kind[int]){switch x {case "a","b":}}
func Second(x Kind[int]){if x == "b" || x == "a" {}}
func Other(x Kind[string]){switch x {case "a","b":}}
type Role[T any] interface{Value() T}
type Left[T any] struct{}
func (Left[T]) Value() T {var zero T;return zero}
type Right[T any] struct{}
func (*Right[T]) Value() T {var zero T;return zero}
func One(x Role[int]) {switch x.(type){case Left[int],*Right[int]:}}
func Two(x Role[int]) {switch x.(type){case Left[int],*Right[int]:}}
func Three(x Role[string]) {switch x.(type){case Left[string],*Right[string]:}}
`
	report := h.investigate(h.fixture(source), variantConfig())
	require.Len(t, report.Cases, 2)
	domains := []string{}
	for _, c := range report.Cases {
		domains = append(domains, c.Clues[0].Subject)
	}
	require.Contains(t, domains, "value:fixture.Kind[int]")
	require.Contains(t, domains, "type:fixture.Role[int]")
}

func TestVariantAliasReceiptsAndSupport(t *testing.T) {
	h := &testHarness{T: t}
	dir := h.fixture(variantPrelude + variantFunction("First", "if x==A || x==Alias || x==B {}") + variantFunction("Second", variantSwitch("A,B")))
	c := h.one(h.investigate(dir, variantConfig()), variantSmell)
	arms := []string{}
	for _, receipt := range c.Receipts {
		s := receipt.(Source)
		if s.Kind == "variant-arm" {
			arms = append(arms, s.Spelling)
		}
	}
	require.ElementsMatch(t, []string{"A", "Alias", "B", "A", "B"}, arms)
	for _, clue := range c.Clues {
		if clue.Kind == "variant-support" {
			require.Equal(t, 2, clue.Value)
		}
	}
}

func TestVariantHigherThresholdBoundaries(t *testing.T) {
	h := &testHarness{T: t}
	dir := h.fixture(variantPrelude + variantFunction("First", variantSwitch("A,B,C")) + variantFunction("Second", variantSwitch("A,B,C")))
	config := variantConfig()
	config.Counts["repeated-variant-sites"] = 3
	config.Counts["repeated-variant-variants"] = 3
	require.Empty(t, h.investigate(dir, config).Cases)
	h.write(dir, "third.go", "package fixture\n"+variantFunction("Third", variantSwitch("A,B")))
	require.Empty(t, h.investigate(dir, config).Cases)
	h.write(dir, "third.go", "package fixture\n"+variantFunction("Third", variantSwitch("A,B,C")))
	c := h.one(h.investigate(dir, config), variantSmell)
	require.Equal(t, 3, h.clueValue(c, "variant-decision-sites"))
	require.Equal(t, 3, h.clueValue(c, "repeated-variant-count"))
}

func TestVariantImportedAliasesAndNamedEmptyInterface(t *testing.T) {
	h := &testHarness{T: t}
	dir := h.fixture("package fixture\nimport first \"fixture/domain\"\ntype Alias = first.Kind\nfunc First(x Alias){switch x{case first.A,first.B:}}")
	require.NoError(t, os.Mkdir(filepath.Join(dir, "domain"), 0700))
	h.write(filepath.Join(dir, "domain"), "domain.go", "package domain\ntype Kind string\nconst(A Kind=\"a\"; B Kind=\"b\")")
	h.write(dir, "second.go", "package fixture\nimport second \"fixture/domain\"\nfunc Second(y second.Kind){if y == second.B || second.A == y {}}")
	c := h.one(h.investigate(dir, variantConfig()), variantSmell)
	require.Equal(t, "value:fixture/domain.Kind", c.Clues[0].Subject)
	source := `package fixture
type Role interface{}
type A struct{}
type B struct{}
func First(x Role){switch x.(type){case A,B:}}
func Second(x Role){switch x.(type){case A,B,struct{}:}}
`
	require.Empty(t, h.investigate(h.fixture(source), variantConfig()).Cases)
}

func TestVariantHistoryUsesAllSupportingFiles(t *testing.T) {
	h := &testHarness{T: t}
	fixture := h.committedFixture(variantPrelude + variantFunction("First", variantSwitch("A,B")))
	h.write(fixture.dir, "second.go", "package fixture\n"+variantFunction("Second", variantSwitch("A,B")))
	fixture.commit(h, "supporting site")
	config := variantConfig()
	baseline := h.one(h.investigate(fixture.dir, config), variantSmell)
	config.History = true
	report := h.investigate(fixture.dir, config)
	c := h.one(report, variantSmell)
	require.Equal(t, baseline.ID, c.ID)
	history := historyReceipts(c)
	require.Len(t, history, 2)
	files := []string{}
	for _, receipt := range history {
		files = append(files, receipt.Files...)
	}
	require.ElementsMatch(t, []string{"source.go", "second.go"}, files)
	db := h.snapshot(report)
	require.Equal(t, 2, h.sqlCount(db, "SELECT COUNT(*) FROM case_history_files WHERE case_id=?", c.ID))
}

func TestVariantSiteOrderUsesPhysicalOffsets(t *testing.T) {
	first := metric("variant-set", "same.go:9", []string{"a", "b"})
	second := metric("variant-set", "same.go:10", []string{"a", "b"})
	require.True(t, first.before(second))
	require.False(t, second.before(first))
}
