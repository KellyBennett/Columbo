package harness

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"sync"
)

type Runner struct {
	Env      Env
	Store    *Store
	Info     RunInfo
	Source   string
	Config   []byte
	Ops      []Operator
	Workers  int
	MaxSites int
	Log      func(format string, args ...any)

	base   string
	corpus *Corpus
	run    int64
}

type job struct {
	op      Operator
	step    Step
	applied Applied
	parent  *result
}

type result struct {
	job     job
	edit    Edit
	outcome Outcome
	dir     string
	id      int64
}

func (r *Runner) logf(format string, args ...any) {
	if r.Log != nil {
		r.Log(format, args...)
	}
}

func (r *Runner) Prepare() error {
	base, err := TempDir("calibration-base-")
	if err != nil {
		return err
	}
	r.base = base
	if err := CopyModule(r.Source, base); err != nil {
		return err
	}
	if r.Config != nil {
		if err := os.WriteFile(filepath.Join(base, ".columbo.yml"), r.Config, 0o644); err != nil {
			return err
		}
	}
	if why := r.Env.Check(base, nil); why != "" {
		return fmt.Errorf("unmutated corpus is not valid: %s", why)
	}
	snap, err := r.Env.Analyze(base)
	if err != nil {
		return err
	}
	pkgs, err := LoadPackages(base)
	if err != nil {
		return err
	}
	r.corpus = &Corpus{Root: base, Module: r.Info.Module, Version: r.Info.Version, Packages: pkgs, Baseline: snap}
	r.Info.Columbo = snap.Version + "+sha256:" + fileHash(r.Env.Columbo)
	r.Info.ConfigHash = configHash(base)
	r.run, err = r.Store.BeginRun(r.Info, snap.Findings)
	r.logf("baseline: %d findings, columbo %s", len(snap.Findings), snap.Version)
	return err
}

func (r *Runner) Cleanup() { _ = os.RemoveAll(r.base) }

func (r *Runner) Run() error {
	var jobs []job
	evaders := map[string][]Operator{}
	for _, op := range r.Ops {
		if ev, ok := op.(Evader); ok && op.Class() == Evade {
			evaders[ev.Follows()] = append(evaders[ev.Follows()], op)
			continue
		}
		jobs = append(jobs, r.plan(op, r.corpus, nil)...)
	}
	results := r.execute(jobs)
	var follow []job
	for _, res := range results {
		if res.outcome.Status != Killed || len(evaders[res.job.op.ID()]) == 0 {
			continue
		}
		mutantCorpus, err := r.reload(res)
		if err != nil {
			r.logf("reload %s: %v", res.job.step.Site.Symbol, err)
			continue
		}
		for _, ev := range evaders[res.job.op.ID()] {
			follow = append(follow, r.plan(ev, mutantCorpus, res)...)
		}
	}
	r.execute(follow)
	for _, res := range results {
		_ = os.RemoveAll(res.dir)
	}
	return nil
}

func (r *Runner) reload(res *result) (*Corpus, error) {
	pkgs, err := LoadPackages(res.dir)
	if err != nil {
		return nil, err
	}
	return &Corpus{Root: res.dir, Module: r.corpus.Module, Version: r.corpus.Version, Packages: pkgs, Baseline: r.corpus.Baseline}, nil
}

func (r *Runner) plan(op Operator, c *Corpus, parent *result) []job {
	sites := op.Sites(c)
	if parent != nil {
		sites = filterSites(sites, parent.job.applied.Touched)
	}
	sites = sample(sites, r.MaxSites)
	var jobs []job
	for _, site := range sites {
		for _, dose := range op.Doses(c, site) {
			applied, err := op.Apply(c, site, dose)
			if err != nil {
				r.logf("%s %s dose %d: not applicable: %v", op.ID(), site.Symbol, dose, err)
				continue
			}
			jobs = append(jobs, job{op: op, step: Step{Operator: op.ID(), Site: site, Dose: dose}, applied: applied, parent: parent})
		}
	}
	return jobs
}

func filterSites(sites []Site, touched []string) []Site {
	var out []Site
	for _, s := range sites {
		for _, t := range touched {
			if s.Symbol == t {
				out = append(out, s)
				break
			}
		}
	}
	return out
}

