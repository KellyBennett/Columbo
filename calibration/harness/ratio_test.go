package harness

import (
	"database/sql"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

const preRatioSchema = `
CREATE TABLE runs (id INTEGER PRIMARY KEY, corpus_module TEXT NOT NULL, corpus_version TEXT NOT NULL,
 columbo_version TEXT NOT NULL, config_hash TEXT NOT NULL, validity TEXT NOT NULL);
CREATE TABLE baseline_findings (run_id INTEGER NOT NULL REFERENCES runs(id),
 smell TEXT NOT NULL, symbol TEXT NOT NULL, file TEXT NOT NULL, case_id TEXT NOT NULL, verdict TEXT NOT NULL);
CREATE TABLE mutants (id INTEGER PRIMARY KEY, run_id INTEGER NOT NULL REFERENCES runs(id), parent_id INTEGER REFERENCES mutants(id),
 class TEXT NOT NULL, operator TEXT NOT NULL, target TEXT NOT NULL, site_package TEXT NOT NULL, site_symbol TEXT NOT NULL,
 dose INTEGER NOT NULL, chain TEXT NOT NULL, size INTEGER NOT NULL, status TEXT NOT NULL, reason TEXT NOT NULL,
 predicted_kill INTEGER, prediction TEXT NOT NULL, patch BLOB NOT NULL);
CREATE TABLE mutant_findings (mutant_id INTEGER NOT NULL REFERENCES mutants(id),
 side TEXT NOT NULL CHECK (side IN ('added','removed')),
 smell TEXT NOT NULL, symbol TEXT NOT NULL, case_id TEXT NOT NULL, verdict TEXT NOT NULL);
INSERT INTO runs VALUES (1, 'example.com/old', 'v0', 'columbo', 'h', 'test');
INSERT INTO baseline_findings VALUES (1, 'long-function', 'example.com/old.f', 'a.go', 'C-old', 'FAIL');
`

func ratio(v float64) *float64 { return &v }

func TestBeginRunAddsRatioColumnsToAnOldDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "calibration.sqlite")
	old, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := old.Exec(preRatioSchema); err != nil {
		t.Fatal(err)
	}
	old.Close()
	store, err := OpenStore(path)
	if err != nil {
		t.Fatalf("opening an old database: %v", err)
	}
	defer store.Close()
	finding := Finding{Smell: "long-function", Symbol: "example.com/m.f", File: "a.go", CaseID: "C-1", Verdict: "FAIL", Ratio: ratio(12.0 / 11)}
	run, err := store.BeginRun(RunInfo{Module: "example.com/m", Validity: ValidityTest}, []Finding{finding})
	if err != nil {
		t.Fatal(err)
	}
	unmeasured := finding
	unmeasured.Ratio = nil
	step := Step{Operator: "inline-function", Site: Site{Package: "example.com/m", Symbol: "example.com/m.f"}, Dose: 3}
	mutant, err := store.Record(run, 0, Outcome{Mutant: Mutant{Chain: []Step{step}, Patch: []byte{}}, Class: Inject, Target: "long-function",
		Added: []Finding{finding}, Removed: []Finding{unmeasured}, Status: Killed})
	if err != nil {
		t.Fatal(err)
	}
	var oldRatio, baseline, added, removed sql.NullFloat64
	db := store.DB()
	for _, q := range []struct {
		dest  *sql.NullFloat64
		query string
		args  []any
	}{
		{&oldRatio, `SELECT ratio FROM baseline_findings WHERE case_id = 'C-old'`, nil},
		{&baseline, `SELECT ratio FROM baseline_findings WHERE run_id = ?`, []any{run}},
		{&added, `SELECT ratio FROM mutant_findings WHERE mutant_id = ? AND side = 'added'`, []any{mutant}},
		{&removed, `SELECT ratio FROM mutant_findings WHERE mutant_id = ? AND side = 'removed'`, []any{mutant}},
	} {
		if err := db.QueryRow(q.query, q.args...).Scan(q.dest); err != nil {
			t.Fatalf("%s: %v", q.query, err)
		}
	}
	if oldRatio.Valid {
		t.Errorf("a row recorded before the migration has ratio %v, want NULL", oldRatio.Float64)
	}
	if !baseline.Valid || baseline.Float64 != 12.0/11 || !added.Valid || added.Float64 != 12.0/11 {
		t.Errorf("baseline ratio %v, added ratio %v, want 12/11 for both", baseline, added)
	}
	if removed.Valid {
		t.Errorf("a finding without a ratio was recorded as %v, want NULL", removed.Float64)
	}
}

func TestReadSnapshotRatioIsTheFirstLimitedClueOverItsTrigger(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs columbo")
	}
	resolved, err := filepath.EvalSymlinks(os.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", resolved)
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"go.mod": "module example.com/ratio\n\ngo 1.25.1\n",
		"ratio.go": `package ratio

func Twelve() int {
	a := 1
	a++
	a++
	a++
	a++
	a++
	a++
	a++
	a++
	a++
	a++
	return a
}

func ClumpA(a int, b string, c bool) {}
func ClumpB(a int, b string, c bool) {}
func ClumpC(a int, b string, c bool) {}
`,
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	snap, err := Env{Columbo: buildColumbo(t)}.Analyze(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]float64{
		"long-function": 12.0 / 11,
		"data-clump":    3.0 / 3,
	}
	for _, f := range snap.Findings {
		w, ok := want[f.Smell]
		if !ok {
			continue
		}
		delete(want, f.Smell)
		if f.Ratio == nil {
			t.Errorf("%s at %s has no ratio, want %v", f.Smell, f.Symbol, w)
		} else if *f.Ratio != w {
			t.Errorf("%s at %s: ratio %v, want %v", f.Smell, f.Symbol, *f.Ratio, w)
		}
	}
	for smell := range want {
		t.Errorf("no %s finding in %+v", smell, snap.Findings)
	}
}

func buildColumbo(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "columbo")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/columbo")
	cmd.Dir = filepath.Join("..", "..")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build columbo: %v\n%s", err, out)
	}
	return bin
}
