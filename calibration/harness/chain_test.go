package harness

import (
	"slices"
	"testing"
)

type fakeOp struct {
	id     string
	class  Class
	target string
}

func (o fakeOp) ID() string                                { return o.id }
func (o fakeOp) Class() Class                              { return o.class }
func (o fakeOp) Target() string                            { return o.target }
func (o fakeOp) Sites(*Corpus) []Site                      { return nil }
func (o fakeOp) Doses(*Corpus, Site) []int                 { return nil }
func (o fakeOp) Apply(*Corpus, Site, int) (Applied, error) { return Applied{}, nil }

func TestTrackComposesRenamesMovesAndTouchedAcrossSteps(t *testing.T) {
	kill := true
	tr := newTrack()
	tr.add(fakeOp{"rename", Neutral, ""}, Expectation{Renames: map[string]string{"p.a": "p.b"}})
	tr.add(fakeOp{"inject", Inject, "long-function"}, Expectation{Touched: []string{"p.b"}, Prediction: Prediction{Kill: &kill}})
	tr.add(fakeOp{"rename", Neutral, ""}, Expectation{Renames: map[string]string{"p.b": "p.c"}})
	tr.add(fakeOp{"move", Neutral, ""}, Expectation{Moves: map[string]string{"p.c": "other.go"}})
	if got := tr.renames["p.a"]; got != "p.c" || len(tr.renames) != 1 {
		t.Errorf("renames = %v, want p.a -> p.c only", tr.renames)
	}
	if got := tr.moves["p.a"]; got != "other.go" {
		t.Errorf("moves = %v, want the original symbol p.a moved to other.go", tr.moves)
	}
	if !slices.Equal(tr.touched, []string{"p.c"}) {
		t.Errorf("touched = %v, want the inject's site under its final name", tr.touched)
	}
	if tr.invariant() != InjectNoticed || !slices.Equal(tr.smells, []string{"long-function"}) {
		t.Errorf("invariant %q smells %v", tr.invariant(), tr.smells)
	}
}

func TestTrackOnlyHoldsAnInjectToTheInvariantWhenItsPredictedToFire(t *testing.T) {
	quiet := false
	tr := newTrack()
	tr.add(fakeOp{"inject", Inject, "long-function"}, Expectation{Prediction: Prediction{Kill: &quiet}})
	tr.add(fakeOp{"evade", Evade, "long-function"}, Expectation{})
	if inv := tr.invariant(); inv != "" {
		t.Errorf("invariant = %q, want none for an inject below its limit", inv)
	}
	neutral := newTrack()
	neutral.add(fakeOp{"rename", Neutral, ""}, Expectation{})
	if inv := neutral.invariant(); inv != NeutralKeepsIDs {
		t.Errorf("neutral-only invariant = %q", inv)
	}
}
