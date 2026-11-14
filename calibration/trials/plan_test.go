package trials

import (
	"fmt"
	"testing"

	"github.com/KellyBennett/Columbo/calibration/harness"
)

func plant(smell, corpus, symbol string, dose int) *Plant {
	return &Plant{Smell: smell, Operator: "op", Corpus: &harness.Corpus{Module: corpus}, Site: harness.Site{Symbol: symbol}, Dose: dose}
}

func names(ts []Trial) []string {
	var out []string
	for _, t := range ts {
		out = append(out, fmt.Sprintf("%s.%s.d%d.r%d", t.Plant.Corpus.Module, t.Plant.Site.Symbol, t.Plant.Dose, t.Repeat))
	}
	return out
}

func TestScheduleTakesEveryCorpusBeforeASecondSiteOrDose(t *testing.T) {
	plants := []*Plant{
		plant("s", "a", "f", 1), plant("s", "a", "f", 2), plant("s", "a", "g", 1),
		plant("s", "b", "h", 1),
	}
	got := names(Schedule(plants, 5))
	want := []string{"a.f.d1.r0", "b.h.d1.r0", "a.g.d1.r0", "a.f.d2.r0", "a.f.d1.r1"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("schedule %v, want %v", got, want)
	}
}

func TestScheduleRepeatsASingleMutantToReachTheTrialCount(t *testing.T) {
	got := names(Schedule([]*Plant{plant("s", "a", "f", 1), plant("t", "b", "g", 1)}, 3))
	want := []string{"a.f.d1.r0", "a.f.d1.r1", "a.f.d1.r2", "b.g.d1.r0", "b.g.d1.r1", "b.g.d1.r2"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("schedule %v, want %v", got, want)
	}
}

var complexityRow = PlanRow{Smell: "high-cognitive-complexity", Operator: "inline-function-complexity", Corpus: "fixtures/inline"}

func TestASmellThatPlantsNothingIsNamed(t *testing.T) {
	pl := &Planter{Env: fixtureEnv(t), PlanDir: "..", Workers: 4}
	plants, unplanted, err := pl.PlantPlan([]PlanRow{inlineRow, complexityRow}, 5, 0)
	t.Cleanup(func() { pl.Cleanup(plants) })
	if err != nil {
		t.Fatal(err)
	}
	if len(plants) == 0 || fmt.Sprint(unplanted) != "[high-cognitive-complexity]" {
		t.Fatalf("%d plants, unplanted %v; want long-function planted and high-cognitive-complexity named", len(plants), unplanted)
	}
}

func TestALimitStopsPlantingOnceTheScheduleIsFull(t *testing.T) {
	pl := &Planter{Env: fixtureEnv(t), PlanDir: "..", Workers: 4}
	plants, unplanted, err := pl.PlantPlan([]PlanRow{inlineRow, complexityRow}, 1, 1)
	t.Cleanup(func() { pl.Cleanup(plants) })
	if err != nil {
		t.Fatal(err)
	}
	if len(unplanted) != 0 || len(plants) == 0 {
		t.Fatalf("%d plants, unplanted %v; want only long-function probed", len(plants), unplanted)
	}
	for _, p := range plants {
		if p.Smell != inlineRow.Smell {
			t.Fatalf("planted %s past a limit the first smell already filled", p.Smell)
		}
	}
}
