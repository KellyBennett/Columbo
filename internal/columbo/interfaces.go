package columbo

import (
	"go/types"
	"golang.org/x/tools/go/packages"
)

// interfaceDiscovery follows loaded semantic type graphs, independent of source
// exclusions, while the seen set prevents recursive named types from looping.
type interfaceDiscovery struct {
	seen       map[types.Type]bool
	interfaces []types.Type
}

func (a *engine) findInterfaces(pkgs []*packages.Package) {
	discovery := &interfaceDiscovery{seen: map[types.Type]bool{}}
	packages.Visit(pkgs, discovery.packageTypes, nil)
	a.interfaces = append(a.interfaces, discovery.interfaces...)
}
func (d *interfaceDiscovery) packageTypes(p *packages.Package) bool {
	if p.Types != nil {
		d.scope(p.Types.Scope())
	}
	if p.TypesInfo != nil {
		d.expressions(p.TypesInfo)
	}
	return true
}
func (d *interfaceDiscovery) expressions(info *types.Info) {
	for _, tv := range info.Types {
		d.walk(tv.Type)
	}
}
func (d *interfaceDiscovery) scope(scope *types.Scope) {
	for _, name := range scope.Names() {
		d.walk(scope.Lookup(name).Type())
	}
}
func (d *interfaceDiscovery) walk(t types.Type) {
	if t == nil {
		return
	}
	t = types.Unalias(t)
	if d.seen[t] {
		return
	}
	d.seen[t] = true
	d.components(t)
}
func (d *interfaceDiscovery) components(t types.Type) {
	switch t := t.(type) {
	case *types.Named:
		d.named(t)
	case *types.Interface:
		d.iface(t)
	default:
		for _, part := range typeComponents(t) {
			d.walk(part)
		}
	}
}
func (d *interfaceDiscovery) named(t *types.Named) {
	if _, ok := t.Underlying().(*types.Interface); ok {
		d.interfaces = append(d.interfaces, t)
	}
	d.walk(t.Underlying())
}
func (d *interfaceDiscovery) iface(t *types.Interface) {
	d.interfaces = append(d.interfaces, t)
	for i := 0; i < t.NumMethods(); i++ {
		d.walk(t.Method(i).Type())
	}
}
