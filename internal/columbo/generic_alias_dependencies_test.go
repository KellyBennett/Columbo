package columbo

import "testing"

func TestGenericAliasDependencyInventory(t *testing.T) {
	for _, tt := range []struct{ name, alias string }{
		{"pointer", "type Alias[T any] = *Box[T]"},
		{"slice", "type Alias[T any] = []Box[T]"},
		{"map", "type Alias[T comparable] = map[T]Box[T]"},
		{"nongeneric instantiated alias", "type Alias = Box[int]"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h := &testHarness{T: t}
			parameter := "Alias[int]"
			if tt.name == "nongeneric instantiated alias" {
				parameter = "Alias"
			}
			dir := h.fixture("package fixture\ntype Box[T any] struct{Value T}\n" + tt.alias + "\nfunc F(x " + parameter + ") {}")
			engine := h.loadDependencyFixture(dir, quiet())
			d := engine.declarations[0]
			h.dependencyMeasurement(engine, d, []string{"type:fixture.Box[int]"})
		})
	}
}
