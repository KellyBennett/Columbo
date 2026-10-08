package columbo

import (
	"fmt"
	"go/constant"
	"go/token"
)

func (s *overwriteScan) groups() []advisoryGroup {
	groups := []advisoryGroup{}
	for i, first := range s.candidates {
		for _, last := range s.candidates[i+1:] {
			groups = append(groups, s.pair(first, last)...)
		}
	}
	return groups
}
func (s *overwriteScan) pair(first, last overwriteCandidate) []advisoryGroup {
	if first.guard == last.guard || first.target != last.target || constant.Compare(first.value, token.EQL, last.value) {
		return nil
	}
	if witness := s.witness(first, last); witness != nil {
		return []advisoryGroup{s.group(first, last, witness)}
	}
	return nil
}
func (s *overwriteScan) group(first, last overwriteCandidate, w *overwriteWitness) advisoryGroup {
	subject := s.owner.symbol + ":" + first.target.Name()
	key := fmt.Sprintf("%d:%d", first.write.Pos()-s.owner.fn.Pos(), last.write.Pos()-s.owner.fn.Pos())
	id, _ := identity(overwriteKind, s.owner.file.rel, s.owner.symbol, key)
	group := advisoryGroup{id: id, kind: overwriteKind, subject: subject, lead: overwriteLead, limits: overwriteLimits}
	group.values = s.values(first, last, w)
	group.sites = []advisorySite{
		s.assignmentSite(first, "earlier complete assignment", "overwritten-assignment"),
		s.assignmentSite(last, "later complete assignment", "overriding-assignment"),
		{symbol: s.owner.symbol, representation: "evaluated prefix under witness", receipts: w.receipts},
	}
	return group
}
func (s *overwriteScan) assignmentSite(candidate overwriteCandidate, representation, kind string) advisorySite {
	return advisorySite{symbol: s.owner.symbol, representation: representation, receipts: []Source{
		s.owner.nodeSource("overlap-guard", candidate.guard.Cond, Detail{Subject: "true under witness"}),
		s.owner.nodeSource(kind, candidate.write, Detail{Subject: candidate.value.ExactString()}),
	}}
}
func (s *overwriteScan) values(first, last overwriteCandidate, w *overwriteWitness) []advisoryValue {
	values := []advisoryValue{{"earlier-result", first.target.Name(), first.value.ExactString()}, {"later-result", last.target.Name(), last.value.ExactString()}}
	for _, input := range s.inputs {
		values = append(values, advisoryValue{"witness-input", s.labels[input] + " (" + input.typ().String() + ")", w.inputs[input].ExactString()})
	}
	return values
}
