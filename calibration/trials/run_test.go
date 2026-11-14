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

func plantInline(t *testing.T) (harness.Env, []*Plant) {
	t.Helper()
	if testing.Short() {
		t.Skip("runs columbo and go test on fixture modules")
	}
	columbotest.ResolveTempDir(t)
	env := harness.Env{Columbo: columbotest.Binary(t), Validity: harness.ValidityTest}
	pl := &Planter{Env: env, PlanDir: "..", Workers: 4}
	plants, err := pl.Plant([]PlanRow{{Smell: "long-function", Operator: "inline-function", Corpus: "fixtures/inline"}})
	t.Cleanup(func() { pl.Cleanup(plants) })
	if err != nil {
		t.Fatal(err)
	}
	if len(plants) == 0 {
		t.Fatal("inline-function planted nothing on fixtures/inline")
	}
	return env, plants
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
	var cleared, vet, tests, introduced int
	var distance float64
	if err := store.DB().QueryRow(`SELECT cleared, vet_pass, tests_pass, introduced, fix_distance FROM trials`).Scan(&cleared, &vet, &tests, &introduced, &distance); err != nil {
		t.Fatal(err)
	}
	if cleared != 1 || vet != 1 || tests != 1 || introduced != 0 || distance != 0 {
		t.Fatalf("restored original scored cleared=%d vet=%d tests=%d introduced=%d distance=%.2f", cleared, vet, tests, introduced, distance)
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
	if s.Cleared() || s.TargetRemaining == 0 || !strings.Contains(strings.Join(s.Flags, ","), "config") {
		t.Fatalf("raised threshold scored %+v", s)
	}
}
