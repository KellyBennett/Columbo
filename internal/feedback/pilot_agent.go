package feedback

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type agentCall struct {
	phase, dir, prompt, prefix, schema string
	readDirs                           []string
}

func (c agentCall) retrying(reason error) agentCall {
	c.prompt += fmt.Sprintf("\nYour previous response could not be accepted: %s\nReturn a corrected JSON object matching this schema:\n%s\n", reason, c.schema)
	c.prefix += "-retry"
	return c
}

func invokeAgent(ctx context.Context, options PilotOptions, call agentCall) CommandResult {
	if err := writePrivate(call.prefix+".prompt", []byte(call.prompt)); err != nil {
		return CommandResult{Exit: -1, Error: err.Error()}
	}
	return runTrialCommand(ctx, call.dir, agentArgs(options, call), call.prompt, call.prefix, options.Timeout, options.PassEnv...)
}

func agentArgs(options PilotOptions, call agentCall) []string {
	args := append([]string{options.Agent}, options.AgentArgs...)
	if len(options.AgentArgs) == 0 {
		return append(args, claudeArgs(options, call)...)
	}
	placeholders := strings.NewReplacer("{model}", options.Model, "{turns}", strconv.Itoa(options.Turns), "{phase}", call.phase)
	for i := 1; i < len(args); i++ {
		args[i] = placeholders.Replace(args[i])
	}
	return args
}

func claudeArgs(options PilotOptions, call agentCall) []string {
	tools, allowed := "Read,Glob,Grep,Bash", "Read,Glob,Grep,Bash(git show:*),Bash(git log:*),Bash(git diff:*)"+call.gitPermissions()
	if call.phase == "refactor" {
		tools = "Read,Edit,Write,Glob,Grep,Bash"
		allowed = "Read,Edit,Write,Glob,Grep,Bash(go build:*),Bash(go vet:*),Bash(go test:*),Bash(gofmt:*),Bash(go fmt:*),Bash(columbo:*)"
	}
	format := "text"
	if call.schema != "" {
		format = "json"
	}
	args := []string{"-p", "--safe-mode", "--restricted", "--no-session-persistence", "--permission-mode", "dontAsk", "--max-turns", strconv.Itoa(options.Turns), "--output-format", format, "--tools", tools, "--allowedTools", allowed}
	if call.schema != "" {
		args = append(args, "--json-schema", call.schema)
	}
	if options.Model != "" {
		args = append(args, "--model", options.Model)
	}
	if call.phase != "assess" {
		args = append(args, "--add-dir", filepath.Dir(call.dir))
	}
	return args
}

func (c agentCall) gitPermissions() string {
	var allowed strings.Builder
	for _, checkout := range c.readDirs {
		for _, verb := range []string{"show", "log", "diff"} {
			allowed.WriteString(",Bash(git -C " + quoteShell(checkout) + " " + verb + ":*)")
		}
	}
	return allowed.String()
}

