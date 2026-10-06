package columbo

var selectionPolicy = PolicyReview{
	ID:           "SUC-001",
	Note:         "This rule deliberately treats local concrete-player selection followed by role collaboration as architectural pressure. This is a provisional dogfooding policy; its review note does not change the case verdict.",
	ReviewPrompt: "Show the human the selected role, concrete origins, controlling decision, value-flow evidence, and downstream messages. Discuss whether this function should own both implementation selection and collaboration before changing code or requesting suppression.",
}
var selectionAdvice = advice{
	why:       "Selecting a concrete implementation and collaborating with the resulting role in the same function couples construction policy to behavioral use. Changes to implementation selection can therefore affect code whose primary job is collaboration.",
	diagnosis: "{symbol} selects among multiple concrete implementations of the same role and subsequently sends messages to the selected collaborator. This may indicate that implementation selection belongs at a separate construction or factory boundary.",
	leads: []string{
		"Consider moving implementation selection into a factory or composition boundary that returns the role.",
		"Consider accepting the role as a dependency instead of deciding which concrete player to use here.",
		"Keep collaboration expressed in terms of the role while centralizing knowledge of concrete implementations.",
	},
	avoid: []string{
		"Moving only the switch into a helper that still exposes concrete implementation details to the caller.",
		"Wrapping the concrete implementations in another facade solely to reduce the finding.",
		"Renaming constructors, variables, or implementations.",
		"Extracting branch bodies while leaving selection and collaboration in the same function.",
	},
}
