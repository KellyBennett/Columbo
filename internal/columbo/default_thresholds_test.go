package columbo

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func defaultsWith(smell string) Config {
	c := quiet()
	c.Severity[smell] = "fail"
	return c
}

func (t *testHarness) countCases(r Report, smell string) int {
	n := 0
	for _, c := range r.Cases {
		if c.Smell == smell {
			n++
		}
	}
	return n
}

const sevenPackages = `os.Getenv("X");strings.TrimSpace("");strconv.Itoa(1);filepath.Clean("");fmt.Sprint();math.Abs(1);sort.Ints(nil)`

func TestDefaultDependencyLimitAllowsSevenAndFailsEight(t *testing.T) {
	h := &testHarness{T: t}
	imports := `package fixture
import("fmt";"math";"os";"path/filepath";"sort";"strconv";"strings";"unicode")
var _ = unicode.IsUpper
`
	seven := h.investigate(h.fixture(imports+"func F(){"+sevenPackages+"}\n"), defaultsWith("excessive-dependencies"))
	require.Zero(t, h.countCases(seven, "excessive-dependencies"), "seven dependencies must pass at the default limit")
	eight := h.investigate(h.fixture(imports+"func F(){"+sevenPackages+";unicode.IsUpper('a')}\n"), defaultsWith("excessive-dependencies"))
	c := h.one(eight, "excessive-dependencies")
	require.Equal(t, 8, h.clueValue(c, "dependencies"))
}

func TestDefaultFeatureEnvyAllowsFiveForeignReadsAndFailsSix(t *testing.T) {
	h := &testHarness{T: t}
	types := "package fixture\ntype T struct{A,B,C,D,E,F int}\n"
	five := h.investigate(h.fixture(types+"func F(t T)int{return t.A+t.B+t.C+t.D+t.E}\n"), defaultsWith("feature-envy"))
	require.Zero(t, h.countCases(five, "feature-envy"), "five foreign reads must pass at the default limit")
	six := h.investigate(h.fixture(types+"func F(t T)int{return t.A+t.B+t.C+t.D+t.E+t.F}\n"), defaultsWith("feature-envy"))
	h.one(six, "feature-envy")
}

// The helpers share two of their three dependencies, the shape of cobra's writePreamble split.
const twoOfThreeSharedDependencies = `package fixture
type B struct{}
type C struct{}
type D struct{}
func Parent(a,b int){one(a,b);two(a,b)}
func one(a,b int){var y C;var z D;_,_=y,z;println(a);println(b)}
func two(a,b int){var x B;var y C;var z D;_,_,_=x,y,z;println(a);println(b)}`

func TestDefaultCosmeticOverlapCatchesTwoOfThreeSharedDependencies(t *testing.T) {
	h := &testHarness{T: t}
	c := defaultsWith("cosmetic-extraction")
	c.Counts["function-lines"] = 1
	found := h.one(h.investigate(h.fixture(twoOfThreeSharedDependencies), c), "cosmetic-extraction")
	for _, clue := range found.Clues {
		if clue.Kind == "dependency-overlap" {
			require.InDelta(t, 2.0/3, clue.Value, .001, "the fixture must sit between the old 0.75 and the default 0.5")
		}
	}
}
