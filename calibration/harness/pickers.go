package harness

import "fmt"

// BytePicker decodes fuzz input three bytes per step: operator, site, and dose, each taken modulo what the current code offers.
type BytePicker struct {
	Data     []byte
	MaxSteps int
	taken    int
}

func (p *BytePicker) Next(ops []Operator, c *Corpus) (Pick, bool, error) {
	for p.taken < p.MaxSteps && len(p.Data) >= 3 {
		b := p.Data[:3]
		p.Data = p.Data[3:]
		if pick, ok := choose(ops, c, b); ok {
			p.taken++
			return pick, true, nil
		}
	}
	return Pick{}, false, nil
}

func choose(ops []Operator, c *Corpus, b []byte) (Pick, bool) {
	if len(ops) == 0 {
		return Pick{}, false
	}
	op := ops[int(b[0])%len(ops)]
	sites := op.Sites(c)
	if len(sites) == 0 {
		return Pick{}, false
	}
	site := sites[int(b[1])%len(sites)]
	doses := op.Doses(c, site)
	if len(doses) == 0 {
		return Pick{}, false
	}
	return Pick{Op: op, Site: site, Dose: doses[int(b[2])%len(doses)], Choices: b}, true
}

// recordedPicker replays a saved chain, finding each step's site again by symbol.
type recordedPicker struct {
	steps []ChainStep
	next  int
}

func (p *recordedPicker) Next(ops []Operator, c *Corpus) (Pick, bool, error) {
	if p.next == len(p.steps) {
		return Pick{}, false, nil
	}
	step := p.steps[p.next]
	p.next++
	opIndex := indexOf(ops, func(o Operator) bool { return o.ID() == step.Operator })
	if opIndex < 0 {
		return Pick{}, false, fmt.Errorf("operator %s is not in the alphabet", step.Operator)
	}
	op := ops[opIndex]
	sites := op.Sites(c)
	siteIndex := indexOf(sites, func(s Site) bool { return s.Symbol == step.Symbol })
	if siteIndex < 0 {
		return Pick{}, false, fmt.Errorf("%s has no site %s", step.Operator, step.Symbol)
	}
	doses := op.Doses(c, sites[siteIndex])
	doseIndex := indexOf(doses, func(d int) bool { return d == step.Dose })
	if doseIndex < 0 {
		return Pick{}, false, fmt.Errorf("%s on %s has no dose %d", step.Operator, step.Symbol, step.Dose)
	}
	return Pick{Op: op, Site: sites[siteIndex], Dose: step.Dose, Choices: []byte{byte(opIndex), byte(siteIndex), byte(doseIndex)}}, true, nil
}

func indexOf[T any](xs []T, match func(T) bool) int {
	for i, x := range xs {
		if match(x) {
			return i
		}
	}
	return -1
}
