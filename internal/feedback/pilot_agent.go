package feedback

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type jsonPhase[T any] struct {
	name     string
	schema   string
	validate func(T) error
}

func runJSONPhase[T any](ctx context.Context, options PilotOptions, dir, prompt, prefix string, phase jsonPhase[T]) (T, []CommandResult, error) {
	var commands []CommandResult
	original := prompt
	var value T
	for attempt := 0; attempt < 2; attempt++ {
		if err := ctx.Err(); err != nil {
			return value, commands, err
		}
		log := prefix
		if attempt > 0 {
			log += "-retry"
		}
		command := invokeAgent(ctx, options, dir, phase.name, prompt, log, phase.schema)
		commands = append(commands, command)
		if !commandPassed(command) {
			return value, commands, fmt.Errorf("%s agent failed; see %s", phase.name, command.Stderr)
		}
		var decoded T
		err := readAgentJSON(command.Stdout, &decoded)
		if err == nil {
			err = phase.validate(decoded)
		}
		if err == nil {
			return decoded, commands, nil
		}
		if attempt == 1 {
			return value, commands, fmt.Errorf("%s response invalid after one retry: %w", phase.name, err)
		}
		prompt = original + fmt.Sprintf("\nYour previous response could not be accepted: %s\nReturn a corrected JSON object matching this schema:\n%s\n", err, phase.schema)
	}
	panic("unreachable")
}

func assessmentPhase() jsonPhase[DesignAssessment] {
	schema := objectSchema(map[string]any{
		"relationship": nonemptyString(), "opportunity": map[string]any{"type": "string"}, "future_change": nonemptyString(),
		"change_basis": map[string]any{"type": "string", "const": "hypothetical"}, "costs": map[string]any{"type": "string"}, "evidence": evidenceSchema(),
	}, []string{"relationship", "future_change", "change_basis", "evidence"})
	return jsonPhase[DesignAssessment]{"assess", schemaText(schema), func(a DesignAssessment) error {
		if strings.TrimSpace(a.Relationship) == "" || strings.TrimSpace(a.FutureChange) == "" || a.ChangeBasis != "hypothetical" || !validEvidence(a.Evidence) {
			return fmt.Errorf("assessment requires nonempty relationship, future_change, evidence and change_basis hypothetical")
		}
		return nil
	}}
}

func reviewPhase(explores bool) jsonPhase[DesignReview] {
	outcomes := []string{"beneficial", "tradeoff", "no-demonstrated-benefit", "inconclusive"}
	judgment := func(allowed []string) map[string]any {
		return objectSchema(map[string]any{"outcome": map[string]any{"type": "string", "enum": allowed}, "reason": nonemptyString(), "evidence": evidenceSchema()}, []string{"outcome", "reason", "evidence"})
	}
	candidateOutcomes := append([]string{}, outcomes...)
	if !explores {
		candidateOutcomes = []string{"not-applicable"}
	}
	schema := objectSchema(map[string]any{
		"finding": judgment(outcomes), "direction": judgment(outcomes), "candidate": judgment(candidateOutcomes), "actual": judgment(outcomes),
		"limitations": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}, "reported_model": map[string]any{"type": "string"},
	}, []string{"finding", "direction", "candidate", "actual"})
	return jsonPhase[DesignReview]{"review", schemaText(schema), func(r DesignReview) error {
		for _, item := range []struct {
			name    string
			value   Judgment
			allowed []string
		}{{"finding", r.Finding, outcomes}, {"direction", r.Direction, outcomes}, {"candidate", r.Candidate, candidateOutcomes}, {"actual", r.Actual, outcomes}} {
			if !contains(item.allowed, item.value.Outcome) || strings.TrimSpace(item.value.Reason) == "" || !validEvidence(item.value.Evidence) {
				return fmt.Errorf("review has an invalid %s judgment; require an allowed outcome, nonempty reason and evidence", item.name)
			}
		}
		return nil
	}}
}

func nonemptyString() map[string]any {
	return map[string]any{"type": "string", "minLength": 1, "pattern": `\S`}
}
func evidenceSchema() map[string]any {
	return map[string]any{"type": "array", "minItems": 1, "items": nonemptyString()}
}
func objectSchema(properties map[string]any, required []string) map[string]any {
	return map[string]any{"type": "object", "properties": properties, "required": required, "additionalProperties": false}
}
func schemaText(value any) string { data, _ := json.Marshal(value); return string(data) }
func validEvidence(values []string) bool {
	if len(values) == 0 {
		return false
	}
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return false
		}
	}
	return true
}

func readAgentJSON(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	text := strings.TrimSpace(string(data))
	if !json.Valid([]byte(text)) {
		remaining, fenced, found := text, "", false
		for {
			start := strings.Index(remaining, "```json")
			if start < 0 {
				break
			}
			block := remaining[start+len("```json"):]
			end := strings.Index(block, "```")
			if end < 0 {
				break
			}
			fenced, found = strings.TrimSpace(block[:end]), true
			remaining = block[end+3:]
		}
		if found {
			text = fenced
		} else if start, end := strings.IndexByte(text, '{'), strings.LastIndexByte(text, '}'); start >= 0 && end > start {
			text = text[start : end+1]
		}
	}
	var envelope struct {
		Type       string          `json:"type"`
		IsError    bool            `json:"is_error"`
		Subtype    string          `json:"subtype"`
		Structured json.RawMessage `json:"structured_output"`
	}
	if err := json.Unmarshal([]byte(text), &envelope); err != nil {
		return err
	}
	if envelope.Type == "result" {
		if envelope.IsError || (envelope.Subtype != "" && envelope.Subtype != "success") {
			return fmt.Errorf("agent returned an error result (%s)", envelope.Subtype)
		}
		if len(envelope.Structured) == 0 || string(envelope.Structured) == "null" {
			return fmt.Errorf("agent result lacks structured_output")
		}
		return json.Unmarshal(envelope.Structured, target)
	}
	return json.Unmarshal([]byte(text), target)
}
