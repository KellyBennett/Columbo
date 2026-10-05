package columbo

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const dependencyUseFixture = `package fixture
type Input struct{}
type Option struct{}
type Result struct{}
func New(input Input, options ...Option) (Result, error) { return Result{}, nil }
`

func dependencyUseCase(t *testing.T, source string) Case {
	t.Helper()
	h := &testHarness{T: t}
	config := dependencyConfig()
	config.Counts["dependencies"] = 1
	report := h.investigate(h.fixture(source), config)
	for _, finding := range report.Cases {
		if finding.Symbol == "fixture.F" {
			return finding
		}
	}
	t.Fatal("missing fixture.F dependency case")
	return Case{}
}

func useValues(finding Case, origin string) []string {
	for _, clue := range finding.Clues {
		if clue.Kind == "dependency-use-"+origin {
			return clue.Value.([]string)
		}
	}
	return nil
}

func TestDependencyOriginsExposeUnusedOptionsAndDiscardedResults(t *testing.T) {
	finding := dependencyUseCase(t, dependencyUseFixture+`func F(input Input) { _, err := New(input); _ = err }`)
	require.Equal(t, []string{"type:fixture.Input", "type:fixture.Option", "type:fixture.Result"}, dependencySet(finding))
	require.Equal(t, []string{"type:fixture.Option"}, useValues(finding, "signature-only"))
	require.Equal(t, []string{"type:fixture.Result"}, useValues(finding, "discarded"))
	require.Equal(t, []string{"type:fixture.Input"}, useValues(finding, "supplied"))
	require.Nil(t, useValues(finding, "consumed"), "unscored error must not appear in scored origin lists")
}

func dependencySet(finding Case) []string {
	for _, clue := range finding.Clues {
		if clue.Kind == "dependency-set" {
			return clue.Value.([]string)
		}
	}
	return nil
}

func TestDependencyOriginsDistinguishConsumedResultsAndSuppliedOptions(t *testing.T) {
	finding := dependencyUseCase(t, dependencyUseFixture+`func F(input Input) { result, _ := New(input, Option{}); _ = result }`)
	require.Equal(t, []string{"type:fixture.Result"}, useValues(finding, "consumed"))
	require.Equal(t, []string{"type:fixture.Input", "type:fixture.Option"}, useValues(finding, "supplied"))
	require.Nil(t, useValues(finding, "signature-only"))
	require.Len(t, dependencySet(finding), 3, "overlapping origins must not multiply the score")
}

func TestDependencyOriginsClassifyDiscardContexts(t *testing.T) {
	for _, body := range []string{"New()", "_ = New()", "_ = (New())", "var _ = (New())", "go New()", "defer New()"} {
		t.Run(body, func(t *testing.T) {
			finding := dependencyUseCase(t, "package fixture\nimport \"context\"\ntype Result struct{}\nfunc New() Result{return Result{}}\nfunc F(ctx context.Context){"+body+"}")
			require.Equal(t, []string{"type:fixture.Result"}, useValues(finding, "discarded"))
			require.Nil(t, useValues(finding, "consumed"))
		})
	}
}

func TestDependencyOriginsPreserveNamedCallableTupleScoring(t *testing.T) {
	source := "package fixture\nimport \"context\"\ntype First struct{}\ntype Second struct{}\ntype Factory func()(First,Second)\nfunc F(ctx context.Context, make Factory){_,_=make()}"
	finding := dependencyUseCase(t, source)
	require.Equal(t, []string{"type:context.Context", "type:fixture.Factory"}, dependencySet(finding))
	require.Nil(t, useValues(finding, "discarded"), "origins must not introduce previously uncounted identities")
}

func TestDependencyOriginsClassifyEachTupleResult(t *testing.T) {
	source := "package fixture\ntype First struct{}\ntype Second struct{}\nfunc Pair()(First,Second){return First{},Second{}}\nfunc F(){first,_:=Pair();_=first}"
	finding := dependencyUseCase(t, source)
	require.Equal(t, []string{"type:fixture.First"}, useValues(finding, "consumed"))
	require.Equal(t, []string{"type:fixture.Second"}, useValues(finding, "discarded"))
	require.Len(t, dependencySet(finding), 2)
}

func TestDependencyOriginsSurviveSnapshotWithPhysicalSupport(t *testing.T) {
	h := &testHarness{T: t}
	config := dependencyConfig()
	config.Counts["dependencies"] = 1
	report := h.investigate(h.fixture(dependencyUseFixture+`func F(input Input){_,err:=New(input);_=err}`), config)
	db := h.snapshot(report)
	values := h.sqlStrings(db, `SELECT cv.value FROM clues q JOIN clue_values cv ON cv.clue_id=q.id
JOIN declarations d ON d.id=q.declaration_id WHERE d.symbol='fixture.F' AND q.kind='dependency-use-discarded'`)
	require.Equal(t, []string{"type:fixture.Result"}, values)
	count := h.sqlCount(db, `SELECT count(*) FROM clues q JOIN clue_receipts cr ON cr.clue_id=q.id
JOIN source_receipts r ON r.id=cr.receipt_id JOIN declarations d ON d.id=q.declaration_id
WHERE d.symbol='fixture.F' AND q.kind='dependency-use-discarded' AND r.subject='type:fixture.Result'`)
	require.Positive(t, count)
}
