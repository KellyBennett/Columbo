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
	register(inlineFunction{id: "inline-function", target: "long-function"})
	register(inlineFunction{id: "inline-function-complexity", target: "high-cognitive-complexity"})
	register(extractFunction{id: "extract-function", target: "long-function"})
	register(extractFunction{id: "extract-function-complexity", target: "high-cognitive-complexity"})
	register(splitForwarding{id: "split-forwarding-helpers", follows: "inline-function", target: "long-function", catches: []string{"long-function", "cosmetic-extraction"}, class: harness.Evade})
	register(splitForwarding{id: "split-forwarding-helpers-complexity", follows: "inline-function-complexity", target: "high-cognitive-complexity", catches: []string{"high-cognitive-complexity", "cosmetic-extraction"}, class: harness.Evade})
	register(splitForwarding{id: "split-long-function", target: "cosmetic-extraction", catches: []string{"cosmetic-extraction"}, class: harness.Inject})
	register(isolateHelperCalls{id: "isolate-helper-calls", follows: "split-long-function"})
	register(moveMethod{id: "move-method-to-function"})
	register(moveMethod{id: "move-method-to-collaborator", collaborator: true})
	register(hoistFields{id: "copy-fields-to-locals", follows: "move-method-to-function"})
	register(hoistFields{id: "hide-behind-getters", follows: "move-method-to-function", getters: true})
	register(moveFunction{})
	register(repeatVariantDecision{})
	register(decisionToMapLookup{})
	register(replaceConditionalWithPolymorphism{})
	register(selectThenUse{})
	register(launderSelection{})
	register(extractFactory{})
	register(splitForwarding{id: "split-forwarding-helpers", follows: "inline-function", target: "long-function", catches: []string{"long-function", "cosmetic-extraction"}})
	register(splitForwarding{id: "split-forwarding-helpers-complexity", follows: "inline-function-complexity", target: "high-cognitive-complexity", catches: []string{"high-cognitive-complexity", "cosmetic-extraction"}})
	register(addCollaborators{})
	register(routeThroughFacade{})
	register(splitByCollaborators{})
	register(addProseComment{})
	register(proseToStringConstant{})
	register(deleteProseComments{})
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
