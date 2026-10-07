package columbo

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const choicePrelude = `package fixture
const Abstain="abstain"
const Research="research"
const Other="other"
type Candidate struct{ID string; Other string}
type Context struct{Candidates []Candidate; Other []Candidate}
type Input struct{Context Context}
type Case struct{Input Input}
`
const choiceMap = `func request(c Context) map[string]any {
out:=map[string]any{Abstain:"description",Research:"other description"}
for _,v:=range c.Candidates {out[v.ID]=v}
return out
}
`
const choiceSlice = `func menu(in Input) []string {
out:=[]string{Abstain,Research}
for _,v:=range in.Context.Candidates {out=append(out,v.ID)}
return out
}
`
const choiceValidation = `func validate(c Context, selected string) bool {
out:=map[string]bool{Abstain:true,Research:true}
for _,v:=range c.Candidates {out[v.ID]=true}
return out[selected]
}
`
const choiceLabels = `func labels(c Case, selected string) bool {
out:=map[string]bool{Abstain:true,Research:true}
for _,v:=range c.Input.Context.Candidates {out[v.ID]=true}
return out[selected]
}
`

func choicesFixture(t *testing.T, source string) ChoiceSetReport {
	t.Helper()
	h := &testHarness{T: t}
	a, err := load(h.fixture(source), []string{"./..."}, quiet())
	require.NoError(t, err)
	return a.choiceSets()
}
func TestChoiceSetsFourRepresentations(t *testing.T) {
	r := choicesFixture(t, choicePrelude+choiceMap+choiceSlice+choiceValidation+choiceLabels)
	require.Len(t, r.Groups, 1)
	g := r.Groups[0]
	require.Len(t, g.Sites, 4)
	require.Equal(t, "fixture.Context.Candidates", g.Collection)
	require.Equal(t, "fixture.Candidate.ID", g.Projection)
	require.Equal(t, 1, r.Files)
	require.Equal(t, 4, r.Declarations)
	for _, s := range g.Sites {
		require.Len(t, s.Receipts, 2)
		require.Contains(t, s.Receipts[0].Spelling, "Abstain")
		require.Contains(t, s.Receipts[1].Spelling, "range")
		require.Greater(t, s.Receipts[1].EndOffset, s.Receipts[1].StartOffset)
	}
}
func TestChoiceSetsRejectUnsupportedConstructions(t *testing.T) {
	tests := []struct{ name, source string }{
		{"different-seed", strings.Replace(choiceSlice, "Abstain,Research", "Abstain,Other", 1)},
		{"different-field", strings.Replace(choiceSlice, "Context.Candidates", "Context.Other", 1)},
		{"different-projection", strings.Replace(choiceSlice, "v.ID", "v.Other", 1)},
		{"filter", strings.Replace(choiceSlice, "out=append(out,v.ID)", "if v.ID!=Other {out=append(out,v.ID)}", 1)},
		{"break", strings.Replace(choiceSlice, "out=append(out,v.ID)", "out=append(out,v.ID);break", 1)},
		{"mutation", strings.Replace(choiceSlice, "out=append(out,v.ID)", "out=append(out,v.ID);out=nil", 1)},
		{"alias", strings.Replace(choiceSlice, "for _,v", "alias:=out;_=alias;for _,v", 1)},
		{"dynamic-seed", strings.Replace(choiceSlice, "Abstain,Research", "Abstain,in.Context.Candidates[0].ID", 1)},
		{"literal-seed", strings.Replace(choiceSlice, "Abstain,Research", `"abstain","research"`, 1)},
		{"nested-construction", strings.Replace(choiceSlice, "out:=", "if len(in.Context.Candidates)>0 {out:=", 1) + ""},
	}
	for _, test := range tests {
		if test.name == "nested-construction" {
			test.source = strings.Replace(test.source, "return out\n}", "return out};return nil\n}", 1)
		}
		t.Run(test.name, func(t *testing.T) { require.Empty(t, choicesFixture(t, choicePrelude+choiceMap+test.source).Groups) })
	}
}
func TestChoiceSetsNearbyControls(t *testing.T) {
	controls := `func count(action string) int {n:=0;if action==Abstain {n++};if action==Research {n++};return n}
func reserved(id string) bool {return id==Abstain||id==Research}
func present(c Context,id string)bool{for _,v:=range c.Candidates{if v.ID==id{return true}};return false}
func factory(c Context)map[string]any{return request(c)}
type Evidence struct{ID string};type Sources struct{Candidates []Evidence}
func evidence(c Sources)[]string{out:=[]string{Abstain,Research};for _,v:=range c.Candidates{out=append(out,v.ID)};return out}
`
	require.Empty(t, choicesFixture(t, choicePrelude+choiceMap+controls).Groups)
}
func TestChoiceSetsStableIdentity(t *testing.T) {
	base := choicesFixture(t, choicePrelude+choiceMap+choiceSlice).Groups[0]
	renamed := strings.ReplaceAll(choiceMap+choiceSlice, "out", "renamed")
	renamed = strings.ReplaceAll(renamed, "Abstain,Research", "Research,Abstain")
	r := choicesFixture(t, choicePrelude+"\n\n"+renamed)
	require.Equal(t, base.ID, r.Groups[0].ID)
	require.NotEqual(t, base.Sites[1].Seeds, r.Groups[0].Sites[1].Seeds)
	require.Equal(t, canonical(r), canonical(choicesFixture(t, choicePrelude+"\n\n"+renamed)))
}
func TestChoiceSetsOneDeclarationIsNotRepeatedOwnership(t *testing.T) {
	source := `func twice(c Context){a:=[]string{Abstain,Research};for _,v:=range c.Candidates{a=append(a,v.ID)};b:=[]string{Abstain,Research};for _,v:=range c.Candidates{b=append(b,v.ID)};_=a;_=b}`
	require.Empty(t, choicesFixture(t, choicePrelude+source).Groups)
}
func TestChoiceSetsExcludeGeneratedAndTests(t *testing.T) {
	h := &testHarness{T: t}
	dir := h.fixture(choicePrelude + choiceMap)
	h.write(dir, "generated.go", "// Code generated fixture. DO NOT EDIT.\npackage fixture\n"+choiceSlice)
	h.write(dir, "fixture_test.go", "package fixture\n"+choiceValidation)
	a, err := load(dir, []string{"./..."}, quiet())
	require.NoError(t, err)
	require.Empty(t, a.choiceSets().Groups)
}
func TestChoiceSetsCLIAndFreshOutput(t *testing.T) {
	h := &testHarness{T: t}
	dir := h.fixture(choicePrelude + choiceMap + choiceSlice)
	var out, stderr bytes.Buffer
	args := []string{"--no-history", "--output", "report.sqlite", "--choice-sets-output", "choices.json"}
	code := Run(args, Invocation{Dir: dir, Stdout: &out, Stderr: &stderr})
	require.NotEqual(t, 2, code, stderr.String())
	data, err := os.ReadFile(filepath.Join(dir, "choices.json"))
	require.NoError(t, err)
	var report ChoiceSetReport
	require.NoError(t, json.Unmarshal(data, &report))
	require.Len(t, report.Groups, 1)
	require.Contains(t, out.String(), "1 groups; no verdict")
	err = writeChoiceSets(filepath.Join(dir, "choices.json"), ChoiceSetReport{})
	require.Error(t, err)
	again, err := os.ReadFile(filepath.Join(dir, "choices.json"))
	require.NoError(t, err)
	require.Equal(t, data, again)
	var normal, normalErr bytes.Buffer
	without := Run([]string{"--no-history", "--output", "normal.sqlite"}, Invocation{Dir: dir, Stdout: &normal, Stderr: &normalErr})
	require.Equal(t, code, without)
	require.NotContains(t, normal.String(), "Choice-set evidence")
}
