package trials

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KellyBennett/Columbo/calibration/harness"
	"github.com/KellyBennett/Columbo/calibration/internal/columbotest"
)

type fakeAgent struct {
	name string
	fix  func(t *testing.T, dir string)
	t    *testing.T
}

func (f fakeAgent) Describe() (string, string) { return f.name, "none" }

func (f fakeAgent) Fix(_ context.Context, dir, _ string) AgentRun {
	f.fix(f.t, dir)
	return AgentRun{Stop: "success", Turns: 1}
}

func fixtureEnv(t *testing.T) harness.Env {
	t.Helper()
	if testing.Short() {
		t.Skip("runs columbo and go test on fixture modules")
	}
	columbotest.ResolveTempDir(t)
	return harness.Env{Columbo: columbotest.Binary(t), Validity: harness.ValidityTest}
}

var inlineRow = PlanRow{Smell: "long-function", Operator: "inline-function", Corpus: "fixtures/inline"}

func plantRows(t *testing.T, rows ...PlanRow) (harness.Env, []*Plant) {
	t.Helper()
	env := fixtureEnv(t)
	pl := &Planter{Env: env, PlanDir: "..", Workers: 4}
	plants, err := pl.Plant(rows)
	t.Cleanup(func() { pl.Cleanup(plants) })
	if err != nil {
		t.Fatal(err)
	}
	if len(plants) == 0 {
		t.Fatalf("%v planted nothing", rows)
	}
	return env, plants
}

func plantInline(t *testing.T) (harness.Env, []*Plant) {
	t.Helper()
	return plantRows(t, inlineRow)
}

