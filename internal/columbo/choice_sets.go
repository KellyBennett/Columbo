package columbo

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
)

type ChoiceSetReport struct {
	Version      int              `json:"version"`
	Files        int              `json:"files_analyzed"`
	Declarations int              `json:"declarations_analyzed"`
	Groups       []ChoiceSetGroup `json:"groups"`
}
type ChoiceSetGroup struct {
	ID         string           `json:"id"`
	Constants  []ChoiceConstant `json:"constants"`
	Collection string           `json:"collection_field"`
	Projection string           `json:"element_field"`
	Sites      []ChoiceSetSite  `json:"sites"`
	Lead       string           `json:"lead"`
	Limits     string           `json:"limits"`
}
type ChoiceConstant struct {
	Identity string `json:"identity"`
	Value    string `json:"value"`
}
type ChoiceSetSite struct {
	Symbol         string           `json:"symbol"`
	Representation string           `json:"representation"`
	Seeds          []ChoiceConstant `json:"seed_order"`
	Input          string           `json:"input_expression"`
	Receipts       []Source         `json:"receipts"`
}

const choiceSetLead = "These declarations independently seed collections with the same constants and add the same field projection. Review whether one domain value should own enumeration and membership. Preserve caller-specific payloads and validation; this does not establish a need for polymorphism."
const choiceSetLimits = "Evidence describes construction through the end of the recorded loop, not later mutations, runtime equality, or shared business meaning. Maps and slices retain different ordering, multiplicity, collision, and payload semantics."

func (a *engine) choiceSets() ChoiceSetReport {
	index := choiceSetIndex{groups: map[string]*ChoiceSetGroup{}}
	for _, f := range a.files {
		if f.included {
			index.report.Files++
		}
	}
	for _, d := range a.declarations {
		index.collect(d)
	}
	return index.finish()
}

type choiceSetIndex struct {
	groups map[string]*ChoiceSetGroup
	report ChoiceSetReport
}

func (i *choiceSetIndex) collect(d *declaration) {
	if !d.file.included || !d.hasBody() {
		return
	}
	i.report.Declarations++
	for _, construction := range d.choiceConstructions() {
		i.add(construction)
	}
}
func (i *choiceSetIndex) add(c *choiceConstruction) {
	key := c.key()
	if i.groups[key] == nil {
		i.groups[key] = c.group(key)
	}
	i.groups[key].Sites = append(i.groups[key].Sites, c.site())
}
func (i *choiceSetIndex) finish() ChoiceSetReport {
	i.report.Version, i.report.Groups = 1, []ChoiceSetGroup{}
	for _, g := range i.groups {
		if g.repeated() {
			i.report.Groups = append(i.report.Groups, *g)
		}
	}
	sort.Slice(i.report.Groups, func(a, b int) bool { return i.report.Groups[a].ID < i.report.Groups[b].ID })
	return i.report
}
func (g *ChoiceSetGroup) repeated() bool {
	owners := map[string]bool{}
	for _, s := range g.Sites {
		owners[s.Symbol] = true
	}
	sort.Slice(g.Sites, func(a, b int) bool { return sourceLess(g.Sites[a].Receipts[0], g.Sites[b].Receipts[0]) })
	return len(owners) >= 2
}
func choiceSetID(key string) string {
	sum := sha256.Sum256([]byte(key))
	return "CS-" + hex.EncodeToString(sum[:])
}
