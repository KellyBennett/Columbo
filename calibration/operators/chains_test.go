package operators_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/KellyBennett/Columbo/calibration/harness"
	"github.com/KellyBennett/Columbo/calibration/operators"
	"github.com/KellyBennett/Columbo/calibration/report"
)

const chainsDir = "../chains"

// savedChain is a shrunk invariant violation kept as a regression fixture under calibration/chains.
type savedChain struct {
	Fixture string `json:"fixture"`
	harness.ChainOutcome
}

func (s savedChain) operators() []string {
	out := make([]string, len(s.Steps))
	for i, st := range s.Steps {
		out[i] = st.Operator
	}
	return out
}

func chainAlphabet() []harness.Operator {
	var out []harness.Operator
	for _, op := range operators.All() {
		if op.Class() != harness.Cure {
			out = append(out, op)
		}
	}
	return out
}

var (
	chainersMu sync.Mutex
	chainers   = map[string]*harness.Chainer{}
)

// chainer prepares each fixture once per process; the fuzzer reuses it across inputs.
func chainer(t testing.TB, fixture string) *harness.Chainer {
	t.Helper()
	chainersMu.Lock()
	defer chainersMu.Unlock()
	if ch, ok := chainers[fixture]; ok {
		return ch
	}
	resolved, err := filepath.EvalSymlinks(os.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("TMPDIR", resolved); err != nil {
		t.Fatal(err)
	}
	src, _ := filepath.Abs(filepath.Join("..", "fixtures", fixture))
	ch := &harness.Chainer{Env: harness.Env{Columbo: columbo(t), Validity: harness.ValidityTest}, Source: src, Ops: chainAlphabet()}
	if err := ch.Prepare(); err != nil {
		t.Fatal(err)
	}
	chainers[fixture] = ch
	return ch
}

func savedChains(t testing.TB) map[string]savedChain {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(chainsDir, "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]savedChain{}
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		var s savedChain
		if err := json.Unmarshal(data, &s); err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		out[filepath.Base(p)] = s
	}
	return out
}

// knownViolation matches by edit sequence, not fixture, so one blind spot found on several fixtures is saved once.
// A plant-then-hide pair whose hide triage.yaml already lists as a systemic blind spot is documented there instead.
func knownViolation(t testing.TB, o harness.ChainOutcome) bool {
	got := savedChain{ChainOutcome: o}.operators()
	for _, s := range savedChains(t) {
		if s.Invariant == o.Invariant && slices.Equal(s.operators(), got) {
			return true
		}
	}
	if o.Invariant != harness.InjectNoticed || len(got) != 2 {
		return false
	}
	entries, err := report.LoadTriage("../triage.yaml")
	if err != nil {
		t.Fatal(err)
	}
	return slices.ContainsFunc(entries, func(e report.TriageEntry) bool {
		return e.Operator == got[1] && e.Symbol == "*" && e.Status == "blind-spot"
	})
}

func chainJSON(fixture string, o harness.ChainOutcome) []byte {
	data, _ := json.MarshalIndent(savedChain{Fixture: fixture, ChainOutcome: o}, "", "  ")
	return append(data, '\n')
}

// The seed wraps a known blind spot (an anonymous struct parameter hides a planted Data Clump) in neutral steps the shrinker must remove.
var seededChain = []harness.ChainStep{
	{Operator: "rename-decl", Symbol: "example.com/clump.route", Dose: 1},
	{Operator: "inline-parameter-object", Symbol: "example.com/clump.Address", Dose: 2},
	{Operator: "move-decl-to-file", Symbol: "example.com/clump.Ship", Dose: 1},
	{Operator: "pack-anonymous-struct", Symbol: "example.com/clump.Address", Dose: 1},
}

const seededChainFile = "clump-anonymous-struct-hides-clump.json"

func TestSeededChainSurvivorShrinksToTwoSteps(t *testing.T) {
	if testing.Short() {
		t.Skip("runs columbo on chained mutants")
	}
	ch := chainer(t, "clump")
	seeded := ch.Replay(seededChain)
	if seeded.Verdict != harness.Violated || seeded.Invariant != harness.InjectNoticed {
		t.Fatalf("seeded chain: %s %s (%s), want a violated %s", seeded.Verdict, seeded.Invariant, seeded.Reason, harness.InjectNoticed)
	}
	shrunk := ch.Shrink(seeded)
	if len(shrunk.Steps) > 2 {
		t.Fatalf("shrunk to %d steps, want at most 2: %+v", len(shrunk.Steps), shrunk.Steps)
	}
	path := filepath.Join(chainsDir, seededChainFile)
	got := chainJSON("clump", shrunk)
	if os.Getenv("CALIBRATION_WRITE_CHAINS") == "1" {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v; write it with CALIBRATION_WRITE_CHAINS=1", err)
	}
	if string(want) != string(got) {
		t.Errorf("shrunk chain changed; rerun with CALIBRATION_WRITE_CHAINS=1 if intended\nwant %s\ngot  %s", want, got)
	}
}

// When Columbo starts noticing a saved chain, this fails so the blind spot is retired on purpose, as with triage.yaml.
func TestSavedChainsStillViolate(t *testing.T) {
	if testing.Short() {
		t.Skip("runs columbo on chained mutants")
	}
	saved := savedChains(t)
	if len(saved) == 0 {
		t.Fatal("no saved chains")
	}
	for name, s := range saved {
		got := chainer(t, s.Fixture).Replay(s.Steps)
		if got.Verdict != harness.Violated || got.Invariant != s.Invariant {
			t.Errorf("%s: now %s %s (%s); Columbo notices it, so delete the fixture", name, got.Verdict, got.Invariant, got.Reason)
		}
	}
}

func TestNeutralChainKeepsCaseIDs(t *testing.T) {
	if testing.Short() {
		t.Skip("runs columbo on chained mutants")
	}
	got := chainer(t, "clump").Replay([]harness.ChainStep{
		{Operator: "rename-decl", Symbol: "example.com/clump.valid", Dose: 1},
		{Operator: "move-decl-to-file", Symbol: "example.com/clump.label", Dose: 1},
		{Operator: "rename-local", Symbol: "example.com/clump.Ship", Dose: 1},
	})
	if got.Verdict != harness.Holds || got.Invariant != harness.NeutralKeepsIDs {
		t.Fatalf("neutral chain: %s %s (%s)", got.Verdict, got.Invariant, got.Reason)
	}
}

var fuzzFixtures = []string{"clump", "inline", "envy", "deps", "neutral", "cosmetic", "variants"}

// FuzzChains selects a fixture with the first byte and up to four edits with the rest, then checks both chain invariants.
// A new violation is shrunk and printed for saving under calibration/chains; known ones pass.
func FuzzChains(f *testing.F) {
	seed := chainer(f, "clump").Replay(seededChain)
	f.Add(append([]byte{0}, seed.Choices...))
	f.Add([]byte{4, 1, 0, 0, 2, 0, 0, 3, 1, 0})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) < 4 {
			return
		}
		fixture := fuzzFixtures[int(data[0])%len(fuzzFixtures)]
		ch := chainer(t, fixture)
		got := ch.Walk(&harness.BytePicker{Data: data[1:], MaxSteps: 4})
		if got.Verdict != harness.Violated {
			return
		}
		shrunk := ch.Shrink(got)
		if knownViolation(t, shrunk) {
			return
		}
		name := fixture + "-" + strings.Join(savedChain{ChainOutcome: shrunk}.operators(), "-") + ".json"
		t.Errorf("new %s violation: %s\nsave as calibration/chains/%s:\n%s", shrunk.Invariant, shrunk.Reason, name, chainJSON(fixture, shrunk))
	})
}
