package columbo

import "go/types"

// typeVisitor chooses which edges of a semantic type graph to follow.
// Named types and interfaces need analysis-specific policy; other types expose
// their structure through typeComponents.
type typeVisitor interface {
	named(*types.Named)
	iface(*types.Interface)
	components(types.Type)
}

// visitType owns classification, while the visitor owns recursion and results.
func visitType(t types.Type, visitor typeVisitor) {
	if t == nil {
		return
	}
	switch t := types.Unalias(t).(type) {
	case *types.Named:
		visitor.named(t)
	case *types.Interface:
		visitor.iface(t)
	default:
		visitor.components(t)
	}
}
