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
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/KellyBennett/Columbo/internal/feedbackdb"
)

const pilotVersion = 4

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
	FullTests          bool          `json:"full_tests"`
	Parallel           int           `json:"-"`
}

type PilotCase struct {
	ActualChange     ActualChange `json:"actual_change"`
	ID               string       `json:"id"`
	CaseID           string       `json:"case_id"`
	Rule             string       `json:"rule"`
	PR               int64        `json:"pr"`
	Reaction         string       `json:"reaction"`
	Finding          Comment      `json:"finding"`
	Before           string       `json:"before"`
	After            string       `json:"after,omitempty"`
	RevisionEvidence string       `json:"revision_evidence"`
	Initial          Observation  `json:"initial"`
	Latest           Observation  `json:"latest"`
}

func (c PilotCase) explores() bool { return c.Reaction == "thumbs-down" }
func (c PilotCase) analysisMode() string {
	if c.explores() {
		return "exploratory-refactoring"
	}
	return "actual-changes"
}

type PilotPlan struct {
	ActualChangeCounts map[string]ActualChangeCounts `json:"actual_change_counts"`
	Version            int                           `json:"version"`
	Options            PilotOptions                  `json:"options"`
	Cases              []PilotCase                   `json:"cases"`
	Excluded           []string                      `json:"excluded"`
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
	flags.Var(&o.Checks, "check", "validation shell command; repeat to replace the default go build, vet and test checks")
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
	flags.IntVar(&o.Parallel, "parallel", 1, "number of trials to run at once")
	flags.BoolVar(&o.FullTests, "full-tests", false, "run the default go test check on the whole module instead of affected packages")
	flags.StringVar(&o.Config, "config", "", "configuration path within the module (default captured CI config, then .columbo.yml)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 || !repositoryPattern.MatchString(o.Repository) || o.Out == "" || o.Limit < 1 || o.Parallel < 1 || o.Turns < 1 || o.Attempts < 1 || o.Timeout <= 0 || o.CheckTimeout <= 0 {
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
	failures := runPilotCases(ctx, options, plan.Cases, out)
	if err := pilotSummary(options.Out, plan); err != nil {
		return err
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	fmt.Fprintln(out, "Pilot report:", filepath.Join(options.Out, "report.md"))
	if failures > 0 {
		return fmt.Errorf("%d trials incomplete; rerun the same command to retry (earlier attempts retained)", failures)
	}
	return nil
}

func runPilotCases(ctx context.Context, options PilotOptions, cases []PilotCase, out io.Writer) (failures int) {
	var mu sync.Mutex
	var workers sync.WaitGroup
	indices := make(chan int)
	for range max(options.Parallel, 1) {
		workers.Go(func() {
			for i := range indices {
				finding := cases[i]
				label := fmt.Sprintf("[%d/%d] PR %d %s (%s)", i+1, len(cases), finding.PR, finding.CaseID, finding.Reaction)
				mu.Lock()
				fmt.Fprintln(out, label+": started")
				mu.Unlock()
				err := runPilotCase(ctx, options, finding)
				mu.Lock()
				if err != nil {
					failures++
					fmt.Fprintf(out, "%s: incomplete: %v\n", label, err)
				} else {
					fmt.Fprintln(out, label+": done")
				}
				mu.Unlock()
			}
		})
	}
	for i := 0; i < len(cases) && ctx.Err() == nil; i++ {
		select {
		case indices <- i:
		case <-ctx.Done():
		}
	}
	close(indices)
	workers.Wait()
	return failures
}

func loadOrPlan(ctx context.Context, options PilotOptions) (PilotPlan, error) {
	path := filepath.Join(options.Out, "plan.json")
	data, err := os.ReadFile(path)
	if err == nil {
		return resumePlan(data, options)
	} else if !os.IsNotExist(err) {
		return PilotPlan{}, err
	}
	observations, err := readObservations(ctx, options)
	if err != nil {
		return PilotPlan{}, err
	}
	plan := makePilotPlan(ctx, options, observations)
	if err := ctx.Err(); err != nil {
		return plan, err
	}
	return plan, saveJSON(path, plan)
}

func resumePlan(data []byte, options PilotOptions) (PilotPlan, error) {
	var plan PilotPlan
	if err := json.Unmarshal(data, &plan); err != nil {
		return plan, err
	}
	if plan.Version != pilotVersion {
		return plan, fmt.Errorf("pilot analysis policy changed; choose a new --out directory to preserve earlier results")
	}
	before, _ := json.Marshal(plan.Options)
	after, _ := json.Marshal(options)
	if !bytesEqual(before, after) {
		return plan, fmt.Errorf("pilot options or executable hashes changed; choose a new --out directory")
	}
	return plan, nil
}

func readObservations(ctx context.Context, options PilotOptions) ([]Observation, error) {
	if _, err := os.Stat(options.Store); err != nil {
		return nil, fmt.Errorf("collect feedback first: %w", err)
	}
	store, err := feedbackdb.Open(options.Store)
	if err != nil {
		return nil, err
	}
	defer store.Close()
	rows, err := store.Queries().Observations(ctx, feedbackdb.ObservationsParams{Repository: options.Repository})
	if err != nil {
		return nil, err
	}
	observations := make([]Observation, 0, len(rows))
	for _, row := range rows {
		var observation Observation
		if err := json.Unmarshal([]byte(row.Payload), &observation); err != nil {
			return nil, err
		}
		observations = append(observations, observation)
	}
	return observations, nil
}

func bytesEqual(a, b []byte) bool { return string(a) == string(b) }

func saveJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return writePrivate(path, append(data, '\n'))
}

var reactionGroupOrder = []string{"thumbs-down", "thumbs-up", "unreacted", "mixed", "other"}

func makePilotPlan(ctx context.Context, options PilotOptions, observations []Observation) PilotPlan {
	plan := PilotPlan{Version: pilotVersion, Options: options, ActualChangeCounts: map[string]ActualChangeCounts{}}
	candidates, excluded := eligibleFindings(options, observations)
	plan.Excluded = excluded
	for i, finding := range candidates {
		if ctx.Err() != nil {
			return plan
		}
		candidates[i].ActualChange = finding.inspectActualChange(ctx)
		counts := plan.ActualChangeCounts[finding.Reaction]
		counts.add(candidates[i].ActualChange)
		plan.ActualChangeCounts[finding.Reaction] = counts
	}
	plan.Cases = samplePilotCases(options, candidates)
	return plan
}

func eligibleFindings(options PilotOptions, observations []Observation) (candidates []PilotCase, excluded []string) {
	latest := map[int64]Observation{}
	for _, observation := range observations {
		latest[observation.Record.PullRequest.Number] = observation
	}
	seen := map[string]bool{}
	for _, observation := range observations {
		pr := observation.Record.PullRequest.Number
		for _, comment := range observation.Record.Comments {
			if !options.selects(pr, comment) {
				continue
			}
			key := strconv.FormatInt(pr, 10) + "/" + comment.Marker.CaseID
			if seen[key] {
				continue
			}
			seen[key] = true
			finding := newPilotCase(key, comment, observation, latest[pr])
			if reason := finding.exclusion(options); reason != "" {
				excluded = append(excluded, key+": "+reason)
			} else {
				candidates = append(candidates, finding)
			}
		}
	}
	return candidates, excluded
}

func (options PilotOptions) selects(pr int64, comment Comment) bool {
	return (options.PR == 0 || options.PR == pr) && comment.Marker != nil && comment.Marker.Role == "finding" && (len(options.Cases) == 0 || contains(options.Cases, comment.Marker.CaseID))
}

func newPilotCase(key string, comment Comment, initial, latest Observation) PilotCase {
	finding := PilotCase{ID: shortHash(key), CaseID: comment.Marker.CaseID, Rule: comment.Marker.Rule, PR: initial.Record.PullRequest.Number, Finding: comment, Initial: initial, Latest: latest}
	finding.Before, finding.RevisionEvidence = findingRevision(comment, initial)
	finding.Reaction = reactionGroup(latest.Record, comment.Kind, comment.ID)
	for _, revision := range latest.Revisions {
		if contains(revision.Roles, "pr-head") && revision.Present {
			finding.After = revision.SHA
		}
	}
	return finding
}

func (c PilotCase) exclusion(options PilotOptions) string {
	switch {
	case c.Before == "":
		return "no review-anchor revision"
	case c.RevisionEvidence != "recorded analyzed revision" && !options.AllowReconstructed:
		return "requires --allow-reconstructed"
	case c.Initial.GitDir == "":
		return "history not retained; collect with --checkout"
	case !revisionPresent(c.Initial, c.Before):
		return "starting revision unavailable"
	}
	return ""
}

func (c PilotCase) prioritized() bool {
	return !c.explores() && c.ActualChange.Status == actualChanged
}

func samplePilotCases(options PilotOptions, candidates []PilotCase) []PilotCase {
	groups := rotations(options.Seed, candidates)
	var cases []PilotCase
	for added := true; added && len(cases) < options.Limit; {
		added = false
		for _, group := range reactionGroupOrder {
			if len(cases) == options.Limit {
				break
			}
			if finding, ok := groups[group].next(); ok {
				cases, added = append(cases, finding), true
			}
		}
	}
	return cases
}

type rotation struct {
	buckets  [][]PilotCase
	position int
}

func rotations(seed string, candidates []PilotCase) map[string]*rotation {
	buckets := map[string][]PilotCase{}
	for _, finding := range candidates {
		key := finding.Reaction + "/" + finding.Rule
		buckets[key] = append(buckets[key], finding)
	}
	groups := map[string]*rotation{}
	for _, key := range slices.Sorted(maps.Keys(buckets)) {
		bucket := buckets[key]
		sort.Slice(bucket, func(i, j int) bool {
			if bucket[i].prioritized() != bucket[j].prioritized() {
				return bucket[i].prioritized()
			}
			return shortHash(seed+bucket[i].ID) < shortHash(seed+bucket[j].ID)
		})
		group := bucket[0].Reaction
		if groups[group] == nil {
			groups[group] = &rotation{}
		}
		groups[group].buckets = append(groups[group].buckets, bucket)
	}
	return groups
}

func (r *rotation) next() (PilotCase, bool) {
	if r == nil {
		return PilotCase{}, false
	}
	preferred := slices.ContainsFunc(r.buckets, func(bucket []PilotCase) bool { return len(bucket) > 0 && bucket[0].prioritized() })
	for range len(r.buckets) {
		bucket := &r.buckets[r.position%len(r.buckets)]
		r.position++
		if len(*bucket) == 0 || (preferred && !(*bucket)[0].prioritized()) {
			continue
		}
		finding := (*bucket)[0]
		*bucket = (*bucket)[1:]
		return finding, true
	}
	return PilotCase{}, false
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
