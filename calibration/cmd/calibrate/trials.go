package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/KellyBennett/Columbo/calibration/harness"
	"github.com/KellyBennett/Columbo/calibration/report"
	"github.com/KellyBennett/Columbo/calibration/trials"
)

func trialsCmd(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: calibrate trials run|leads|report|export [flags]")
	}
	switch args[0] {
	case "run":
		return trialsRun(args[1:])
	case "leads":
		return trialsLeads(args[1:])
	case "report":
		return withStore(args[1:], "report", nil, func(s *harness.Store) error { return report.WriteTrials(os.Stdout, s.DB()) })
	case "export":
		var table, format string
		return withStore(args[1:], "export", func(fs *flag.FlagSet) {
			fs.StringVar(&table, "table", "trials", "trials or lead_checks")
			fs.StringVar(&format, "format", "tsv", "tsv or jsonl")
		}, func(s *harness.Store) error { return report.Export(os.Stdout, s.DB(), table, format) })
	}
	return fmt.Errorf("unknown trials command %q", args[0])
}

func withStore(args []string, name string, extra func(*flag.FlagSet), do func(*harness.Store) error) error {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	db := fs.String("db", "calibration.sqlite", "results database")
	if extra != nil {
		extra(fs)
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	store, err := harness.OpenStore(*db)
	if err != nil {
		return err
	}
	defer store.Close()
	if _, err := trials.NewStore(store.DB()); err != nil {
		return err
	}
	return do(store)
}

func trialsRun(args []string) error {
	fs := flag.NewFlagSet("trials run", flag.ContinueOnError)
	plan := fs.String("plan", "trials.tsv", "trial plan: smell, inject operator, corpus per line")
	columbo := fs.String("columbo", "columbo", "columbo binary")
	db := fs.String("db", "calibration.sqlite", "results database")
	perSmell := fs.Int("per-smell", 5, "trials per smell")
	workers := fs.Int("workers", 4, "agents running at once")
	probes := fs.Int("probes", 8, "mutants probed at once while planting")
	model := fs.String("model", "sonnet", "model passed to the agent command as {model}")
	command := fs.String("agent", trials.DefaultCommand, "agent command; {model} and {turns} are substituted and the prompt arrives on stdin")
	turns := fs.Int("max-turns", 40, "agent turn cap, passed as {turns}")
	timeout := fs.Duration("timeout", 20*time.Minute, "wall-clock limit per agent run")
	out := fs.String("out", "", "directory for each trial's prompt, transcript, and patches")
	keep := fs.Bool("keep", false, "keep each trial workspace")
	smells := fs.String("smells", "", "comma-separated smells to run; empty runs the whole plan")
	limit := fs.Int("limit", 0, "run at most this many scheduled trials; 0 runs all")
	dryRun := fs.Bool("dry-run", false, "plant and schedule, print each plant's prompt, and run no agent")
	if err := fs.Parse(args); err != nil {
		return err
	}
	rows, err := trials.ReadPlan(*plan)
	if err != nil {
		return err
	}
	if *smells != "" {
		want := strings.Split(*smells, ",")
		rows = slices.DeleteFunc(rows, func(r trials.PlanRow) bool { return !slices.Contains(want, r.Smell) })
	}
	bin, err := filepath.Abs(*columbo)
	if err != nil {
		return err
	}
	env := harness.Env{Columbo: bin, Validity: harness.ValidityTest, Timeout: 15 * time.Minute}
	logf := func(f string, a ...any) { fmt.Fprintf(os.Stderr, f+"\n", a...) }
	planter := &trials.Planter{Env: env, PlanDir: filepath.Dir(*plan), Workers: *probes, Log: logf}
	plants, unplanted, err := planter.PlantPlan(rows, *perSmell, *limit)
	defer planter.Cleanup(plants)
	if err != nil {
		return err
	}
	warnUnplanted := func() {
		for _, smell := range unplanted {
			logf("WARNING: %s planted no killed mutant, so it gets no trials. Check its rows in %s.", smell, *plan)
		}
	}
	warnUnplanted()
	defer warnUnplanted()
	scheduled := trials.Schedule(plants, *perSmell)
	if *limit > 0 && len(scheduled) > *limit {
		scheduled = scheduled[:*limit]
	}
	if *dryRun {
		return printSchedule(scheduled)
	}
	path, err := agentPath(bin)
	if err != nil {
		return err
	}
	defer os.RemoveAll(filepath.Dir(path))
	store, err := harness.OpenStore(*db)
	if err != nil {
		return err
	}
	defer store.Close()
	ts, err := trials.NewStore(store.DB())
	if err != nil {
		return err
	}
	agent := trials.CommandAgent{Command: *command, Model: *model, MaxTurns: *turns, Timeout: *timeout,
		Env: []string{"PATH=" + filepath.Dir(path) + string(os.PathListSeparator) + os.Getenv("PATH"), "TMPDIR=" + os.TempDir(), "GOFLAGS=-buildvcs=false", "GOWORK=off"}}
	r := &trials.Runner{Env: env, Agent: agent, Store: ts, Workers: *workers, Out: *out, Keep: *keep, Log: logf}
	return r.Run(scheduled)
}

// agentPath puts columbo on the agent's PATH under its plain name, so the allowed-tools rule can name it.
func agentPath(bin string) (string, error) {
	dir, err := harness.TempDir("calibration-agent-bin-")
	if err != nil {
		return "", err
	}
	link := filepath.Join(dir, "columbo")
	return link, os.Symlink(bin, link)
}

func printSchedule(scheduled []trials.Trial) error {
	seen := map[*trials.Plant]bool{}
	for _, t := range scheduled {
		p := t.Plant
		fmt.Printf("%-28s %-30s %-24s %-60s dose %d repeat %d\n", p.Smell, p.Operator, p.Corpus.Module, p.Site.Symbol, p.Dose, t.Repeat)
	}
	for _, t := range scheduled {
		if seen[t.Plant] {
			continue
		}
		seen[t.Plant] = true
		fmt.Printf("\n===== %s %s %s dose %d\n%s", t.Plant.Smell, t.Plant.Corpus.Module, t.Plant.Site.Symbol, t.Plant.Dose, trials.Prompt(t.Plant.Mutant, t.Plant.Added))
	}
	return nil
}

type multiFlag []string

func (m *multiFlag) String() string     { return strings.Join(*m, ",") }
func (m *multiFlag) Set(v string) error { *m = append(*m, v); return nil }

func trialsLeads(args []string) error {
	fs := flag.NewFlagSet("trials leads", flag.ContinueOnError)
	var corpora multiFlag
	fs.Var(&corpora, "corpus", "fixture directory or pinned module@version; repeatable")
	columbo := fs.String("columbo", "columbo", "columbo binary")
	db := fs.String("db", "calibration.sqlite", "results database")
	validity := fs.String("validity", "test", "test or build")
	workers := fs.Int("workers", 8, "cures probed at once")
	if err := fs.Parse(args); err != nil {
		return err
	}
	bin, err := filepath.Abs(*columbo)
	if err != nil {
		return err
	}
	env := harness.Env{Columbo: bin, Validity: harness.Validity(*validity), Timeout: 15 * time.Minute}
	store, err := harness.OpenStore(*db)
	if err != nil {
		return err
	}
	defer store.Close()
	ts, err := trials.NewStore(store.DB())
	if err != nil {
		return err
	}
	batch, err := ts.NextBatch()
	if err != nil {
		return err
	}
	cwd, _ := os.Getwd()
	for _, spec := range corpora {
		if err := checkLeads(env, ts, batch, spec, cwd, *workers); err != nil {
			return err
		}
	}
	return nil
}

func checkLeads(env harness.Env, ts *trials.Store, batch int64, spec, cwd string, workers int) error {
	src, err := trials.Resolve(spec, cwd)
	if err != nil {
		return err
	}
	c, err := env.PrepareCorpus(src.Dir, nil, src.Module, src.Version)
	if err != nil {
		return fmt.Errorf("%s: %w", spec, err)
	}
	defer os.RemoveAll(c.Root)
	checks, err := trials.LeadChecker{Env: env, Workers: workers}.Check(c)
	if err != nil {
		return err
	}
	prov := trials.Provenance{Module: c.Module, Version: c.Version, Columbo: env.Build(c.Baseline), ConfigHash: harness.ConfigHash(c.Root), Validity: string(env.Validity)}
	counts := map[trials.LeadOutcome]int{}
	for _, ch := range checks {
		counts[ch.Outcome]++
		if err := ts.RecordLead(batch, prov, ch); err != nil {
			return err
		}
	}
	fmt.Fprintf(os.Stderr, "%s: %d cases, %d leads, %v\n", c.Module, len(c.Baseline.Findings), len(checks), counts)
	return nil
}
