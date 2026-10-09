package columbo

import (
	"fmt"
	"slices"
	"strings"
)

func (report Report) stageDefinitions() []stageDefinition {
	definitions := refactoringStages()
	for i := range definitions {
		definitions[i].addContext(report.Cases)
	}
	return definitions
}
func (definition *stageDefinition) addContext(cases []Case) {
	for _, c := range cases {
		if slices.Contains(definition.contextSmells, c.Smell) {
			definition.task += c.stageContext()
		}
	}
}
func (c Case) stageContext() string {
	return fmt.Sprintf("\n  Review lead (not a gate): %s %s:%d %s\n    %s\n    %s", c.ID, c.File, c.StartLine, c.Symbol, c.Diagnosis, strings.Join(c.Leads, "\n    "))
}
