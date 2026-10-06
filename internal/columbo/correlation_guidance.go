package columbo

var missingRolePolicy = PolicyReview{"MPR-001", "This diagnosis is derived by correlating independently enforced evidence. The correlation itself does not add a CI failure.", "Review whether the linked cases genuinely point to one role and one selection boundary. Pay particular attention to the variant-to-implementation mapping and inferred message surface."}

const missingRoleStrong = "Repeated interpretation of the same variant domain selects concrete implementations that already share an observed behavioral role. These cases may represent a missing polymorphic role with distributed player-selection knowledge."
const missingRoleConflict = "Repeated interpretation of the same variant domain selects incompatible concrete implementations for the same variant. The linked evidence identifies the conflicting mappings; Columbo cannot establish a stable variant-to-player relationship."
const missingRoleIncomplete = "Repeated interpretation of the same variant domain selects concrete implementations with a common observed behavioral surface, but Columbo cannot establish a complete variant-to-player mapping from the available evidence."
const missingRoleVariantSelection = "Repeated interpretation of the same variant domain also selects concrete collaborators. Selection knowledge appears distributed, although Columbo cannot establish a common behavioral role from the available evidence."
const missingRoleSelectionRole = "Concrete implementations are selected and then used through a common observed behavioral surface. The collaboration may be expressible as a role even though repeated variant knowledge has not been established."

func (c *Correlation) diagnose(repeated, role bool, mapping string) {
	c.Diagnosis = missingRoleDiagnosis(repeated, role, mapping)
	if repeated && role && mapping == "stable" {
		c.Confidence = "strong"
	}
}
func missingRoleDiagnosis(repeated, role bool, mapping string) string {
	if !repeated {
		return missingRoleSelectionRole
	}
	if mapping == "conflicting" {
		return missingRoleConflict
	}
	if !role {
		return missingRoleVariantSelection
	}
	if mapping == "stable" {
		return missingRoleStrong
	}
	return missingRoleIncomplete
}
func missingRoleLeads(confidence string) []string {
	if confidence != "strong" {
		return []string{"Review the linked selection sites, observed messages, and variant mappings before choosing a role or centralizing selection."}
	}
	return []string{
		"Identify the inferred minimal role.",
		"Consider introducing or reusing an interface representing that role at the consumer boundary.",
		"Centralize variant-to-concrete-player knowledge in a factory or composition boundary.",
		"Pass or return the role rather than exposing concrete implementation selection to collaborators.",
		"Re-run Columbo and verify that underlying cases disappear because knowledge moved, not because evidence was obscured.",
	}
}

var missingRoleAvoid = []string{
	"Do not introduce an interface while leaving repeated concrete selection in every caller.",
	"Do not extract each switch into a differently named helper.",
	"Do not create one facade per concrete implementation while preserving the variant taxonomy in callers.",
	"Do not suppress all member cases merely because the correlation identifies a likely abstraction.",
	"Do not treat creation of a named interface as proof that coupling improved.",
}
