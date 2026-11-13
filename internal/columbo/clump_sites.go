package columbo

import (
	"go/types"
	"sort"
	"strings"
)

type clumpMember struct {
	key       string
	primitive bool
	receipt   Source
}

type clumpSite struct {
	receipt   Source
	members   []clumpMember
	signature parameterMultiset
}

func newClumpSite(receipt Source, members []clumpMember) clumpSite {
	signature := parameterMultiset{}
	for _, member := range members {
		signature[member.key]++
	}
	return clumpSite{receipt: receipt, members: members, signature: signature}
}
func (d *declaration) clumpSite() clumpSite {
	members := []clumpMember{}
	for _, p := range d.params {
		if key, ok := clumpKey(p.obj.Name(), canonicalType(p.typ, d.signature)); ok {
			members = append(members, clumpMember{key: key, primitive: primitive(p.typ), receipt: d.parameterReceipt(p)})
		}
	}
	return newClumpSite(d.declReceipt(), members)
}
func clumpKey(name, canonical string) (string, bool) {
	if name == "" || name == "_" || canonical == "context.Context" {
		return "", false
	}
	return looseName(name) + " " + canonical, true
}
func looseName(name string) string {
	lower := strings.ToLower(name)
	if trimmed := strings.TrimSuffix(lower, "id"); trimmed != "" {
		return trimmed
	}
	return lower
}
func primitive(t types.Type) bool {
	switch t := types.Unalias(t).(type) {
	case *types.Basic:
		return true
	case *types.Pointer:
		return primitive(t.Elem())
	case *types.Slice:
		return primitive(t.Elem())
	case *types.Array:
		return primitive(t.Elem())
	case *types.Map:
		return primitive(t.Key()) && primitive(t.Elem())
	}
	return false
}
func (s clumpSite) matched(pattern parameterMultiset) []clumpMember {
	need := pattern.copy()
	out := []clumpMember{}
	for _, member := range s.members {
		if need[member.key] > 0 {
			need[member.key]--
			out = append(out, member)
		}
	}
	return out
}
func (s clumpSite) ref() DeclarationRef { return *s.receipt.Declaration }
func (s clumpSite) before(other clumpSite) bool {
	if s.receipt.File != other.receipt.File {
		return s.receipt.File < other.receipt.File
	}
	return s.receipt.StartOffset < other.receipt.StartOffset
}

func (a *engine) packageClumpSites() map[string][]clumpSite {
	out := map[string][]clumpSite{}
	for _, d := range a.declarations {
		if d.file.included {
			out[d.file.packagePath()] = append(out[d.file.packagePath()], d.clumpSite())
		}
	}
	for _, sites := range out {
		sort.Slice(sites, func(i, j int) bool { return sites[i].before(sites[j]) })
	}
	return out
}
