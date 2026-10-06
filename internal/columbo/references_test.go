package columbo

import (
	"go/parser"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCalleeReferenceNormalization(t *testing.T) {
	for _, source := range []string{
		"Fetch", "(Fetch)", "source.Fetch", "(source).Fetch",
		"Fetch[T]", "source.Fetch[T]", "(source.Fetch)[T,U]",
	} {
		t.Run(source, func(t *testing.T) {
			expr, err := parser.ParseExpr(source)
			require.NoError(t, err)
			id := calleeIdentifier(expr)
			require.NotNil(t, id)
			require.Equal(t, "Fetch", id.Name)
			require.Equal(t, "Fetch", source[int(id.Pos())-1:int(id.End())-1], "retain the original identifier and source range")
		})
	}
}

func TestCalleeReferenceRejectsOtherExpressionForms(t *testing.T) {
	for _, source := range []string{"makeFunc()", "func(){}", "*callback", "values[1:2]", "1", "a+b", "Source{}", "value.(Role)"} {
		t.Run(source, func(t *testing.T) {
			expr, err := parser.ParseExpr(source)
			require.NoError(t, err)
			require.Nil(t, calleeIdentifier(expr))
		})
	}
}
