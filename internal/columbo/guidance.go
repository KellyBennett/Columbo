package columbo

import "strings"

func guidance(c *Case) {
	switch c.Smell {
	case "long-function":
		c.Why = "Long functions can obscure responsibility boundaries and ownership."
		c.Diagnosis = strings.ReplaceAll("{symbol} exceeds the configured source-line limit; this may indicate responsibilities that deserve separate boundaries.", "{symbol}", c.Symbol)
		c.Leads = []string{"Look for cohesive responsibilities with different reasons to change.", "Move behavior toward the object that owns its data."}
		c.Avoid = []string{"Extracting arbitrary helpers solely to reduce the line count.", "Moving the same procedure into another file."}
	case "long-parameter-list":
		c.Why = "Many parameters can obscure which inputs belong together."
		c.Diagnosis = strings.ReplaceAll("{symbol} exceeds the configured parameter limit; its inputs may represent a missing domain concept or mixed responsibilities.", "{symbol}", c.Symbol)
		c.Leads = []string{"Examine whether inputs form a cohesive domain object.", "Separate responsibilities that require different inputs."}
		c.Avoid = []string{"Bundling unrelated inputs into an opaque bag solely to reduce the count."}
	case "high-cognitive-complexity":
		c.Why = "Deep or repeated decision structures make behavior harder to follow."
		c.Diagnosis = strings.ReplaceAll("{symbol} exceeds the configured cognitive-complexity limit; its decision structure may need simplification.", "{symbol}", c.Symbol)
		c.Leads = []string{"Clarify early exits and simplify nested decisions.", "Examine whether variants deserve meaningful abstractions."}
		c.Avoid = []string{"Moving the same decision tree into trivial helpers solely to hide its score."}
	case "excessive-dependencies":
		c.Why = "Broad dependency sets can indicate excessive coordination or coupling."
		c.Diagnosis = strings.ReplaceAll("{symbol} exceeds the configured dependency limit; it may coordinate responsibilities with separate ownership.", "{symbol}", c.Symbol)
		c.Leads = []string{"Inspect the contributing dependencies for cohesive ownership boundaries.", "Move behavior toward the data and collaborators it primarily uses."}
		c.Avoid = []string{"Introducing a facade whose only purpose is hiding the dependency count."}
	case "feature-envy":
		c.Why = "Concentrated access to another value can indicate misplaced behavior."
		c.Diagnosis = strings.ReplaceAll("{symbol} meets the configured foreign-access and ratio conditions; behavior may belong nearer a qualifying foreign value. Zero own accesses satisfies the ratio condition once the foreign minimum is met.", "{symbol}", c.Symbol)
		c.Leads = []string{"Review each qualifying foreign value and its contributing accesses.", "Consider moving cohesive behavior toward the owner of that data."}
		c.Avoid = []string{"Renaming or aliasing a collaborator solely to split the access count."}
	case "data-clump":
		c.Why = "Repeated groups of parameter types can indicate a missing shared concept."
		c.Diagnosis = strings.ReplaceAll("The parameter multiset anchored at {symbol} recurs at the configured support and size; it may represent a missing domain object.", "{symbol}", c.Symbol)
		c.Leads = []string{"Inspect the participating signatures for a shared domain concept.", "Introduce a parameter object only if the values belong together."}
		c.Avoid = []string{"Bundling unrelated values solely to silence the repeated-type rule."}
	case "cosmetic-extraction":
		c.Why = "Single-use helper sequences can retain the original procedure's coupling while hiding its metrics."
		c.Diagnosis = strings.ReplaceAll("{symbol} has helper clusters meeting the configured overlap conditions, and candidate expansion exceeds a configured metric limit; this may be cosmetic decomposition even if the helper names sound meaningful.", "{symbol}", c.Symbol)
		c.Leads = []string{"Review the qualifying helpers with their shared inputs and dependencies.", "Establish cohesive boundaries with distinct ownership and collaboration."}
		c.Avoid = []string{"Creating stepOne/stepTwo/stepThree helpers solely to satisfy metrics.", "Treating a meaningful name as proof of a meaningful boundary.", "Suppressing the case without documented justification."}
	}
}
