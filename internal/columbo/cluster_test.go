package columbo

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"strings"
)

type clusterBoundaryCase struct {
	name, body string
	want       bool
}

var clusterBoundaryCases = []clusterBoundaryCase{
	{"ordinary", "one(a,b);two(a,b)", true},
	{"assignments", "one(a,b);x:=1;_ = x;two(a,b)", true},
	{"opposite-if", "if a>0 {one(a,b)} else {two(a,b)}", false},
	{"loop-boundary", "one(a,b);for a>0 {two(a,b)}", false},
	{"return-boundary", "one(a,b);return;two(a,b)", false},
	{"go-boundary", "go one(a,b);two(a,b)", false},
	{"defer-boundary", "defer one(a,b);two(a,b)", false},
	{"literal-cluster-exclusion", "_ = func(){one(a,b);two(a,b)}", false},
	{"builtin-exception", "one(a,b);println(a);two(a,b)", true},
}

func (t *testHarness) checkClusterBoundary(c clusterBoundaryCase) {
	t.Helper()
	r := t.investigate(t.fixture(c.source()), cosmeticConfig())
	t.require((len(r.Cases) > 0) == c.want, r)
	if len(r.Cases) == 0 {
		return
	}
	finding := t.one(r, "cosmetic-extraction")
	t.reconcile(finding, "expanded-lines")
	t.reconcile(finding, "expanded-complexity")
}
func (c clusterBoundaryCase) source() string {
	return "package fixture\nfunc Parent(a,b int){" + c.body + "}\n" + helpers
}
func orderedClusterSource() string {
	return "package fixture\nfunc Parent(a,b int){one(a,b);two(a,b);if a>0{};three(a,b);four(a,b)}\n" + helpers + strings.ReplaceAll(strings.ReplaceAll(helpers, "one", "three"), "two", "four")
}

// Ownership evidence retains every reported cluster owner and parent input.
type clusterOwnershipEvidence struct {
	owners []string
	inputs []string
}

func clusterOwnership(c Case) clusterOwnershipEvidence {
	evidence := clusterOwnershipEvidence{}
	for _, cluster := range c.Clusters {
		evidence.owners = append(evidence.owners, cluster.Owner)
	}
	for _, clue := range c.Clues {
		if clue.Kind == "parent-input-set" {
			evidence.inputs = append(evidence.inputs, clue.Value.([]string)...)
		}
	}
	return evidence
}
func (t *testHarness) requireOwnedCluster(c Case, owner string) {
	t.Helper()
	evidence := clusterOwnership(c)
	require.Equal(t.T, []string{owner}, evidence.owners)
	require.NotEmpty(t.T, evidence.inputs, "missing parent-input evidence")
	for _, input := range evidence.inputs {
		require.True(t.T, strings.HasPrefix(input, owner+":"), input)
	}
}

// reportDocument tests the serialized schema rather than the in-memory structs.
func (t *testHarness) reportDocument(r Report) map[string]any {
	t.Helper()
	data, err := Serialize(r, "json")
	t.require(err == nil, err)
	var document map[string]any
	err = json.Unmarshal(data, &document)
	t.require(err == nil, err)
	return document
}
