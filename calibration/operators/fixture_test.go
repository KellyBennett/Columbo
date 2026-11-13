package operators_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/KellyBennett/Columbo/calibration/harness"
	"github.com/KellyBennett/Columbo/calibration/operators"
)

var (
	columboOnce sync.Once
	columboBin  string
	columboErr  error
)

func columbo(t *testing.T) string {
	t.Helper()
	columboOnce.Do(func() {
		dir, err := os.MkdirTemp("", "calibration-columbo-")
		if err != nil {
			columboErr = err
			return
		}
		columboBin = filepath.Join(dir, "columbo")
		cmd := exec.Command("go", "build", "-o", columboBin, "./cmd/columbo")
		cmd.Dir = filepath.Join("..", "..")
		out, err := cmd.CombinedOutput()
		if err != nil {
			columboErr = err
			t.Log(string(out))
		}
	})
	if columboErr != nil {
		t.Fatalf("build columbo: %v", columboErr)
	}
	return columboBin
}

type row struct {
	Class, Operator, Symbol, Status, Reason string
	Dose                                    int
	Predicted                               *bool
}

func runFixture(t *testing.T, fixture string, ops ...string) []row {
	t.Helper()
	if testing.Short() {
		t.Skip("runs columbo and go test on mutants")
	}
	resolved, err := filepath.EvalSymlinks(os.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", resolved)
	selected, err := operators.Select(ops)
	if err != nil {
		t.Fatal(err)
	}
	store, err := harness.OpenStore(filepath.Join(t.TempDir(), "calibration.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	src, _ := filepath.Abs(filepath.Join("..", "fixtures", fixture))
	r := &harness.Runner{
		Env:     harness.Env{Columbo: columbo(t), Validity: harness.ValidityTest},
		Store:   store,
		Info:    harness.RunInfo{Module: fixture, Version: "fixture", Validity: harness.ValidityTest},
		Source:  src,
		Ops:     selected,
		Workers: 4,
		Log:     t.Logf,
	}
	if err := r.Prepare(); err != nil {
		t.Fatal(err)
	}
	defer r.Cleanup()
	if err := r.Run(); err != nil {
		t.Fatal(err)
	}
	rows, err := store.DB().Query(`SELECT class, operator, site_symbol, dose, status, reason, predicted_kill FROM mutants ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []row
	for rows.Next() {
		var r row
		var predicted *int
		if err := rows.Scan(&r.Class, &r.Operator, &r.Symbol, &r.Dose, &r.Status, &r.Reason, &predicted); err != nil {
			t.Fatal(err)
		}
		if predicted != nil {
			p := *predicted == 1
			r.Predicted = &p
		}
		out = append(out, r)
	}
	return out
}

func TestInlineKillsAtPredictedDoseAndSplitIsCosmetic(t *testing.T) {
	rows := runFixture(t, "inline", "inline-function", "split-forwarding-helpers")
	firstKill := 0
	evades := 0
	for _, r := range rows {
		switch r.Class {
		case "inject":
			if r.Predicted == nil || *r.Predicted != (r.Status == "killed") {
				t.Errorf("dose %d: status %s disagrees with prediction (%s)", r.Dose, r.Status, r.Reason)
			}
			if r.Status == "killed" && firstKill == 0 {
				firstKill = r.Dose
			}
		case "evade":
			evades++
			if r.Status != "killed" {
				t.Errorf("split dose %d survived: %s", r.Dose, r.Reason)
			}
		}
	}
	if firstKill != 3 {
		t.Errorf("long-function first fired at dose %d, want 3", firstKill)
	}
	if evades == 0 {
		t.Error("no split evade ran on a killed inline mutant")
	}
}

func TestExtractFunctionCuresLongFunction(t *testing.T) {
	rows := runFixture(t, "extract", "extract-function")
	if len(rows) == 0 {
		t.Fatal("no cure mutants ran")
	}
	for _, r := range rows {
		if r.Status != "killed" {
			t.Errorf("dose %d: %s (%s)", r.Dose, r.Status, r.Reason)
		}
	}
}

// The block-isolation survivor is a known blind spot (see triage.yaml). When Columbo starts
// seeing through bare blocks this test fails and the triage entry should be removed.
func TestSplitIsCosmeticButBlockIsolationEvades(t *testing.T) {
	rows := runFixture(t, "cosmetic", "split-long-function", "isolate-helper-calls")
	var injects, evades int
	for _, r := range rows {
		switch r.Class {
		case "inject":
			injects++
			if r.Status != "killed" {
				t.Errorf("split dose %d: %s (%s)", r.Dose, r.Status, r.Reason)
			}
		case "evade":
			evades++
			if r.Status != "survived" {
				t.Errorf("block isolation is now %s; remove the blind-spot triage entry", r.Status)
			}
		}
	}
	if injects == 0 || evades == 0 {
		t.Fatalf("injects=%d evades=%d", injects, evades)
	}
}
func TestNeutralOperatorsKeepOrPredictEveryCaseID(t *testing.T) {
	ops := []string{"reorder-decls", "move-decl-to-file", "rename-local", "rename-decl"}
	rows := runFixture(t, "neutral", ops...)
	ran, remapped := map[string]int{}, map[string]int{}
		ran[r.Operator]++
		if r.Status != "equivalent" {
			t.Errorf("%s at %s: %s: %s", r.Operator, r.Symbol, r.Status, r.Reason)
		if strings.Contains(r.Reason, "re-identified as SPEC identity predicts") {
			remapped[r.Operator]++
	for _, op := range ops {
		if ran[op] == 0 {
			t.Errorf("%s found no site in the fixture", op)
	for _, op := range []string{"reorder-decls", "move-decl-to-file", "rename-decl"} {
		if remapped[op] == 0 {
			t.Errorf("%s never moved a case ID, so the fixture does not exercise its mapping", op)
