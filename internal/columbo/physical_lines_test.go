package columbo

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestLineDirectivesKeepMetricReceiptsPhysical(t *testing.T) {
	h := &testHarness{T: t}
	dir := h.fixture("package fixture\n//line pretend.go:900\nfunc Example(x bool) int {\n if x { return 1 }\n return 0\n}\n")
	a, err := load(dir, []string{"./..."}, quiet())
	require.NoError(t, err)
	_, err = a.Analyze()
	require.NoError(t, err)
	require.Len(t, a.declarations, 1)
	d := a.declarations[0]
	require.Equal(t, 2, d.lines)
	require.Len(t, d.lineReceipts, 2)
	require.Equal(t, 4, d.lineReceipts[0].StartLine)
	require.Equal(t, 5, d.lineReceipts[1].StartLine)
	require.Equal(t, d.file.rel, d.lineReceipts[0].File)
}
