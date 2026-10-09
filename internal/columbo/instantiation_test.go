package columbo

import (
	"go/ast"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInstantiationSyntaxOwnership(t *testing.T) {
	head, argument := &ast.Ident{Name: "Box"}, &ast.Ident{Name: "Item"}
	paren := &ast.ParenExpr{X: head}
	index := &ast.IndexExpr{X: head, Index: argument}
	multiple := &ast.IndexListExpr{X: head, Indices: []ast.Expr{argument}}
	wrapped := &ast.IndexExpr{X: paren, Index: argument}
	for _, tt := range []struct {
		name string
		path []ast.Node
		node ast.Expr
		want ast.Expr
	}{
		{"index head", []ast.Node{index, head}, head, index},
		{"index argument", []ast.Node{index, argument}, argument, nil},
		{"list head", []ast.Node{multiple, head}, head, multiple},
		{"list argument", []ast.Node{multiple, argument}, argument, nil},
		{"parenthesized head", []ast.Node{wrapped, paren, head}, head, wrapped},
		{"no parent", []ast.Node{head}, head, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			scan := &dependencyScan{path: tt.path}
			require.Equal(t, tt.want, scan.typeInstantiation(tt.node))
		})
	}
}

func TestInstantiationOriginIdentity(t *testing.T) {
	h := &testHarness{T: t}
	dir := h.fixture(`package fixture
 type Box[T any] struct{ Value T }
 type Other[T any] struct{ Value T }
 type Alias[T any] = *Box[T]
 type OtherAlias[T any] = *Box[T]
 var BoxValue Box[int]
 var OtherValue Other[int]
 var AliasValue Alias[int]
 var OtherAliasValue OtherAlias[int]
 var BasicValue int
 `)
	engine := h.loadDependencyFixture(dir, quiet())
	scope := engine.files[0].pkg.Types.Scope()
	for _, tt := range []struct {
		instance, head string
		want           bool
	}{
		{"BoxValue", "Box", true},
		{"BoxValue", "Other", false},
		{"OtherValue", "Box", false},
		{"AliasValue", "Alias", true},
		{"AliasValue", "OtherAlias", false},
		{"OtherAliasValue", "Alias", false},
		{"AliasValue", "Box", false},
		{"BoxValue", "Alias", false},
		{"Box", "Box", false},
		{"Alias", "Alias", false},
		{"BasicValue", "BasicValue", false},
	} {
		t.Run(tt.instance+"/"+tt.head, func(t *testing.T) {
			require.Equal(t, tt.want, instantiatedFrom(scope.Lookup(tt.instance).Type(), scope.Lookup(tt.head).Type()))
		})
	}
}