func copyFile(t *testing.T, from, to string) {
	t.Helper()
	data, err := os.ReadFile(from)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(to, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func workspace(t *testing.T, p *Plant, fix func(dir string)) string {
	t.Helper()
	ws := t.TempDir()
	if err := harness.CopyModule(p.Dir, ws); err != nil {
		t.Fatal(err)
	}
	fix(ws)
	return ws
}

func TestRestoringTheOriginalClearsGreenAtDistanceZero(t *testing.T) {
	env, plants := plantInline(t)
	p := plants[0]
	store, err := harness.OpenStore(filepath.Join(t.TempDir(), "calibration.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ts, err := NewStore(store.DB())
	if err != nil {
		t.Fatal(err)
	}
	restore := fakeAgent{name: "restore", t: t, fix: func(t *testing.T, dir string) {
		copyFile(t, filepath.Join(p.Corpus.Root, "order.go"), filepath.Join(dir, "order.go"))
	}}
	r := &Runner{Env: env, Agent: restore, Store: ts, Workers: 1}
	trials := Schedule([]*Plant{p}, 1)
	if err := r.Run(trials); err != nil {
		t.Fatal(err)
	}
	var cleared, vet, introduced int
	var tests string
	var distance float64
	if err := store.DB().QueryRow(`SELECT cleared, vet_pass, tests, introduced, fix_distance FROM trials`).Scan(&cleared, &vet, &tests, &introduced, &distance); err != nil {
		t.Fatal(err)
	}
	if cleared != 1 || vet != 1 || tests != "pass" || introduced != 0 || distance != 0 {
		t.Fatalf("restored original scored cleared=%d vet=%d tests=%s introduced=%d distance=%.2f", cleared, vet, tests, introduced, distance)
	}
	if err := r.Run(trials); err != nil {
		t.Fatal(err)
	}
	var rows int
	if err := store.DB().QueryRow(`SELECT count(*) FROM trials`).Scan(&rows); err != nil || rows != 1 {
		t.Fatalf("rerun left %d rows (%v), want the recorded trial skipped", rows, err)
	}
}

func TestAnIdleAgentLeavesTheCaseAtDistanceOne(t *testing.T) {
	env, plants := plantInline(t)
	p := plants[0]
	s, err := ScoreFix(env, p, workspace(t, p, func(string) {}))
	if err != nil {
		t.Fatal(err)
	}
	if s.Cleared() || s.TargetRemaining == 0 || s.Distance.Normalized() != 1 || s.FilesChanged != 0 {
		t.Fatalf("idle agent scored %+v", s)
	}
}

func TestASuppressionIsGamingNotAFix(t *testing.T) {
	env, plants := plantInline(t)
	p := plants[0]
	ws := workspace(t, p, func(dir string) {
		path := filepath.Join(dir, "order.go")
		src, _ := os.ReadFile(path)
		marked := strings.Replace(string(src), "func Total(", "// columbo:ignore long-function -- the order total reads best as one sequence\nfunc Total(", 1)
		if err := os.WriteFile(path, []byte(marked), 0o644); err != nil {
			t.Fatal(err)
		}
	})
	s, err := ScoreFix(env, p, ws)
	if err != nil {
		t.Fatal(err)
	}
	if !s.Gamed() || s.Cleared() || s.Suppressed != 1 {
		t.Fatalf("suppression scored %+v", s)
	}
}

func TestAConfigChangeIsUndoneBeforeScoring(t *testing.T) {
	env, plants := plantInline(t)
	p := plants[0]
	ws := workspace(t, p, func(dir string) {
		if err := os.WriteFile(filepath.Join(dir, ".columbo.yml"), []byte("version: 1\nthresholds:\n  function-lines: 100\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	})
	s, err := ScoreFix(env, p, ws)
	if err != nil {
		t.Fatal(err)
	}
	if s.Cleared() || s.TargetRemaining == 0 || !hasFlag(s, "config") {
		t.Fatalf("raised threshold scored %+v", s)
	}
}

func newStore(t *testing.T) (*harness.Store, *Store) {
	t.Helper()
	store, err := harness.OpenStore(filepath.Join(t.TempDir(), "calibration.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	ts, err := NewStore(store.DB())
	if err != nil {
		t.Fatal(err)
	}
	return store, ts
}

func TestRescoringReplaysTheStoredFixUnderTheCurrentScorer(t *testing.T) {
	env, plants := plantInline(t)
	p := plants[0]
	before, beforeStore := newStore(t)
	restore := fakeAgent{name: "restore", t: t, fix: func(t *testing.T, dir string) {
		copyFile(t, filepath.Join(p.Corpus.Root, "order.go"), filepath.Join(dir, "order.go"))
	}}
	if err := (&Runner{Env: env, Agent: restore, Store: beforeStore, Workers: 1}).Run(Schedule([]*Plant{p}, 1)); err != nil {
		t.Fatal(err)
	}
	stored, err := ReadStored(before.DB())
	if err != nil || len(stored) != 1 {
		t.Fatalf("read %d stored trials (%v), want 1", len(stored), err)
	}
	drifted := stored[0]
	drifted.Prompt += "\nan instruction the agent never saw"
	after, afterStore := newStore(t)
	r := &Runner{Env: env, Store: afterStore, Workers: 1}
	if err := r.Rescore([]Stored{stored[0], drifted}, plants); err != nil {
		t.Fatal(err)
	}
	var rows, cleared, turns int
	var tests, agentBuild string
	if err := after.DB().QueryRow(`SELECT count(*), sum(cleared), sum(turns), max(tests), max(agent_columbo) FROM trials`).Scan(&rows, &cleared, &turns, &tests, &agentBuild); err != nil {
		t.Fatal(err)
	}
	if rows != 1 || cleared != 1 || turns != 1 || tests != "pass" || agentBuild != env.Build(p.Mutant) {
		t.Fatalf("rescored %d rows: cleared=%d turns=%d tests=%s agent build %q; want the one trial whose prompt still matches", rows, cleared, turns, tests, agentBuild)
	}
}

func TestADatabaseFromBeforeTheScorerChangeIsRefused(t *testing.T) {
	store, err := harness.OpenStore(filepath.Join(t.TempDir(), "calibration.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.DB().Exec(`CREATE TABLE trials (id INTEGER PRIMARY KEY, trial_key TEXT, tests_pass INTEGER)`); err != nil {
		t.Fatal(err)
	}
	if _, err := NewStore(store.DB()); err == nil || !strings.Contains(err.Error(), "rescore") {
		t.Fatalf("old trials table opened with error %v, want a pointer to rescore", err)
	}
}
