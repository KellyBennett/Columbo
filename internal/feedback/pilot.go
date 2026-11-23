package feedback

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/KellyBennett/Columbo/internal/feedbackdb"
)

type stringList []string

func (s *stringList) String() string         { return strings.Join(*s, ",") }
func (s *stringList) Set(value string) error { *s = append(*s, value); return nil }

type PilotOptions struct {
	Repository         string        `json:"repository"`
	Store              string        `json:"store"`
	Out                string        `json:"out"`
	Agent              string        `json:"agent"`
	AgentArgs          stringList    `json:"agent_args"`
	AgentSHA256        string        `json:"agent_sha256"`
	Model              string        `json:"requested_model"`
	Columbo            string        `json:"columbo"`
	ColumboSHA256      string        `json:"columbo_sha256"`
	Checks             stringList    `json:"checks"`
	Limit              int           `json:"limit"`
	Turns              int           `json:"turns"`
	Attempts           int           `json:"attempts"`
	Timeout            time.Duration `json:"agent_timeout"`
	CheckTimeout       time.Duration `json:"check_timeout"`
	Seed               string        `json:"seed"`
	PR                 int64         `json:"pr,omitempty"`
	Cases              stringList    `json:"cases,omitempty"`
	AllowReconstructed bool          `json:"allow_reconstructed"`
	Module             string        `json:"module"`
	PassEnv            stringList    `json:"pass_env_names,omitempty"`
	RunnerSHA256       string        `json:"runner_sha256"`
	Config             string        `json:"config,omitempty"`
	ConfigSHA256       string        `json:"config_sha256,omitempty"`
}

type PilotCase struct {
	ID               string      `json:"id"`
	CaseID           string      `json:"case_id"`
	Rule             string      `json:"rule"`
	PR               int64       `json:"pr"`
	Reaction         string      `json:"reaction"`
	Finding          Comment     `json:"finding"`
	Before           string      `json:"before"`
	After            string      `json:"after,omitempty"`
	RevisionEvidence string      `json:"revision_evidence"`
	Initial          Observation `json:"initial"`
	Latest           Observation `json:"latest"`
}

type PilotPlan struct {
	Version  int          `json:"version"`
	Options  PilotOptions `json:"options"`
	Cases    []PilotCase  `json:"cases"`
	Excluded []string     `json:"excluded"`
}

func (c cli) pilot(args []string) int {
	flags := flag.NewFlagSet("pilot", flag.ContinueOnError)
	flags.SetOutput(c.stderr)
	o := PilotOptions{}
	flags.StringVar(&o.Repository, "repo", "", "repository as owner/name")
	flags.StringVar(&o.Store, "store", defaultStore(), "feedback store")
	flags.StringVar(&o.Out, "out", "", "pilot directory; use the same path to resume")
	flags.StringVar(&o.Agent, "agent", "claude", "agent executable; receives prompt on stdin")
	flags.Var(&o.AgentArgs, "agent-arg", "agent argument; repeat to replace the default Claude arguments")
	flags.StringVar(&o.Model, "model", "", "requested model (default agent's configured model)")
	flags.StringVar(&o.Columbo, "columbo", "columbo", "analyzer executable for before/after reconstructions")
	flags.Var(&o.Checks, "check", "validation shell command; repeat to replace go build/vet/test ./...")
	flags.IntVar(&o.Limit, "limit", 12, "maximum findings, balanced across reaction groups and rules")
	flags.IntVar(&o.Turns, "turns", 30, "maximum turns per default Claude invocation")
	flags.IntVar(&o.Attempts, "attempts", 2, "maximum refactoring attempts per finding")
	flags.DurationVar(&o.Timeout, "timeout", 10*time.Minute, "time limit per agent phase")
	flags.DurationVar(&o.CheckTimeout, "check-timeout", 5*time.Minute, "time limit per validation command")
	flags.StringVar(&o.Seed, "seed", "pilot-1", "repeatable sample selection seed")
	flags.Int64Var(&o.PR, "pr", 0, "restrict to one pull request")
	flags.Var(&o.Cases, "case", "restrict to a case ID; repeat for more")
	flags.BoolVar(&o.AllowReconstructed, "allow-reconstructed", false, "allow review-anchor revisions when the exact analyzed revision is unavailable")
	flags.StringVar(&o.Module, "module", ".", "Go module subdirectory relative to the repository")
	flags.Var(&o.PassEnv, "pass-env", "additional environment variable name to pass to agents and checks; repeat for more")
	flags.StringVar(&o.Config, "config", "", "configuration path within the module (default captured CI config, then .columbo.yml)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 || !repositoryPattern.MatchString(o.Repository) || o.Out == "" || o.Limit < 1 || o.Turns < 1 || o.Attempts < 1 || o.Timeout <= 0 || o.CheckTimeout <= 0 {
		return c.fail(2, errors.New("pilot requires --repo owner/name, --out path, and positive limits"))
	}
	if filepath.IsAbs(o.Module) || o.Module == ".." || strings.HasPrefix(filepath.Clean(o.Module), ".."+string(filepath.Separator)) {
		return c.fail(2, errors.New("--module must stay within the repository"))
	}
	if filepath.IsAbs(o.Config) || o.Config == ".." || strings.HasPrefix(filepath.Clean(o.Config), ".."+string(filepath.Separator)) {
		return c.fail(2, errors.New("--config must name a file within the module"))
	}
	for _, name := range o.PassEnv {
		if name == "" || strings.ContainsAny(name, "=\x00") {
			return c.fail(2, errors.New("--pass-env takes a variable name, not a value"))
		}
	}
	if len(o.Checks) == 0 {
		o.Checks = stringList{"go build ./...", "go vet ./...", "go test ./..."}
	}
	var err error
	if o.Out, err = filepath.Abs(o.Out); err != nil {
		return c.fail(2, err)
	}
	if o.Store, err = filepath.Abs(o.Store); err != nil {
		return c.fail(2, err)
	}
	if o.Agent, o.AgentSHA256, err = identifyExecutable(o.Agent); err != nil {
		return c.fail(2, err)
	}
	if o.Columbo, o.ColumboSHA256, err = identifyExecutable(o.Columbo); err != nil {
		return c.fail(2, err)
	}
	if self, err := os.Executable(); err == nil {
		_, o.RunnerSHA256, _ = identifyExecutable(self)
	}
	if err := Pilot(c.ctx, o, c.stdout); err != nil {
		return c.fail(1, err)
	}
	return 0
}

