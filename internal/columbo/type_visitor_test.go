package columbo

import (
	"go/types"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTypeVisitorsPreserveDifferentTraversalPolicies(t *testing.T) {
	h := &testHarness{T: t}
	dir := h.fixture(`package fixture
type Argument struct{}
type Hidden interface{ Hidden() }
type Box[T any] struct {
 Next *Box[T]
 Role interface{ Visit() interface{ Done() } }
}
type Alias = Box[Argument]
func Probe(box *Alias, role interface{ Use(Hidden) }) {}
`)
	engine := h.loadDependencyFixture(dir, quiet())
	d := engine.declarations[0]
	collector := d.dependencyScan(engine).collector
	collector.identities = map[string]bool{}
	collector.walk(d.signature)
	collector.walk(nil)
	require.Equal(t, []string{
		"interface:interface{Use(fixture.Hidden)}",
		"type:fixture.Argument",
		"type:fixture.Box[fixture.Argument]",
	}, sortedSet(collector.identities), "dependencies retain named boundaries and follow type arguments")

	discovery := &interfaceDiscovery{seen: map[types.Type]bool{}}
	discovery.walk(d.signature)
	identities := map[string]bool{}
	for _, iface := range discovery.interfaces {
		identities[canonicalType(iface, nil)] = true
	}
	require.Equal(t, []string{
		"fixture.Hidden",
		"interface{Done()}",
		"interface{Hidden()}",
		"interface{Use(fixture.Hidden)}",
		"interface{Visit() interface{Done()}}",
	}, sortedSet(identities), "discovery follows underlying fields and method signatures through recursive types")
	before := len(discovery.interfaces)
	discovery.walk(d.signature)
	discovery.walk(nil)
	require.Len(t, discovery.interfaces, before, "revisiting a graph must not duplicate interfaces")
}
