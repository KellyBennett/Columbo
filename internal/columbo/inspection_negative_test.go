package columbo

import "testing"

func TestInspectionNegativeIndependentPolicies(t *testing.T) {
	x := &inspectionNegativeIndependent{label: "a"}
	x.Visit()
	if x.visits != 1 || x.label != "a" {
		t.Fatal(x)
	}
	x.Rename("b")
	if x.visits != 1 || x.label != "b" {
		t.Fatal(x)
	}
}
func TestInspectionNegativeSubsetMembership(t *testing.T) {
	s := &inspectionNegativeSubset{selected: map[*inspectionNegativeEntry]bool{}}
	a, b := &inspectionNegativeEntry{Marked: true}, &inspectionNegativeEntry{}
	s.Append(a)
	s.Append(b)
	if len(s.all) != 2 || len(s.selected) != 1 || !s.Selected(a) || s.Selected(b) {
		t.Fatal(s)
	}
}
func TestInspectionNegativeCoherentList(t *testing.T) {
	l := &inspectionNegativeList{}
	l.Append(7)
	if l.Len() != 1 || l.At(0) != 7 {
		t.Fatal(l)
	}
}
func TestInspectionNegativeDifferentOwnership(t *testing.T) {
	s := &inspectionNegativeStore{values: []int{1}}
	a, b := s.Borrow(), s.Copy()
	a[0] = 2
	if s.values[0] != 2 || b[0] != 1 {
		t.Fatal(s, a, b)
	}
}
func TestInspectionNegativeFirstVersusCardinality(t *testing.T) {
	s := &inspectionNegativeSelection{values: []int{4, 9}}
	x, ok := s.First()
	_, one := s.ExactlyOne()
	if x != 4 || !ok || one {
		t.Fatal(x, ok, one)
	}
}
func TestInspectionNegativeGenericAliasEmbedding(t *testing.T) {
	c := &inspectionNegativeCell{Value: 3}
	c.Write(4)
	var a *inspectionNegativeAlias = c
	b := &inspectionNegativeBox[int]{value: 5}
	e := &inspectionNegativeEmbedded{inspectionNegativeCell: inspectionNegativeCell{Value: 6}, held: c}
	if a.AliasRead() != 4 || b.Read() != 5 || b.Copy() != 5 || e.Inspect() != 10 {
		t.Fatal("identity")
	}
}

type inspectionNegativeCell struct{ Value int }

func (c *inspectionNegativeCell) Read() int   { return c.Value }
func (c *inspectionNegativeCell) Write(v int) { c.Value = v }

type inspectionNegativeAlias = inspectionNegativeCell

func (c *inspectionNegativeAlias) AliasRead() int { return c.Value }

type inspectionNegativeBox[T any] struct{ value T }

func (b *inspectionNegativeBox[T]) Read() T { return b.value }
func (b inspectionNegativeBox[T]) Copy() T  { return b.value }

type inspectionNegativeEmbedded struct {
	inspectionNegativeCell
	held *inspectionNegativeCell
}

func (e *inspectionNegativeEmbedded) Inspect() int             { x := e.held; return x.Read() + e.Read() }
func inspectionNegativeFree(e *inspectionNegativeEmbedded) int { return e.Inspect() }

type inspectionNegativeIndependent struct {
	visits int
	label  string
}

func (x *inspectionNegativeIndependent) Visit()          { x.visits++ }
func (x *inspectionNegativeIndependent) Rename(s string) { x.label = s }

type inspectionNegativeEntry struct {
	Marked bool
	Value  int
}
type inspectionNegativeSubset struct {
	all      []*inspectionNegativeEntry
	selected map[*inspectionNegativeEntry]bool
}

func (s *inspectionNegativeSubset) Append(e *inspectionNegativeEntry) {
	s.all = append(s.all, e)
	if e.Marked {
		s.selected[e] = true
	}
}
func (s *inspectionNegativeSubset) Selected(e *inspectionNegativeEntry) bool { return s.selected[e] }

type inspectionNegativeList struct{ values []int }

func (l *inspectionNegativeList) Append(x int) { l.values = append(l.values, x) }
func (l *inspectionNegativeList) Len() int     { return len(l.values) }
func (l *inspectionNegativeList) At(i int) int { return l.values[i] }

type inspectionNegativeStore struct{ values []int }

func (s *inspectionNegativeStore) Borrow() []int { return s.values }
func (s *inspectionNegativeStore) Copy() []int   { return append([]int(nil), s.values...) }

type inspectionNegativeSelection struct{ values []int }

func (s *inspectionNegativeSelection) First() (int, bool) {
	if len(s.values) == 0 {
		return 0, false
	}
	return s.values[0], true
}
func (s *inspectionNegativeSelection) ExactlyOne() (int, bool) {
	if len(s.values) != 1 {
		return 0, false
	}
	return s.values[0], true
}
