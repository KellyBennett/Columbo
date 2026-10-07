package columbo

import (
	"go/ast"
	"go/parser"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBooleanWalkPreservesLeafPolicy(t *testing.T) {
	expr, err := parser.ParseExpr("(a && rejected) || c")
	require.NoError(t, err)
	for _, strict := range []bool{false, true} {
		var visited []string
		ok := walkBooleanLeaves(expr, func(leaf ast.Expr) bool {
			name := leaf.(*ast.Ident).Name
			visited = append(visited, name)
			return !strict || name != "rejected"
		})
		if strict {
			require.False(t, ok)
			require.Equal(t, []string{"a", "rejected"}, visited)
		} else {
			require.True(t, ok)
			require.Equal(t, []string{"a", "rejected", "c"}, visited)
		}
	}
}

func TestBooleanWalkLeavesNegationToCaller(t *testing.T) {
	expr, err := parser.ParseExpr("!(a && b)")
	require.NoError(t, err)
	count := 0
	require.True(t, walkBooleanLeaves(expr, func(leaf ast.Expr) bool {
		_, ok := leaf.(*ast.UnaryExpr)
		require.True(t, ok)
		count++
		return true
	}))
	require.Equal(t, 1, count)
}
