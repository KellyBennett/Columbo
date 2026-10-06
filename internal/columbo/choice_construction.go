package columbo

import (
	"go/ast"
	"go/token"
	"go/types"
	"sort"
)

type choiceConstruction struct {
	owner                                  *declaration
	facts                                  choiceFacts
	local                                  *types.Var
	initializer                            *ast.AssignStmt
	loop                                   *ast.RangeStmt
	constants                              []ChoiceConstant
	representation, collection, projection string
}

func (d *declaration) choiceConstructions() []*choiceConstruction {
	result := []*choiceConstruction{}
	for n := 1; n < len(d.fn.Body.List); n++ {
		c := &choiceConstruction{owner: d, facts: choiceFacts{d.file.typeInfo()}}
		if c.match(d.fn.Body.List[n-1], d.fn.Body.List[n]) {
			result = append(result, c)
		}
	}
	return result
}
func (c *choiceConstruction) match(before, after ast.Stmt) bool {
	c.initializer, _ = before.(*ast.AssignStmt)
	c.loop, _ = after.(*ast.RangeStmt)
	return c.initializer != nil && c.loop != nil && c.seed() && c.extend()
}
func (c *choiceConstruction) seed() bool {
	s := c.initializer
	if s.Tok != token.DEFINE || len(s.Lhs) != 1 || len(s.Rhs) != 1 {
		return false
	}
	c.local = c.facts.localDefinition(s.Lhs[0])
	literal, ok := unparen(s.Rhs[0]).(*ast.CompositeLit)
	return c.local != nil && ok && c.seedLiteral(literal)
}
func (c *choiceConstruction) seedLiteral(literal *ast.CompositeLit) bool {
	c.representation = choiceRepresentation(c.local.Type())
	return c.representation != "" && c.seedConstants(literal.Elts)
}
func (c *choiceConstruction) seedConstant(element ast.Expr) (ChoiceConstant, bool) {
	if c.representation == "map-keys" {
		element = c.facts.mapKey(element)
	}
	return choiceConstant(c.facts.object(element))
}
func (c *choiceConstruction) seedConstants(elements []ast.Expr) bool {
	for _, element := range elements {
		constant, ok := c.seedConstant(element)
		if !ok {
			return false
		}
		c.constants = append(c.constants, constant)
	}
	return len(c.constants) >= 2 && uniqueChoiceConstants(c.constants)
}
func uniqueChoiceConstants(constants []ChoiceConstant) bool {
	seen := map[string]bool{}
	for _, c := range constants {
		if seen[c.Value] {
			return false
		}
		seen[c.Value] = true
	}
	return true
}
func (c *choiceConstruction) sortedConstants() []ChoiceConstant {
	constants := append([]ChoiceConstant{}, c.constants...)
	sort.Slice(constants, func(a, b int) bool { return constants[a].Identity < constants[b].Identity })
	return constants
}
func (c *choiceConstruction) key() string {
	return canonical([]any{c.sortedConstants(), c.collection, c.projection})
}
func (c *choiceConstruction) group(key string) *ChoiceSetGroup {
	return &ChoiceSetGroup{ID: choiceSetID(key), Constants: c.sortedConstants(), Collection: c.collection, Projection: c.projection, Lead: choiceSetLead, Limits: choiceSetLimits}
}
func (c *choiceConstruction) site() ChoiceSetSite {
	return ChoiceSetSite{Symbol: c.owner.symbol, Representation: c.representation, Seeds: c.constants, Input: c.inputSpelling(), Receipts: c.receipts()}
}
func (c *choiceConstruction) receipt(kind string, node ast.Node) Source {
	r := c.owner.source(kind, node.Pos(), node.End(), Detail{Subject: c.owner.symbol})
	r.Spelling = c.spelling(node)
	return r
}
func (c *choiceConstruction) spelling(node ast.Node) string {
	f := c.owner.file
	return string(f.data[f.tf.Offset(node.Pos()):f.tf.Offset(node.End())])
}

func (c *choiceConstruction) receipts() []Source {
	return []Source{c.receipt("choice-seed", c.initializer), c.receipt("choice-projection", c.loop)}
}

func (c *choiceConstruction) inputSpelling() string { return c.spelling(c.loop.X) }
