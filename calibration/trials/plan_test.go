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
