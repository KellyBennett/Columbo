package trials

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/KellyBennett/Columbo/calibration/harness"
)

// Runner hands each scheduled trial's mutant to the agent in a fresh workspace and scores what comes back.
type Runner struct {
	Env     harness.Env
	Agent   Agent
	Store   *Store
	Workers int
	// Out keeps each trial's prompt, transcript, and fix patch for inspection; empty keeps nothing.
	Out  string
	Keep bool
	Log  func(format string, args ...any)

	mu sync.Mutex
}

// Key identifies a trial across reruns: the same mutant, repeat, agent command, model, prompt wording, and Columbo build.
// A different build makes a different trial, so a rerun never pools numbers from two builds under one key.
func Key(t Trial, command, model, build string) string {
	p := t.Plant
	h := sha256.Sum256([]byte(strings.Join([]string{p.Smell, p.Operator, p.Corpus.Module, p.Corpus.Version,
		p.Site.Symbol, fmt.Sprint(p.Site.Offset), fmt.Sprint(p.Dose), fmt.Sprint(t.Repeat), command, model, PromptVersion, build}, "|")))
	return hex.EncodeToString(h[:12])
}

// fixer stands in for the agent's turn: it changes the workspace and says how the run went.
type fixer func(ws, prompt string) (AgentRun, error)

// job is one trial to score, with who fixed it and which Columbo build the fixer read.
type job struct {
	trial          Trial
	command, model string
	agentBuild     string
	fix            fixer
}

// Run skips trials the store already holds, so an interrupted batch resumes where it stopped.
func (r *Runner) Run(trials []Trial) error {
	command, model := r.Agent.Describe()
	jobs := make([]job, len(trials))
	for i, t := range trials {
		jobs[i] = job{trial: t, command: command, model: model, agentBuild: r.build(t.Plant), fix: func(ws, prompt string) (AgentRun, error) {
			return r.Agent.Fix(context.Background(), ws, prompt), nil
		}}
	}
	return r.runJobs(jobs)
}

// Rescore replays stored fixes on fresh copies of their mutants with the current build and scorer. A stored trial is
// replayed only when the current build gives its mutant the prompt the agent saw, since a different prompt is a different trial.
func (r *Runner) Rescore(stored []Stored, plants []*Plant) error {
	var jobs []job
	for _, st := range stored {
		p := st.plant(plants)
		if p == nil {
			return fmt.Errorf("no current mutant matches the stored %s trial on %s at %s; rerun it instead", st.Smell, st.Module, st.Site)
		}
		if Prompt(p.Mutant, p.Added) != st.Prompt {
			r.logf("WARNING: the current build gives %s a different prompt than the agent saw, so it is not rescored; rerun it", shortName(p))
			continue
		}
		jobs = append(jobs, job{trial: Trial{Plant: p, Repeat: st.Repeat}, command: st.Command, model: st.Model, agentBuild: st.AgentColumbo,
			fix: func(ws, _ string) (AgentRun, error) { return st.Run, applyPatch(ws, st.FixPatch) }})
	}
	return r.runJobs(jobs)
}

func (st Stored) plant(plants []*Plant) *Plant {
	for _, p := range plants {
		if p.Smell == st.Smell && p.Operator == st.Operator && p.Corpus.Module == st.Module && p.Corpus.Version == st.Version &&
			bytes.Equal(p.Patch, st.MutantPatch) {
			return p
		}
	}
	return nil
}

func (r *Runner) runJobs(jobs []job) error {
	var todo []job
	for _, j := range jobs {
		done, err := r.Store.Done(r.key(j))
		if err != nil {
			return err
		}
		if !done {
			todo = append(todo, j)
		}
	}
	r.logf("%d trials scheduled, %d already recorded", len(jobs), len(jobs)-len(todo))
	errs := make([]error, len(todo))
	harness.ForEach(len(todo), r.Workers, func(i int) { errs[i] = r.attempt(todo[i]) })
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *Runner) key(j job) string { return Key(j.trial, j.command, j.model, r.build(j.trial.Plant)) }

func (r *Runner) build(p *Plant) string { return r.Env.Build(p.Mutant) }

func (r *Runner) attempt(j job) error {
	t := j.trial
	p := t.Plant
	prompt := Prompt(p.Mutant, p.Added)
	ws, err := harness.TempDir("calibration-trial-")
	if err != nil {
		return err
	}
	if !r.Keep {
		defer os.RemoveAll(ws)
	}
	if err := harness.CopyModule(p.Dir, ws); err != nil {
		return err
	}
	run, err := j.fix(ws, prompt)
	if err != nil {
		return fmt.Errorf("%s: %w", shortName(p), err)
	}
	row := Row{Key: r.key(j), Trial: t, Prov: r.provenance(p), AgentColumbo: j.agentBuild, Command: j.command, Model: j.model, Prompt: prompt, Run: run}
	if run.InfraError == "" {
		sc, err := ScoreFix(r.Env, p, ws)
		if err != nil {
			return fmt.Errorf("score %s: %w", shortName(p), err)
		}
		row.Score = &sc
	}
	if err := r.keep(t, row, ws); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.logf("%s", summary(t, row))
	return r.Store.Record(row)
}

func (r *Runner) provenance(p *Plant) Provenance {
	return Provenance{Module: p.Corpus.Module, Version: p.Corpus.Version, Columbo: r.build(p),
		ConfigHash: harness.ConfigHash(p.Corpus.Root), Validity: string(r.Env.Validity)}
}

func (r *Runner) keep(t Trial, row Row, ws string) error {
	if r.Out == "" {
		return nil
	}
	dir := filepath.Join(r.Out, fmt.Sprintf("%s-r%d", shortName(t.Plant), t.Repeat))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	files := map[string][]byte{"prompt.md": []byte(row.Prompt), "transcript.jsonl": row.Run.Transcript, "mutant.patch": t.Plant.Patch, "workspace.txt": []byte(ws + "\n")}
	if row.Score != nil {
		files["fix.patch"] = row.Score.Patch
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func summary(t Trial, row Row) string {
	head := fmt.Sprintf("%-72s r%d %-14s %3d turns $%.2f %4.0fs", shortName(t.Plant), t.Repeat, row.Run.Stop, row.Run.Turns, row.Run.CostUSD, row.Run.Wall.Seconds())
	if row.Score == nil {
		return head + " infra: " + row.Run.InfraError
	}
	s := row.Score
	return fmt.Sprintf("%s cleared=%v vet=%v tests=%s introduced=%d out-of-scope-gone=%d distance=%.2f (%d/%d) flags=%s",
		head, s.Cleared(), s.VetPass, s.Tests, len(s.Introduced), len(s.OutOfScopeGone), s.Distance.Normalized(), s.Distance.Fixed, s.Distance.Mutant, flagList(s.Flags))
}

func flagList(fs []Flag) string {
	out := make([]string, len(fs))
	for i, f := range fs {
		out[i] = f.String()
	}
	return strings.Join(out, ", ")
}

func (r *Runner) logf(format string, args ...any) {
	if r.Log != nil {
		r.Log(format, args...)
	}
}

func shortSymbol(symbol string) string {
	if i := strings.LastIndex(symbol, "/"); i >= 0 {
		symbol = symbol[i+1:]
	}
	return strings.NewReplacer("(", "", ")", "", "*", "").Replace(symbol)
}
