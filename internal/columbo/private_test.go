package columbo

import (
	"go/ast"
	"go/parser"
	"go/types"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/packages"
)

func TestPrivateTypeDefinitionOwnership(t *testing.T) {
	for _, irrelevant := range []string{"", "a_empty.go", "z_empty.go"} {
		for _, reverse := range []bool{false, true} {
			t.Run(irrelevant+map[bool]string{false: "/forward", true: "/reverse"}[reverse], func(t *testing.T) {
				a := privateOwnershipFixture(t, irrelevant)
				if reverse {
					slices.Reverse(a.files)
				}
				index := &typeReferenceIndex{engine: a, usages: map[string]*privateTypeUsage{}}
				for _, f := range a.files {
					index.definitions(f)
				}
				for _, f := range a.files {
					index.references(f)
				}
				usages := index.sortedUsages()
				require.Len(t, usages, 1)
				usage := usages[0]
				require.Equal(t, "b_type.go", usage.file)
				require.Equal(t, filepath.Join(a.root, "b_type.go"), usage.path)
				require.Equal(t, 3, usage.line)
				require.Equal(t, []string{"b_type.go", "c_use.go", "d_use.go"}, sortedSet(usage.files))
				require.Equal(t, 2, usage.fieldAccesses, "same-file field access must be excluded")
				index.warn()
				require.Empty(t, a.report.Warnings, "three files must remain below the existing warning threshold")
				a.config.Counts["private-type-files"] = 2
				index.warn()
				require.Equal(t, []Warning{{Code: "private-type-dispersion", File: "b_type.go", Line: 3,
					Message: "Private type fixture.private spans 3 production files (limit: 2): b_type.go, c_use.go, d_use.go; 2 cross-file direct field accesses."}}, a.report.Warnings)
			})
		}
	}
}

func TestPrivateTypeDefinitionProductionScope(t *testing.T) {
	for _, scenario := range []string{"excluded", "test"} {
		t.Run(scenario, func(t *testing.T) {
			a := privateOwnershipFixture(t, "a_empty.go")
			for _, f := range a.files {
				if f.rel == "b_type.go" {
					if scenario == "excluded" {
						f.included = false
					} else {
						f.rel = "b_type_test.go"
					}
				}
			}
			index := &typeReferenceIndex{engine: a, usages: map[string]*privateTypeUsage{}}
			for _, f := range a.files {
				index.definitions(f)
			}
			require.Empty(t, index.usages, "a non-production definition must not be attributed to another production file")
		})
	}
}

func privateOwnershipFixture(t *testing.T, irrelevant string) *engine {
	t.Helper()
	a := newEngine(t.TempDir(), "", quiet())
	sources := map[string]string{
		"b_type.go": "package fixture\n\ntype private struct{ N int }\nfunc own(p private) { _ = p.N }\n",
		"c_use.go":  "package fixture\nfunc one(p private) { _ = p.N }\n",
		"d_use.go":  "package fixture\nfunc two(p private) { _ = p.N }\n",
	}
	if irrelevant != "" {
		sources[irrelevant] = "package fixture\n"
	}
	info := &types.Info{Defs: map[*ast.Ident]types.Object{}, Uses: map[*ast.Ident]types.Object{}, Selections: map[*ast.SelectorExpr]*types.Selection{}}
	pkg := &packages.Package{PkgPath: "fixture", TypesInfo: info}
	names := make([]string, 0, len(sources))
	for name := range sources {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		path := filepath.Join(a.root, name)
		node, err := parser.ParseFile(a.fset, path, sources[name], 0)
		require.NoError(t, err)
		f := a.sourceFile(pkg, node, path, name)
		f.included = true
		a.files = append(a.files, f)
		pkg.Syntax = append(pkg.Syntax, node)
	}
	var config types.Config
	var err error
	pkg.Types, err = config.Check(pkg.PkgPath, a.fset, pkg.Syntax, info)
	require.NoError(t, err)
	return a
}
