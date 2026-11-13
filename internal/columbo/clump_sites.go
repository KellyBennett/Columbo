package columbo

import "sort"

type clumpMember struct {
	key     string
	receipt Source
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
			members = append(members, clumpMember{key: key, receipt: d.parameterReceipt(p)})
		}
	}
	return newClumpSite(d.declReceipt(), members)
}
func clumpKey(name, canonical string) (string, bool) {
	if name == "" || name == "_" || canonical == "context.Context" {
		return "", false
	}
	return name + " " + canonical, true
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