func quoteShell(value string) string { return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'" }

type jsonPhase[T any] struct {
	name     string
	schema   string
	validate func(T) error
}

func (p jsonPhase[T]) run(ctx context.Context, options PilotOptions, call agentCall) (value T, commands []CommandResult, err error) {
	call.phase, call.schema = p.name, p.schema
	for range 2 {
		if err := ctx.Err(); err != nil {
			return value, commands, err
		}
		command := invokeAgent(ctx, options, call)
		commands = append(commands, command)
		if !commandPassed(command) {
			return value, commands, fmt.Errorf("%s agent failed; see %s", p.name, command.Stderr)
		}
		if value, err = p.decode(command.Stdout); err == nil {
			return value, commands, nil
		}
		call = call.retrying(err)
	}
	return value, commands, fmt.Errorf("%s response invalid after one retry: %w", p.name, err)
}

func (p jsonPhase[T]) decode(path string) (T, error) {
	var value, zero T
	if err := readAgentJSON(path, &value); err != nil {
		return zero, err
	}
	if err := p.validate(value); err != nil {
		return zero, err
	}
	return value, nil
}

func assessmentPhase() jsonPhase[DesignAssessment] {
	schema := objectSchema(map[string]any{
		"relationship": nonemptyString(), "opportunity": map[string]any{"type": "string"}, "future_change": nonemptyString(),
		"change_basis": map[string]any{"type": "string", "const": "hypothetical"}, "costs": map[string]any{"type": "string"}, "evidence": evidenceSchema(),
	}, []string{"relationship", "future_change", "change_basis", "evidence"})
	return jsonPhase[DesignAssessment]{"assess", schemaText(schema), DesignAssessment.validate}
}

func (a DesignAssessment) validate() error {
	if strings.TrimSpace(a.Relationship) == "" || strings.TrimSpace(a.FutureChange) == "" || a.ChangeBasis != "hypothetical" || !validEvidence(a.Evidence) {
		return fmt.Errorf("assessment requires nonempty relationship, future_change, evidence and change_basis hypothetical")
	}
	return nil
}

var reviewOutcomes = []string{"beneficial", "tradeoff", "no-demonstrated-benefit", "inconclusive"}

func reviewPhase(explores bool) jsonPhase[DesignReview] {
	candidateOutcomes := reviewOutcomes
	if !explores {
		candidateOutcomes = []string{"not-applicable"}
	}
	schema := objectSchema(map[string]any{
		"finding": judgmentSchema(reviewOutcomes), "direction": judgmentSchema(reviewOutcomes), "candidate": judgmentSchema(candidateOutcomes), "actual": judgmentSchema(reviewOutcomes),
		"limitations": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}, "reported_model": map[string]any{"type": "string"},
	}, []string{"finding", "direction", "candidate", "actual"})
	return jsonPhase[DesignReview]{"review", schemaText(schema), func(r DesignReview) error { return r.validate(candidateOutcomes) }}
}

func (r DesignReview) validate(candidateOutcomes []string) error {
	for _, item := range []struct {
		name     string
		judgment Judgment
		allowed  []string
	}{{"finding", r.Finding, reviewOutcomes}, {"direction", r.Direction, reviewOutcomes}, {"candidate", r.Candidate, candidateOutcomes}, {"actual", r.Actual, reviewOutcomes}} {
		if !item.judgment.valid(item.allowed) {
			return fmt.Errorf("review has an invalid %s judgment; require an allowed outcome, nonempty reason and evidence", item.name)
		}
	}
	return nil
}

func (j Judgment) valid(allowed []string) bool {
	return contains(allowed, j.Outcome) && strings.TrimSpace(j.Reason) != "" && validEvidence(j.Evidence)
}

func judgmentSchema(allowed []string) map[string]any {
	return objectSchema(map[string]any{"outcome": map[string]any{"type": "string", "enum": allowed}, "reason": nonemptyString(), "evidence": evidenceSchema()}, []string{"outcome", "reason", "evidence"})
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
	payload, err := structuredPayload(extractJSON(strings.TrimSpace(string(data))))
	if err != nil {
		return err
	}
	return json.Unmarshal(payload, target)
}

func extractJSON(text string) string {
	if json.Valid([]byte(text)) {
		return text
	}
	if fenced, ok := lastJSONFence(text); ok {
		return fenced
	}
	if start, end := strings.IndexByte(text, '{'), strings.LastIndexByte(text, '}'); start >= 0 && end > start {
		return text[start : end+1]
	}
	return text
}

func lastJSONFence(text string) (fenced string, found bool) {
	for {
		_, block, ok := strings.Cut(text, "```json")
		if !ok {
			return fenced, found
		}
		body, rest, ok := strings.Cut(block, "```")
		if !ok {
			return fenced, found
		}
		fenced, found, text = strings.TrimSpace(body), true, rest
	}
}

func structuredPayload(text string) ([]byte, error) {
	var envelope struct {
		Type       string          `json:"type"`
		IsError    bool            `json:"is_error"`
		Subtype    string          `json:"subtype"`
		Structured json.RawMessage `json:"structured_output"`
	}
	if err := json.Unmarshal([]byte(text), &envelope); err != nil {
		return nil, err
	}
	switch {
	case envelope.Type != "result":
		return []byte(text), nil
	case envelope.IsError || (envelope.Subtype != "" && envelope.Subtype != "success"):
		return nil, fmt.Errorf("agent returned an error result (%s)", envelope.Subtype)
	case len(envelope.Structured) == 0 || string(envelope.Structured) == "null":
		return nil, fmt.Errorf("agent result lacks structured_output")
	}
	return envelope.Structured, nil
}
