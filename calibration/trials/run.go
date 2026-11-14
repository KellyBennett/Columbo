package trials

import (
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

// Key identifies a trial across reruns: the same mutant, repeat, agent, model, and prompt wording.
func (r *Runner) Key(t Trial) string {
	command, model := r.Agent.Describe()
	p := t.Plant
	h := sha256.Sum256([]byte(strings.Join([]string{p.Smell, p.Operator, p.Corpus.Module, p.Corpus.Version,
		p.Site.Symbol, fmt.Sprint(p.Site.Offset), fmt.Sprint(p.Dose), fmt.Sprint(t.Repeat), command, model, PromptVersion}, "|")))
	return hex.EncodeToString(h[:12])
}

// Run skips trials the store already holds, so an interrupted batch resumes where it stopped.
func (r *Runner) Run(trials []Trial) error {
	var todo []Trial
	for _, t := range trials {
		done, err := r.Store.Done(r.Key(t))
		if err != nil {
			return err
		}
		if !done {
			todo = append(todo, t)
		}
	}
	r.logf("%d trials scheduled, %d already recorded", len(trials), len(trials)-len(todo))
	errs := make([]error, len(todo))
	harness.ForEach(len(todo), r.Workers, func(i int) { errs[i] = r.attempt(todo[i]) })
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *Runner) attempt(t Trial) error {
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
	run := r.Agent.Fix(context.Background(), ws, prompt)
	command, model := r.Agent.Describe()
	row := Row{Key: r.Key(t), Trial: t, Prov: r.provenance(p), Command: command, Model: model, Prompt: prompt, Run: run}
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
	return Provenance{Module: p.Corpus.Module, Version: p.Corpus.Version, Columbo: r.Env.Build(p.Mutant),
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
	tests := "skip"
	if s.TestsPass != nil {
		tests = fmt.Sprint(*s.TestsPass)
	}
	return fmt.Sprintf("%s cleared=%v vet=%v tests=%s introduced=%d out-of-scope-gone=%d distance=%.2f (%d/%d) flags=%s",
		head, s.Cleared(), s.VetPass, tests, len(s.Introduced), len(s.OutOfScopeGone), s.Distance.Normalized(), s.Distance.Fixed, s.Distance.Mutant, strings.Join(s.Flags, ","))
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
