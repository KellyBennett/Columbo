package columbo

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStatementMutation(t *testing.T) {
	for _, tt := range []struct {
		source         string
		targets, reads []string
		ok             bool
	}{
		{"x = y", []string{"x"}, []string{"y"}, true},
		{"x := y", []string{"x"}, []string{"y"}, true},
		{"x, z = y, w", []string{"x", "z"}, []string{"y", "w"}, true},
		{"x += y", []string{"x"}, []string{"x", "y"}, true},
		{"x++", []string{"x"}, []string{"x"}, true},
		{"x--", []string{"x"}, []string{"x"}, true},
		{"var x = y", nil, nil, false},
		{"f(x)", nil, nil, false},
	} {
		t.Run(tt.source, func(t *testing.T) {
			source := "package p; func f(){" + tt.source + "}"
			fs := token.NewFileSet()
			file, err := parser.ParseFile(fs, "source.go", source, 0)
			require.NoError(t, err)
			stmt := file.Decls[0].(*ast.FuncDecl).Body.List[0]
			m, ok := mutationOf(stmt)
			require.Equal(t, tt.ok, ok)
			spell := func(expressions []ast.Expr) []string {
				var result []string
				for _, expr := range expressions {
					result = append(result, source[fs.Position(expr.Pos()).Offset:fs.Position(expr.End()).Offset])
				}
				return result
			}
			require.Equal(t, tt.targets, spell(m.targets))
			require.Equal(t, tt.reads, spell(m.reads))
		})
	}
}
