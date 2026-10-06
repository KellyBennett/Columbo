package columbo

import (
	"go/ast"
	"go/token"
	"testing"

	"github.com/stretchr/testify/require"
)

const valuePathSource = `package fixture
import "time"
type Kind string
const A Kind = "a"
type Inner struct{Kind Kind}
type Source struct{Kind Kind; Inner Inner; Other Inner}
func (Source) Method(){}
func Make() Source{return Source{}}
func Probe(s *Source,items []Source){
 _=s.Kind
 _=(s).Kind
 _=(*s).Kind
 _=(&s.Inner).Kind
 _=s.Inner.Kind
 _=s.Other.Kind
 _=Make().Kind
 _=items[0].Kind
 _=s.Method
 _=time.Monday
 _=A
 {s:=s; _=s.Kind}
}
`

func assignmentValuePaths(t *testing.T, normalize func(ast.Expr) ast.Expr) []resolvedValuePath {
	t.Helper()
	h := &testHarness{T: t}
	engine, err := load(h.fixture(valuePathSource), []string{"./..."}, quiet())
	require.NoError(t, err)
	paths := []resolvedValuePath{}
	file := engine.files[0]
	ast.Inspect(file.ast, func(node ast.Node) bool {
		if assignment, ok := node.(*ast.AssignStmt); ok && assignment.Tok == token.ASSIGN {
			paths = append(paths, resolveValuePath(file.typeInfo(), assignment.Rhs[0], normalize))
		}
		return true
	})
	return paths
}

func TestResolvedValuePathPreservesNormalizationBoundaries(t *testing.T) {
	for _, scenario := range []struct {
		name      string
		normalize func(ast.Expr) ast.Expr
		valid     []int
	}{
		{"strict variant grammar", ast.Unparen, []int{0, 1, 4, 5, 11}},
		{"feature envy", valueBase, []int{0, 1, 2, 3, 4, 5, 11}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			paths := assignmentValuePaths(t, scenario.normalize)
			require.Len(t, paths, 12)
			for i, path := range paths {
				require.Equal(t, containsPathIndex(scenario.valid, i), path.valid(), "expression %d", i)
			}
			require.True(t, paths[0].same(paths[1]), "parentheses preserve identity")
			require.False(t, paths[0].same(paths[11]), "shadowed variables are different roots")
			require.False(t, paths[4].same(paths[5]), "distinct field paths retain their resolved objects")
			require.Equal(t, "source.Inner.Kind", paths[4].key("source"))
			if scenario.name == "feature envy" {
				require.True(t, paths[0].same(paths[2]), "dereferencing preserves the root")
				require.True(t, paths[3].same(paths[4]), "address-taking inside a chain preserves the path")
			}
		})
	}
}

func containsPathIndex(indices []int, wanted int) bool {
	for _, index := range indices {
		if index == wanted {
			return true
		}
	}
	return false
}

func TestResolvedValuePathExtensionDoesNotMutateEarlierPaths(t *testing.T) {
	paths := assignmentValuePaths(t, ast.Unparen)
	original := paths[4]
	extended := original.withField(original.fields[1])
	require.Equal(t, "s.Inner.Kind", original.key("s"))
	require.Equal(t, "s.Inner.Kind.Kind", extended.key("s"))
	require.False(t, original.same(extended))
	require.False(t, (resolvedValuePath{}).same(resolvedValuePath{}))
	require.False(t, original.withField(nil).valid())
}

func TestVariantAddressAndDereferenceChainsRemainExcluded(t *testing.T) {
	h := &testHarness{T: t}
	prelude := variantPrelude + "type Container struct{Source Source}\n"
	for _, expr := range []string{"(*s).Kind", "(&s.Source).Kind"} {
		parameter := "s *Source"
		if expr == "(&s.Source).Kind" {
			parameter = "s Container"
		}
		source := prelude + variantFunction("First", variantSwitch("A,B")) + "func Second(" + parameter + "){if " + expr + "==A {} else if " + expr + "==B {}}"
		require.Empty(t, h.investigate(h.fixture(source), variantConfig()).Cases)
	}
}
