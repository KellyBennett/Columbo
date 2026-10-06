package columbo

var variantPolicy = PolicyReview{
	ID:           "RVD-001",
	Note:         "This rule deliberately treats repeated interpretation of the same typed variant domain as architectural pressure even when the decisions may be intentional. This is a provisional dogfooding policy; its review note does not change the case verdict.",
	ReviewPrompt: "Show the human the variant domain, every supporting decision site, each site's variant set, and the repeated variants with their support counts. Evaluate a factory that selects role implementations once, or a visitor/adapter that centralizes variant dispatch while each operation owns its behavior and traversal. Also consider a shared data mapping. Compare cohesion and coupling before changing code or requesting suppression; the finding does not prove that a pattern is appropriate.",
}

var variantAdvice = advice{
	why:       "Repeated decisions over the same variant domain distribute knowledge of that taxonomy. Adding or changing a variant may require coordinated edits across otherwise unrelated code.",
	diagnosis: "Multiple decision sites repeatedly distinguish variants of the same typed domain. This may indicate distributed variant knowledge, a missing behavioral role, or a decision that should be centralized.",
	leads: []string{
		"Examine whether callers can send the same message to interchangeable role players instead of selecting behavior themselves.",
		"If the variants select implementations, use a factory or construction boundary to select once and return a common behavioral role; callers then send messages through that role.",
		"If independent operations inspect the same variants, evaluate a visitor or adapter with one dispatch boundary; keep operation-specific behavior, traversal policy, and results with each operation.",
		"If branches merely map variants to data, consider a data-driven representation owned near the variant definition.",
		"Move behavior toward the object or role that has the knowledge required to perform it.",
	},
	avoid: []string{
		"Adding wrapper objects, broad visitor protocols, or no-op methods solely to clear the finding without improving cohesion or reducing coupling.",
		"Moving each decision into a differently named helper while retaining the repeated variant knowledge.",
		"Replacing switches with equivalent if/else chains.",
		"Introducing an interface while callers still select concrete implementations repeatedly.",
		"Renaming constants, aliases, variables, or files solely to evade correlation.",
	},
}