func identifyExecutable(name string) (string, string, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		return "", "", err
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return "", "", err
	}
	file, err := os.Open(path)
	if err != nil {
		return "", "", err
	}
	defer file.Close()
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return "", "", err
	}
	return path, hex.EncodeToString(digest.Sum(nil)), nil
}

func Pilot(ctx context.Context, options PilotOptions, out io.Writer) error {
	if err := os.MkdirAll(options.Out, 0o700); err != nil {
		return err
	}
	lock, err := os.OpenFile(filepath.Join(options.Out, ".lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return fmt.Errorf("another pilot is using this output directory")
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	plan, err := loadOrPlan(ctx, options)
	if err != nil {
		return err
	}
	if len(plan.Cases) == 0 {
		return fmt.Errorf("no eligible findings; see %s for exclusions", filepath.Join(options.Out, "plan.json"))
	}
	var failures []string
	for i, finding := range plan.Cases {
		fmt.Fprintf(out, "[%d/%d] PR %d %s (%s)\n", i+1, len(plan.Cases), finding.PR, finding.CaseID, finding.Reaction)
		if err := runPilotCase(ctx, options, finding); err != nil {
			failures = append(failures, finding.ID+": "+err.Error())
			fmt.Fprintf(out, "  incomplete: %v\n", err)
		}
		if err := pilotSummary(options.Out, plan); err != nil {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
	fmt.Fprintln(out, "Pilot report:", filepath.Join(options.Out, "report.md"))
	if len(failures) > 0 {
		return fmt.Errorf("%d trials incomplete; rerun the same command to retry (earlier attempts retained)", len(failures))
	}
	return nil
}

func loadOrPlan(ctx context.Context, options PilotOptions) (PilotPlan, error) {
	path := filepath.Join(options.Out, "plan.json")
	var plan PilotPlan
	data, err := os.ReadFile(path)
	if err == nil {
		if err := json.Unmarshal(data, &plan); err != nil {
			return plan, err
		}
		before, _ := json.Marshal(plan.Options)
		after, _ := json.Marshal(options)
		if !bytesEqual(before, after) {
			return plan, fmt.Errorf("pilot options or executable hashes changed; choose a new --out directory")
		}
		return plan, nil
	}
	if !os.IsNotExist(err) {
		return plan, err
	}
	if _, err := os.Stat(options.Store); err != nil {
		return plan, fmt.Errorf("collect feedback first: %w", err)
	}
	store, err := feedbackdb.Open(options.Store)
	if err != nil {
		return plan, err
	}
	defer store.Close()
	rows, err := store.Queries().Observations(ctx, feedbackdb.ObservationsParams{Repository: options.Repository})
	if err != nil {
		return plan, err
	}
	var observations []Observation
	for _, row := range rows {
		var observation Observation
		if err := json.Unmarshal([]byte(row.Payload), &observation); err != nil {
			return plan, err
		}
		observations = append(observations, observation)
	}
	plan = makePilotPlan(options, observations)
	return plan, saveJSON(path, plan)
}

func bytesEqual(a, b []byte) bool { return string(a) == string(b) }

func saveJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return writePrivate(path, append(data, '\n'))
}

func makePilotPlan(options PilotOptions, observations []Observation) PilotPlan {
	plan := PilotPlan{Version: 1, Options: options}
	latest := map[int64]Observation{}
	for _, observation := range observations {
		latest[observation.Record.PullRequest.Number] = observation
	}
	seen := map[string]bool{}
	buckets := map[string][]PilotCase{}
	for _, observation := range observations {
		pr := observation.Record.PullRequest.Number
		if options.PR != 0 && options.PR != pr {
			continue
		}
		for _, comment := range observation.Record.Comments {
			if comment.Marker == nil || comment.Marker.Role != "finding" {
				continue
			}
			if len(options.Cases) > 0 && !contains(options.Cases, comment.Marker.CaseID) {
				continue
			}
			key := strconv.FormatInt(pr, 10) + "/" + comment.Marker.CaseID
			if seen[key] {
				continue
			}
			seen[key] = true
			finding := PilotCase{CaseID: comment.Marker.CaseID, Rule: comment.Marker.Rule, PR: pr, Finding: comment, Initial: observation, Latest: latest[pr]}
			finding.ID = shortHash(key)
			finding.Before, finding.RevisionEvidence = findingRevision(comment, observation)
			finding.Reaction = reactionGroup(finding.Latest.Record, comment.Kind, comment.ID)
			for _, revision := range finding.Latest.Revisions {
				if contains(revision.Roles, "pr-head") && revision.Present {
					finding.After = revision.SHA
				}
			}
			reason := ""
			switch {
			case finding.Before == "":
				reason = "no review-anchor revision"
			case finding.RevisionEvidence != "recorded analyzed revision" && !options.AllowReconstructed:
				reason = "requires --allow-reconstructed"
			case observation.GitDir == "":
				reason = "history not retained; collect with --checkout"
			case !revisionPresent(observation, finding.Before):
				reason = "starting revision unavailable"
			}
			if reason != "" {
				plan.Excluded = append(plan.Excluded, key+": "+reason)
				continue
			}
			bucket := finding.Reaction + "/" + finding.Rule
			buckets[bucket] = append(buckets[bucket], finding)
		}
	}
	var keys []string
	for key := range buckets {
		keys = append(keys, key)
		sort.Slice(buckets[key], func(i, j int) bool {
			return shortHash(options.Seed+buckets[key][i].ID) < shortHash(options.Seed+buckets[key][j].ID)
		})
	}
	sort.Strings(keys)
	groups := map[string][]string{}
	for _, key := range keys {
		group := strings.SplitN(key, "/", 2)[0]
		groups[group] = append(groups[group], key)
	}
	groupOrder := []string{"thumbs-down", "thumbs-up", "unreacted", "mixed", "other"}
	positions := map[string]int{}
	for len(plan.Cases) < options.Limit {
		added := false
		for _, group := range groupOrder {
			groupKeys := groups[group]
			for step := 0; step < len(groupKeys); step++ {
				index := positions[group] % len(groupKeys)
				positions[group]++
				key := groupKeys[index]
				if len(buckets[key]) == 0 {
					continue
				}
				plan.Cases = append(plan.Cases, buckets[key][0])
				buckets[key] = buckets[key][1:]
				added = true
				break
			}
			if len(plan.Cases) == options.Limit {
				break
			}
		}
		if !added {
			break
		}
	}
	return plan
}

func shortHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:8])
}
func contains(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}
func revisionPresent(observation Observation, sha string) bool {
	for _, r := range observation.Revisions {
		if r.SHA == sha && r.Present {
			return true
		}
	}
	return false
}