// sample keeps a deterministic, content-addressed subset so adding a site elsewhere in the
// corpus does not reshuffle which sites a later run measures.
func sample(sites []Site, max int) []Site {
	if max <= 0 || len(sites) <= max {
		return sites
	}
	type keyed struct {
		key  string
		site Site
	}
	ks := make([]keyed, len(sites))
	for i, s := range sites {
		h := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%d", s.Package, s.Symbol, s.Offset)))
		ks[i] = keyed{hex.EncodeToString(h[:]), s}
	}
	sort.Slice(ks, func(i, j int) bool { return ks[i].key < ks[j].key })
	out := make([]Site, max)
	for i := range out {
		out[i] = ks[i].site
	}
	return out
}

func (r *Runner) execute(jobs []job) []*result {
	results := make([]*result, len(jobs))
	workers := r.Workers
	if workers <= 0 {
		workers = 1
	}
	var wg sync.WaitGroup
	ch := make(chan int)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range ch {
				results[i] = r.evaluate(jobs[i])
			}
		}()
	}
	for i := range jobs {
		ch <- i
	}
	close(ch)
	wg.Wait()
	for _, res := range results {
		var parentID int64
		if res.job.parent != nil {
			parentID = res.job.parent.id
		}
		id, err := r.Store.Record(r.run, parentID, res.outcome)
		if err != nil {
			r.logf("record: %v", err)
		}
		res.id = id
		r.logf("%-8s %-28s %-60s dose %-2d %-10s %s", res.outcome.Class, res.job.op.ID(), shortSymbol(res.job.step.Site.Symbol), res.job.step.Dose, res.outcome.Status, res.outcome.Reason)
	}
	return results
}

func (r *Runner) evaluate(j job) *result {
	edit := j.applied.Edit
	chain := []Step{j.step}
	if j.parent != nil {
		edit = mergeEdits(j.parent.edit, j.applied.Edit)
		chain = append(append([]Step{}, j.parent.job.step), j.step)
	}
	res := &result{job: j, edit: edit}
	res.outcome = Outcome{Class: j.op.Class(), Target: j.op.Target(), Mutant: Mutant{Chain: chain}}
	patch, size, err := Measure(r.base, edit)
	if err != nil {
		res.outcome.Status, res.outcome.Reason = Invalid, "measure: "+err.Error()
		return res
	}
	res.outcome.Mutant.Patch, res.outcome.Mutant.Size = patch, size
	dir, err := TempDir("calibration-mutant-")
	if err != nil {
		res.outcome.Status, res.outcome.Reason = Invalid, err.Error()
		return res
	}
	res.dir = dir
	keep := false
	defer func() {
		if !keep {
			_ = os.RemoveAll(dir)
			res.dir = ""
		}
	}()
	if err := CopyModule(r.base, dir); err != nil {
		res.outcome.Status, res.outcome.Reason = Invalid, err.Error()
		return res
	}
	if err := WriteEdit(dir, edit); err != nil {
		res.outcome.Status, res.outcome.Reason = Invalid, err.Error()
		return res
	}
	if why := r.Env.Check(dir, editedPackages(edit)); why != "" {
		res.outcome.Status, res.outcome.Reason = Invalid, why
		return res
	}
	snap, err := r.Env.Analyze(dir)
	if err != nil {
		res.outcome.Status, res.outcome.Reason = Invalid, err.Error()
		return res
	}
	Score(&res.outcome, j.applied.Expectation, r.corpus.Baseline.Findings, snap.Findings)
	keep = j.parent == nil && res.outcome.Status == Killed
	return res
}

func mergeEdits(a, b Edit) Edit {
	out := Edit{}
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}

func editedPackages(edit Edit) []string {
	seen := map[string]bool{}
	var out []string
	for rel := range edit {
		dir := "./" + path.Dir(rel)
		if !seen[dir] {
			seen[dir] = true
			out = append(out, dir)
		}
	}
	sort.Strings(out)
	return out
}

func fileHash(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return "unreadable"
	}
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:8])
}

func configHash(root string) string {
	data, err := os.ReadFile(filepath.Join(root, ".columbo.yml"))
	if err != nil {
		return "default"
	}
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:8])
}
