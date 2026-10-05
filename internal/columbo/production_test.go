package columbo

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTestFilesDoNotAffectAnalysis(t *testing.T) {
	h := &testHarness{T: t}
	dir := h.fixture("package fixture\nfunc F(a,b,c,d,e int){}\n")
	c := longParameterConfig()
	before := h.investigate(dir, c)
	require.Len(t, before.Cases, 1)
	for _, fixture := range []struct{ name, source string }{
		{"source_test.go", "package fixture\nimport _ \"missing.example/testonly\"\n//columbo:ignore invalid-smell\nfunc TestOnly(a,b,c,d,e int){undefined()}\n"},
		{"external_test.go", "package fixture_test\nfunc External(a,b,c,d,e int){undefined()}\n"},
		{"broken_test.go", "package fixture\nfunc broken(\n"},
	} {
		h.write(dir, fixture.name, fixture.source)
		require.Equal(t, before, h.investigate(dir, c), fixture.name)
	}
}

func TestTestInterfacesDoNotAffectHelperCandidates(t *testing.T) {
	h := &testHarness{T: t}
	dir := h.fixture("package fixture\ntype Box struct{}\nfunc (Box) work(x int){}\nfunc Parent(b Box){b.work(1)}\n")
	check := func() {
		t.Helper()
		a, err := load(dir, []string{"./..."}, quiet())
		require.NoError(t, err)
		require.Len(t, a.declarations, 2)
		for _, d := range a.declarations {
			if d.fn.Name.Name == "work" {
				require.True(t, d.candidate)
			}
		}
	}
	check()
	h.write(dir, "source_test.go", "package fixture\ntype TestInterface interface{work(int)}\n")
	check()
}

func TestTestSignaturesDoNotSupportDataClumps(t *testing.T) {
	h := &testHarness{T: t}
	dir := h.fixture("package fixture\nfunc A(a int,b bool,c string){}\nfunc B(a int,b bool,c string){}\n")
	c := quiet()
	c.Severity["data-clump"] = "fail"
	before := h.investigate(dir, c)
	require.Empty(t, before.Cases)
	h.write(dir, "source_test.go", "package fixture\nfunc TestOnly(a int,b bool,c string){}\n")
	require.Equal(t, before, h.investigate(dir, c))
}
