package columbo

import (
	"crypto/sha256"
	"fmt"
	"go/types"
	"maps"
	"slices"
	"strings"
)

// RoleCandidate is evidence, never an independently enforceable case.
type RoleCandidate struct {
	ID                   string
	Implementations      []string
	Messages             []string
	Interface            string
	Confidence           string
	Classification       string
	ExactInterfaces      []string
	CompatibleInterfaces []string
	UsedInterfaces       []string
	Receipts             []Source
	CaseIDs              []string
}

type rolePlayer struct {
	typ      types.Type
	messages map[string]*types.Func
	receipts []Source
}
type roleGroup struct {
	players  map[string]*rolePlayer
	roles    map[string]types.Type
	receipts []Source
}

func newRoleGroup() *roleGroup {
	return &roleGroup{players: map[string]*rolePlayer{}, roles: map[string]types.Type{}}
}
func (g *roleGroup) observe(t types.Type, method *types.Func, receipt Source) {
	if roleImplementation(t) == "" || !roleSupports(t, method) {
		return
	}
	key := roleImplementation(t)
	player := g.players[key]
	if player == nil {
		player = &rolePlayer{typ: t, messages: map[string]*types.Func{}}
		g.players[key] = player
	}
	player.messages[selectedMessage(method)] = method
	player.receipts = append(player.receipts, receipt)
}
func roleSupports(t types.Type, method *types.Func) bool {
	selection := roleMethod(t, method)
	return selection != nil && types.Identical(selection.Obj().Type(), method.Type())
}
func (g *roleGroup) surface() []string {
	var common []string
	for _, key := range slices.Sorted(maps.Keys(g.players)) {
		messages := g.players[key].messages
		if common == nil {
			common = slices.Sorted(maps.Keys(messages))
			continue
		}
		common = slices.DeleteFunc(common, func(message string) bool { return messages[message] == nil })
	}
	return common
}
func (a *engine) recordRole(g *roleGroup, c *Case) {
	role := g.candidate()
	if role == nil {
		return
	}
	role.matchInterfaces(g, a.interfaces)
	if c != nil {
		role.linkCase(c)
	}
	a.report.Roles = append(a.report.Roles, *role)
}
func (g *roleGroup) candidate() *RoleCandidate {
	messages := g.surface()
	if len(g.players) < 2 || len(messages) == 0 {
		return nil
	}
	implementations := slices.Sorted(maps.Keys(g.players))
	role := newRoleCandidate(implementations, messages)
	role.Receipts = append(role.Receipts, g.receipts...)
	for _, key := range implementations {
		role.Receipts = append(role.Receipts, g.players[key].receipts...)
	}
	return role
}
func newRoleCandidate(implementations, messages []string) *RoleCandidate {
	digest := sha256.Sum256([]byte(canonical([]any{implementations, messages})))
	return &RoleCandidate{ID: fmt.Sprintf("R-%x", digest), Implementations: implementations, Messages: messages,
		Interface: "interface { " + strings.Join(messages, "; ") + " }", Confidence: "strong", Classification: "inferred role"}
}
func (r *RoleCandidate) linkCase(c *Case) {
	r.CaseIDs = append(r.CaseIDs, c.ID)
	appendSources(c, r.Receipts)
	c.Clues = append(c.Clues, metric("common-role", r.ID, r.Messages).supportedBy(r.Receipts))
	lead := "The selected implementations already respond to the same required messages. Consider expressing that collaboration as a role owned by the consumer."
	if !slices.Contains(c.Leads, lead) {
		c.Leads = append(c.Leads, lead)
	}
}
func namedRoleInterfaces(universe []types.Type) map[string]types.Type {
	candidates := map[string]types.Type{}
	for _, t := range universe {
		if _, named := types.Unalias(t).(*types.Named); named {
			candidates[canonicalType(t, nil)] = t
		}
	}
	return candidates
}
func (r *RoleCandidate) matchInterfaces(g *roleGroup, universe []types.Type) {
	candidates := namedRoleInterfaces(universe)
	maps.Copy(candidates, g.roles)
	for _, key := range slices.Sorted(maps.Keys(candidates)) {
		r.matchInterface(key, candidates[key], g)
	}
	r.classify()
}
func (r *RoleCandidate) matchInterface(key string, t types.Type, g *roleGroup) {
	iface, ok := t.Underlying().(*types.Interface)
	if !ok || !iface.IsMethodSet() || containsTypeParameter(t) {
		return
	}
	if !r.matchesSurface(iface) || !g.implements(iface) {
		return
	}
	r.recordInterface(key, iface.NumMethods(), g)
}
func (r *RoleCandidate) recordInterface(key string, methodCount int, g *roleGroup) {
	if methodCount == len(r.Messages) {
		r.ExactInterfaces = append(r.ExactInterfaces, key)
	} else {
		r.CompatibleInterfaces = append(r.CompatibleInterfaces, key)
	}
	if _, used := g.roles[key]; used {
		r.UsedInterfaces = append(r.UsedInterfaces, key)
	}
}
func (r *RoleCandidate) matchesSurface(iface *types.Interface) bool {
	messages := map[string]bool{}
	for i := 0; i < iface.NumMethods(); i++ {
		messages[selectedMessage(iface.Method(i))] = true
	}
	for _, message := range r.Messages {
		if !messages[message] {
			return false
		}
	}
	return true
}
func (g *roleGroup) implements(iface *types.Interface) bool {
	for _, player := range g.players {
		if !types.Implements(player.typ, iface) {
			return false
		}
	}
	return true
}
func (r *Report) finishRoles() {
	merged := mergeRoleCandidates(r.Roles)
	r.Roles = nil
	for _, key := range slices.Sorted(maps.Keys(merged)) {
		role := merged[key]
		role.normalize()
		r.Roles = append(r.Roles, *role)
	}
}
func mergeRoleCandidates(roles []RoleCandidate) map[string]*RoleCandidate {
	merged := map[string]*RoleCandidate{}
	for _, role := range roles {
		if prior := merged[role.ID]; prior != nil {
			prior.merge(role)
		} else {
			copy := role
			merged[role.ID] = &copy
		}
	}
	return merged
}
func (r *RoleCandidate) merge(other RoleCandidate) {
	r.Receipts = append(r.Receipts, other.Receipts...)
	r.CaseIDs = append(r.CaseIDs, other.CaseIDs...)
	r.ExactInterfaces = append(r.ExactInterfaces, other.ExactInterfaces...)
	r.CompatibleInterfaces = append(r.CompatibleInterfaces, other.CompatibleInterfaces...)
	r.UsedInterfaces = append(r.UsedInterfaces, other.UsedInterfaces...)
}
func (r *RoleCandidate) normalize() {
	for _, list := range []*[]string{&r.CaseIDs, &r.ExactInterfaces, &r.CompatibleInterfaces, &r.UsedInterfaces} {
		slices.Sort(*list)
		*list = slices.Compact(*list)
	}
	slices.SortFunc(r.Receipts, compareRoleSources)
	r.Receipts = slices.CompactFunc(r.Receipts, equalRoleSources)
	r.classify()
}
func compareRoleSources(a, b Source) int {
	if sourceLess(a, b) {
		return -1
	}
	if sourceLess(b, a) {
		return 1
	}
	return 0
}
func equalRoleSources(a, b Source) bool { return compareRoleSources(a, b) == 0 }
func (r *RoleCandidate) classify() {
	if len(r.ExactInterfaces) > 0 || len(r.UsedInterfaces) > 0 {
		r.Classification = "existing role"
	}
}
func roleMethod(t types.Type, method *types.Func) *types.Selection {
	return types.NewMethodSet(t).Lookup(method.Pkg(), method.Name())
}
func (selection evidenceSelection) addRoles(roles []RoleCandidate) {
	for _, role := range roles {
		role.selectDeclarations(selection)
	}
}
func (r RoleCandidate) selectDeclarations(selection evidenceSelection) {
	for _, receipt := range r.Receipts {
		if receipt.Declaration != nil {
			selection[*receipt.Declaration] = true
		}
	}
}

// Unlike selection counting, role identity preserves pointer/value distinctions.
func roleImplementation(t types.Type) string {
	if selectedImplementation(t) == "" {
		return ""
	}
	return canonicalType(t, nil)
}
