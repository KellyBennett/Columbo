package columbo

import (
	"strconv"
	"strings"
)

// expansionFixture owns the loaded root and its initial recursion context.
// Tests call the metric entry points with the same stack and trace.
type expansionFixture struct {
	engine *engine
	root   *declaration
	trace  expansion
}

func (t *testHarness) expansionFixture(source string) expansionFixture {
	t.Helper()
	engine, err := load(t.fixture(source), []string{"./..."}, quiet())
	t.require(err == nil, err)
	return newExpansionFixture(engine)
}
func newExpansionFixture(engine *engine) expansionFixture {
	for _, d := range engine.declarations {
		d.measure(engine)
	}
	root := engine.declarations[0]
	return expansionFixture{engine, root, expansion{names: []string{root.symbol}, sites: []Site{}}}
}
func (f expansionFixture) lines() expansionEvidence {
	return expansionEvidence{f.engine.expandedLines(f.root, []*declaration{f.root}, f.trace)}
}
func (f expansionFixture) complexity() (int, expansionEvidence) {
	score, sources := f.engine.expandedComplexity(f.root, []*declaration{f.root}, f.trace, 0)
	return score, expansionEvidence{sources}
}

// expansionEvidence owns the provenance queries used by both virtual metrics.
type expansionEvidence struct{ sources []Source }

func (e expansionEvidence) childPaths(suffix string) map[string]bool {
	paths := map[string]bool{}
	for _, source := range e.sources {
		path := source.Detail.Expansion
		if len(path) > 0 && strings.HasSuffix(path[len(path)-1], suffix) {
			paths[canonical(source.Detail.ExpansionSites)] = true
		}
	}
	return paths
}
func (e expansionEvidence) hasCopiedContribution(value, nesting int) bool {
	for _, source := range e.sources {
		if len(source.Detail.ExpansionSites) > 0 && source.Detail.Value == value && source.Detail.Nesting == nesting {
			return true
		}
	}
	return false
}
func (e expansionEvidence) copyKeys() []string {
	keys := []string{}
	for _, source := range e.sources {
		path := source.Detail.Expansion
		if len(path) == 0 {
			continue
		}
		keys = append(keys, path[len(path)-1]+":"+strconv.Itoa(source.StartLine)+":"+canonical(source.Detail.ExpansionSites))
	}
	return keys
}
