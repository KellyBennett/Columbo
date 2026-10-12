package columbo

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInspectionPreservesRepeatedBlankTypes(t *testing.T) {
	root := t.TempDir()
	for name, text := range map[string]string{
		"go.mod": "module fixture\n\ngo 1.25.1\n",
		"a.go":   "package fixture\ntype _ struct{}\ntype _ int\ntype Named struct{}\ntype _ = Named\n",
		"b.go":   "package fixture\ntype _ struct { Value string }\n",
	} {
		require.NoError(t, os.WriteFile(filepath.Join(root, name), []byte(text), 0600))
	}
	input, err := validateInspectionInput(root, "")
	require.NoError(t, err)
	report, err := input.analyze(nil)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "report.sqlite")
	require.NoError(t, WriteSnapshot(path, report, "test"))
	snapshot, err := OpenSnapshot(path)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, snapshot.Close()) })
	types, err := snapshot.Queries().InspectionTypes(context.Background())
	require.NoError(t, err)
	require.Len(t, types, 5)
	seen := map[string]bool{}
	for _, typ := range types {
		require.NotContains(t, seen, typ.TypeKey, "source type keys must be unique")
		seen[typ.TypeKey] = true
		if strings.Contains(typ.TypeKey, "._@") && typ.Alias == 0 {
			require.Equal(t, typ.TypeKey, typ.ResolvedKey)
		}
	}
	require.Contains(t, seen, "fixture.Named")
}
