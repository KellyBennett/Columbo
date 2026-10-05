package columbo

import "strings"

type advice struct {
	why, diagnosis string
	leads, avoid   []string
}

var adviceCatalog = map[string]advice{
	"duplicate-code":            {"Repeated syntax can hide shared policy and increase the cost of coordinated changes.", "The fragment anchored at {symbol} has matching Go AST structure elsewhere; names and literal values are ignored. This is a review lead, not proof of equivalent behavior.", []string{"Compare every duplicate-fragment receipt and identify knowledge that must change together.", "Share an operation only when the copies own the same policy."}, []string{"Extracting ordinary Go boilerplate or unrelated operations solely to remove a structural match.", "Treating structural similarity as semantic equivalence."}},
	"long-function":             {"Long functions can obscure responsibility boundaries and ownership.", "{symbol} exceeds the configured source-line limit; this may indicate responsibilities that deserve separate boundaries.", []string{"Look for cohesive responsibilities with different reasons to change.", "Move behavior toward the object that owns its data."}, []string{"Extracting arbitrary helpers solely to reduce the line count.", "Moving the same procedure into another file."}},
	"long-parameter-list":       {"Many parameters can obscure which inputs belong together.", "{symbol} exceeds the configured parameter limit; its inputs may represent a missing domain concept or mixed responsibilities.", []string{"Examine whether inputs form a cohesive domain object.", "Separate responsibilities that require different inputs."}, []string{"Bundling unrelated inputs into an opaque bag solely to reduce the count."}},
	"high-cognitive-complexity": {"Deep or repeated decision structures make behavior harder to follow.", "{symbol} exceeds the configured cognitive-complexity limit; its decision structure may need simplification.", []string{"Clarify early exits and simplify nested decisions.", "Examine whether variants deserve meaningful abstractions."}, []string{"Moving the same decision tree into trivial helpers solely to hide its score."}},
	"excessive-dependencies":    {"A broad dependency set is a lead to inspect where knowledge and decisions belong; the count alone does not establish mixed responsibilities.", "{symbol} exceeds the configured dependency limit. Inspect its evidence for repeated policy, exposed implementation details, implicit contracts, and cohesive collaboration.", []string{"Does the caller know details its dependency could own?", "Is policy repeated across boundaries?", "Are behavioral assumptions explicit in the contract?", "Do these dependencies support one cohesive operation?", "Compare supplied arguments and consumed results with types appearing only in signatures or discarded results; origin lists overlap and do not alter the score."}, []string{"Introducing a facade or moving code solely to lower the dependency count.", "Treating a lower dependency score as proof of a better design."}},
	"feature-envy":              {"Concentrated access to another value can indicate misplaced behavior.", "{symbol} meets the configured foreign-access and ratio conditions; behavior may belong nearer a qualifying foreign value. Zero own accesses satisfies the ratio condition once the foreign minimum is met.", []string{"Review each qualifying foreign value and its contributing accesses.", "Consider moving cohesive behavior toward the owner of that data."}, []string{"Renaming or aliasing a collaborator solely to split the access count."}},
	"data-clump":                {"Repeated groups of parameter types can indicate a missing shared concept.", "The parameter multiset anchored at {symbol} recurs at the configured support and size; it may represent a missing domain object.", []string{"Inspect the participating signatures for a shared domain concept.", "Introduce a parameter object only if the values belong together."}, []string{"Bundling unrelated values solely to silence the repeated-type rule."}},
	"cosmetic-extraction":       {"Single-use helper sequences can retain the original procedure's coupling while hiding its metrics.", "{symbol} has helper clusters meeting the configured overlap conditions, and candidate expansion exceeds a configured metric limit; this may be cosmetic decomposition even if the helper names sound meaningful.", []string{"Review the qualifying helpers with their shared inputs and dependencies.", "Establish cohesive boundaries with distinct ownership and collaboration."}, []string{"Creating stepOne/stepTwo/stepThree helpers solely to satisfy metrics.", "Treating a meaningful name as proof of a meaningful boundary.", "Suppressing the case without documented justification."}},
}

func guidance(c *Case) {
	a := adviceCatalog[c.Smell]
	c.Why = a.why
	c.Diagnosis = strings.ReplaceAll(a.diagnosis, "{symbol}", c.Symbol)
	c.Leads = append([]string{}, a.leads...)
	c.Avoid = append([]string{}, a.avoid...)
}
