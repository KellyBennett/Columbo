package columbo

import "slices"

type variantRoleMapping struct {
	players  map[string]string
	repeated []string
}

func (a *engine) variantRoles(c *Case, group variantGroup) {
	mapping := variantRoleMapping{players: map[string]string{}, repeated: group.repeated}
	combined := newRoleGroup()
	for _, site := range group.sites {
		observed := mapping.site(site)
		if observed == nil {
			return
		}
		combined.include(observed)
	}
	if len(mapping.players) == len(group.repeated) {
		a.recordRole(combined, c)
	}
}
func (m *variantRoleMapping) site(site variantSite) *roleGroup {
	result := newRoleGroup()
	for _, arm := range selectionRoleArms(site.node) {
		observed := m.arm(site, arm)
		if observed == nil {
			return nil
		}
		result.include(observed)
	}
	if !result.restrictSurface() {
		return nil
	}
	result.receipts = append(result.receipts, site.decision)
	return result
}
func (m *variantRoleMapping) include(variants []string, observed *roleGroup) bool {
	if len(observed.players) != 1 {
		return false
	}
	for key := range observed.players {
		for _, variant := range variants {
			if !m.bind(variant, key) {
				return false
			}
		}
	}
	return true
}
func (m *variantRoleMapping) bind(variant, player string) bool {
	if old, ok := m.players[variant]; ok && old != player {
		return false
	}
	m.players[variant] = player
	return true
}
func (site variantSite) roleArmKeys(arm roleArm, repeated []string) []string {
	var keys []string
	for _, key := range repeated {
		if site.roleArmMatches(arm, key) {
			keys = append(keys, key)
		}
	}
	return keys
}
func (site variantSite) roleArmMatches(arm roleArm, key string) bool {
	for _, expr := range arm.labels {
		offset := site.owner.file.tf.Offset(expr.Pos())
		for _, receipt := range site.arms[key] {
			if receipt.StartOffset == offset {
				return true
			}
		}
	}
	return false
}
func (g *roleGroup) restrictSurface() bool {
	surface := g.surface()
	if len(g.players) < 2 || len(surface) == 0 {
		return false
	}
	for _, player := range g.players {
		player.restrictSurface(surface)
	}
	return true
}
func (p *rolePlayer) restrictSurface(surface []string) {
	for message := range p.messages {
		if !slices.Contains(surface, message) {
			delete(p.messages, message)
		}
	}
}
func (m *variantRoleMapping) arm(site variantSite, arm roleArm) *roleGroup {
	keys := site.roleArmKeys(arm, m.repeated)
	if len(keys) == 0 {
		return newRoleGroup()
	}
	observed := observedRoleArm(site.owner, arm)
	if !m.include(keys, observed) {
		return nil
	}
	return observed
}
