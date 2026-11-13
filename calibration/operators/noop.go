package operators

import "github.com/KellyBennett/Columbo/calibration/harness"

type noop struct{}

func (noop) ID() string                                     { return "noop" }
func (noop) Class() harness.Class                           { return harness.Neutral }
func (noop) Target() string                                 { return "" }
func (noop) Doses(*harness.Corpus, harness.Site) []int      { return []int{0} }
func (noop) Sites(c *harness.Corpus) []harness.Site         { return []harness.Site{{Package: c.Module, Symbol: c.Module}} }
func (noop) Apply(*harness.Corpus, harness.Site, int) (harness.Applied, error) {
	return harness.Applied{Edit: harness.Edit{}}, nil
}
