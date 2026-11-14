package columbo

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const varSubjectImports = "package fixture\nimport (\"bytes\";\"errors\";\"fmt\";\"io\";\"net/http\";\"os\";\"sort\";\"strconv\";\"strings\";\"time\")\n"

const varSubjectBody = `{
	var b bytes.Buffer
	if r.Method == "POST" {
		if len(r.URL.Path) > 1 {
			for _, p := range strings.Split(r.URL.Path, "/") {
				if p == "" || p == "." {
					continue
				}
				b.WriteString(p)
			}
		}
	}
	n, err := strconv.Atoi(b.String())
	if err != nil && !errors.Is(err, io.EOF) {
		fmt.Fprintln(os.Stderr, err, time.Now())
		return
	}
	keys := []string{b.String()}
	sort.Strings(keys)
	w.WriteHeader(n)
	next.ServeHTTP(w, r)
}
`

const varSubjectSignature = "(next http.Handler, w http.ResponseWriter, r *http.Request)"

func varSubjectConfig() Config {
	c := Defaults()
	c.History = false
	c.Staged = false
	return c
}

type caseProjection struct {
	smell, verdict, symbol, id string
	start, end                 int
	clues                      []string
}

func projectCases(r Report) []caseProjection {
	out := []caseProjection{}
	for _, c := range r.Cases {
		clues := []string{}
		for _, clue := range c.Clues {
			clues = append(clues, fmt.Sprint(clue.Kind, clue.Subject, clue.Value, clue.Limit, clue.Operator))
		}
		out = append(out, caseProjection{c.Smell, c.Verdict, c.Symbol, c.ID, c.StartLine, c.EndLine, clues})
	}
	return out
}

func smellsOf(r Report) []string {
	out := []string{}
	for _, c := range r.Cases {
		out = append(out, c.Smell+" "+c.Symbol)
	}
	return out
}

func TestFunctionLiteralVariableMatchesItsFunctionTwin(t *testing.T) {
	h := &testHarness{T: t}
	declared := h.investigate(h.fixture(varSubjectImports+"func Handle"+varSubjectSignature+" "+varSubjectBody), varSubjectConfig())
	literal := h.investigate(h.fixture(varSubjectImports+"var Handle = func"+varSubjectSignature+" "+varSubjectBody), varSubjectConfig())
	require.ElementsMatch(t, []string{"long-function fixture.Handle", "high-cognitive-complexity fixture.Handle", "excessive-dependencies fixture.Handle"}, smellsOf(declared))
	require.Equal(t, projectCases(declared), projectCases(literal))
}

func TestGroupedMultiNameAndParenthesizedFunctionLiteralVariablesAreSubjects(t *testing.T) {
	h := &testHarness{T: t}
	src := varSubjectImports + "var (\n\tlimit = 3\n\tgrouped = func" + varSubjectSignature + " " + varSubjectBody + ")\n" +
		"var first, second = (func" + varSubjectSignature + " " + strings.TrimSuffix(varSubjectBody, "\n") + "), func() {}\n"
	r := h.investigate(h.fixture(src), varSubjectConfig())
	require.Contains(t, smellsOf(r), "long-function fixture.grouped")
	require.Contains(t, smellsOf(r), "long-function fixture.first")
	require.NotContains(t, strings.Join(smellsOf(r), "\n"), "fixture.second")
	require.NotContains(t, strings.Join(smellsOf(r), "\n"), "fixture.limit")
}

func TestShortFunctionLiteralVariableHasNoCases(t *testing.T) {
	h := &testHarness{T: t}
	r := h.investigate(h.fixture("package fixture\nvar short = func(x int) int { return x + 1 }\n"), varSubjectConfig())
	require.Empty(t, r.Cases)
}

func TestNestedFunctionLiteralsInOtherInitializersAreNotSubjects(t *testing.T) {
	h := &testHarness{T: t}
	src := varSubjectImports + "var table = map[string]func" + varSubjectSignature + "{\"x\": func" + varSubjectSignature + " " + strings.TrimSuffix(varSubjectBody, "\n") + "}\n"
	r := h.investigate(h.fixture(src), varSubjectConfig())
	require.Empty(t, r.Cases)
}

func TestFunctionLiteralVariableIsNotAHelperCandidate(t *testing.T) {
	h := &testHarness{T: t}
	dir := h.fixture("package fixture\nvar work = func(x int) int { return x + 1 }\nfunc Parent() int { return work(1) }\n")
	a, err := load(dir, []string{"./..."}, quiet())
	require.NoError(t, err)
	require.Len(t, a.declarations, 2)
	for _, d := range a.declarations {
		require.False(t, d.candidate, d.symbol)
	}
	require.Equal(t, "fixture.work", a.declarations[0].symbol)
}

func TestFunctionLiteralVariableAcceptsSuppression(t *testing.T) {
	h := &testHarness{T: t}
	for _, src := range []string{
		varSubjectImports + "//columbo:ignore long-function -- handler stays whole for readability\nvar Handle = func" + varSubjectSignature + " " + varSubjectBody,
		varSubjectImports + "var (\n\t//columbo:ignore long-function -- handler stays whole for readability\n\tHandle = func" + varSubjectSignature + " " + varSubjectBody + ")\n",
	} {
		r := h.investigate(h.fixture(src), varSubjectConfig())
		require.Len(t, r.Suppressions, 1)
		require.True(t, r.Suppressions[0].Applied)
		require.Equal(t, "fixture.Handle", r.Suppressions[0].Symbol)
		require.True(t, h.one(r, "long-function").Suppressed)
	}
}

func TestSuppressionOnMultiNameFunctionLiteralVariableIsFatal(t *testing.T) {
	h := &testHarness{T: t}
	dir := h.fixture("package fixture\n//columbo:ignore long-function -- ambiguous between both names here\nvar a, b = func() {}, func() {}\n")
	a, err := load(dir, []string{"./..."}, varSubjectConfig())
	require.NoError(t, err)
	_, err = a.Analyze()
	require.ErrorContains(t, err, "invalid columbo suppression")
}