func findingRevision(comment Comment, observation Observation) (string, string) {
	anchor := ""
	if comment.CommitID != nil {
		anchor = *comment.CommitID
	}
	if comment.OriginalCommitID != nil {
		anchor = *comment.OriginalCommitID
	}
	var exact []string
	for _, artifact := range observation.Artifacts {
		var provenance struct {
			HeadCommit     string   `json:"head_commit"`
			AnalyzedCommit string   `json:"analyzed_commit"`
			Clean          bool     `json:"clean"`
			Cases          []string `json:"cases"`
		}
		if json.Unmarshal(artifact.Provenance, &provenance) == nil && provenance.Clean && provenance.HeadCommit == anchor && comment.Marker != nil && contains(provenance.Cases, comment.Marker.CaseID) && commitPattern.MatchString(provenance.AnalyzedCommit) && !contains(exact, provenance.AnalyzedCommit) {
			exact = append(exact, provenance.AnalyzedCommit)
		}
	}
	if len(exact) == 1 {
		return exact[0], "recorded analyzed revision"
	}
	return anchor, "reconstructed from review anchor; analyzed revision unverified"
}

func reactionGroup(record PullRequestRecord, kind CommentKind, id int64) string {
	up, down, other := false, false, false
	for _, r := range record.Reactions {
		if r.CommentKind == kind && r.CommentID == id {
			switch r.Content {
			case "+1":
				up = true
			case "-1":
				down = true
			default:
				other = true
			}
		}
	}
	switch {
	case up && down:
		return "mixed"
	case down:
		return "thumbs-down"
	case up:
		return "thumbs-up"
	case other:
		return "other"
	default:
		return "unreacted"
	}
}
