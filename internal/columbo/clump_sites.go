package columbo

import (
	"go/ast"
	"go/types"
	"reflect"
	"slices"
	"sort"
	"strings"
	"unicode"
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
		if member, ok := newClumpMember(p, d.signature, d.parameterReceipt(p)); ok {
			members = append(members, member)
		}
	}
	return newClumpSite(d.declReceipt(), members)
}
func (s structDeclaration) clumpSite() clumpSite {
	members := []clumpMember{}
	index := 0
	for _, field := range s.body.Fields.List {
		members = append(members, s.fieldMembers(field, index)...)
		index += max(1, len(field.Names))
	}
	return newClumpSite(s.declReceipt(), members)
}
func (s structDeclaration) fieldMembers(field *ast.Field, first int) []clumpMember {
	members := []clumpMember{}
	for offset, name := range field.Names {
		p := s.fieldParameter(field, name, first+offset)
		if member, ok := newClumpMember(p, nil, s.fieldReceipt(p)); ok {
			members = append(members, member)
		}
	}
	return members
}
func newClumpMember(p parameter, signature *types.Signature, receipt Source) (clumpMember, bool) {
	key, ok := clumpKey(p.obj.Name(), p.typ, signature)
	return clumpMember{key: key, primitive: primitive(p.typ), receipt: receipt}, ok
}
func clumpKey(name string, t types.Type, signature *types.Signature) (string, bool) {
	canonical := canonicalType(t, signature)
	if name == "" || name == "_" || canonical == "context.Context" {
		return "", false
	}
	if interfaceType(t) {
		canonical = "interface"
	}
	return looseName(name) + " " + canonical, true
}
func interfaceType(t types.Type) bool {
	t = types.Unalias(t)
	if _, parameter := t.(*types.TypeParam); parameter {
		return false
	}
	return types.IsInterface(t)
}
func looseName(name string) string {
	return strings.ToLower(trimCamelCaseIDSuffix(name))
}

func trimCamelCaseIDSuffix(name string) string {
	stem, found := strings.CutSuffix(name, "ID")
	if !found {
		stem, found = strings.CutSuffix(name, "Id")
	}
	if !found || stem == "" {
		return name
	}
	if last := stem[len(stem)-1]; unicode.IsLower(rune(last)) || unicode.IsDigit(rune(last)) {
		return stem
	}
	return name
}

var primitiveContainers = map[reflect.Type]bool{
	reflect.TypeOf((*types.Pointer)(nil)): true,
	reflect.TypeOf((*types.Slice)(nil)):   true,
	reflect.TypeOf((*types.Array)(nil)):   true,
	reflect.TypeOf((*types.Map)(nil)):     true,
}

func primitive(t types.Type) bool {
	t = types.Unalias(t)
	if _, basic := t.(*types.Basic); basic {
		return true
	}
	return primitiveContainers[reflect.TypeOf(t)] && !slices.ContainsFunc(typeComponents(t), func(c types.Type) bool { return !primitive(c) })
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
func (s clumpSite) before(other clumpSite) bool {
	if s.receipt.File != other.receipt.File {
		return s.receipt.File < other.receipt.File
	}
	return s.receipt.StartOffset < other.receipt.StartOffset
}

type clumpSitesByPackage map[string][]clumpSite

func (p clumpSitesByPackage) add(f *file, site clumpSite) {
	if f.included {
		p[f.packagePath()] = append(p[f.packagePath()], site)
	}
}
func (a *engine) packageClumpSites() clumpSitesByPackage {
	out := clumpSitesByPackage{}
	for _, d := range a.declarations {
		out.add(d.file, d.clumpSite())
	}
	for _, f := range a.files {
		for _, s := range f.structDeclarations() {
			out.add(f, s.clumpSite())
		}
	}
	for _, sites := range out {
		sort.Slice(sites, func(i, j int) bool { return sites[i].before(sites[j]) })
	}
	return out
}
