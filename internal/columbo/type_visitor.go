package columbo

import "go/types"

type typeVisitor interface {
	named(*types.Named)
	iface(*types.Interface)
	components(types.Type)
}

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
