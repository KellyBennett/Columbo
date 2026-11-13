package operators

import (
	"fmt"
	"sort"

	"github.com/KellyBennett/Columbo/calibration/harness"
)

var registry = map[string]harness.Operator{}

func register(op harness.Operator) {
	if _, dup := registry[op.ID()]; dup {
		panic("duplicate operator " + op.ID())
	}
	registry[op.ID()] = op
}

func init() {
	register(noop{})
}

func All() []harness.Operator {
	ids := make([]string, 0, len(registry))
	for id := range registry {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]harness.Operator, len(ids))
	for i, id := range ids {
		out[i] = registry[id]
	}
	return out
}

func Select(ids []string) ([]harness.Operator, error) {
	var out []harness.Operator
	for _, id := range ids {
		op, ok := registry[id]
		if !ok {
			return nil, fmt.Errorf("unknown operator %q", id)
		}
		out = append(out, op)
	}
	return out, nil
}
