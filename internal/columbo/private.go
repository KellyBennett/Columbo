package columbo

import (
	"fmt"
	"go/ast"
	"go/types"
	"sort"
	"strings"
)

// Private-type dispersion is deliberately separate from function dependency
// fan-out. Function metrics ignore package-private named types; this index
// records how broadly those types are spread through production files.
type privateTypeUsage struct {
	name          string
	path          string
	file          string
	line          int
	files         map[string]bool
	fieldAccesses int
}

type typeReferenceIndex struct {
	engine *engine
	usages map[string]*privateTypeUsage
}

type privateReferences struct {
	index *typeReferenceIndex
	file  *file
	info  *types.Info
}

func (a *engine) findPrivate() {
	index := &typeReferenceIndex{engine: a, usages: map[string]*privateTypeUsage{}}
	for _, f := range a.files {
		index.definitions(f)
	}
	for _, f := range a.files {
		index.references(f)
	}
	index.warn()
}

func (i *typeReferenceIndex) production(f *file) bool {
	return f.included && !strings.HasSuffix(f.rel, "_test.go")
}

func (i *typeReferenceIndex) definitions(f *file) {
	for _, obj := range f.typeInfo().Defs {
		i.definition(obj, f)
	}
}

func (i *typeReferenceIndex) definition(obj types.Object, f *file) {
	t, ok := obj.(*types.TypeName)
	if !ok || t.Exported() || t.IsAlias() || !i.production(f) {
		return
	}
	key := i.engine.typeKey(t)
	if i.usages[key] == nil {
		i.usages[key] = newPrivateTypeUsage(t, f)
	}
}

func newPrivateTypeUsage(t *types.TypeName, f *file) *privateTypeUsage {
	return &privateTypeUsage{name: f.packagePath() + "." + t.Name(), path: f.path, file: f.rel, line: f.line(t.Pos()), files: map[string]bool{f.rel: true}}
}

func (i *typeReferenceIndex) references(f *file) {
	if !i.production(f) {
		return
	}
	scan := &privateReferences{index: i, file: f, info: f.typeInfo()}
	ast.Inspect(f.ast, scan.visit)
}

func (s *privateReferences) visit(n ast.Node) bool {
	switch n := n.(type) {
	case *ast.Ident:
		s.index.reference(s.typeName(n), s.file)
	case *ast.SelectorExpr:
		s.index.fieldAccess(s.fieldReceiver(n), s.file)
	}
	return true
}

func (s *privateReferences) typeName(id *ast.Ident) *types.TypeName {
	obj, _ := s.info.Uses[id].(*types.TypeName)
	if obj == nil {
		return nil
	}
	named, _ := types.Unalias(obj.Type()).(*types.Named)
	if named == nil {
		return nil
	}
	return named.Obj()
}

func (s *privateReferences) fieldReceiver(selector *ast.SelectorExpr) *types.TypeName {
	selection := s.info.Selections[selector]
	if selection == nil || selection.Kind() != types.FieldVal {
		return nil
	}
	named, _ := stripPointer(selection.Recv()).(*types.Named)
	if named == nil {
		return nil
	}
	return named.Obj()
}

func (i *typeReferenceIndex) reference(obj *types.TypeName, f *file) {
	if usage := i.usage(obj); usage != nil {
		usage.files[f.rel] = true
	}
}

func (i *typeReferenceIndex) fieldAccess(obj *types.TypeName, f *file) {
	if usage := i.usage(obj); usage != nil && usage.path != f.path {
		usage.fieldAccesses++
	}
}

func (i *typeReferenceIndex) usage(obj *types.TypeName) *privateTypeUsage {
	if obj == nil || obj.Exported() {
		return nil
	}
	return i.usages[i.engine.typeKey(obj)]
}

func (i *typeReferenceIndex) warn() {
	for _, usage := range i.sortedUsages() {
		i.warnUsage(usage)
	}
}

func (i *typeReferenceIndex) sortedUsages() []*privateTypeUsage {
	usages := make([]*privateTypeUsage, 0, len(i.usages))
	for _, usage := range i.usages {
		usages = append(usages, usage)
	}
	sort.Slice(usages, func(a, b int) bool { return usageBefore(usages[a], usages[b]) })
	return usages
}

func usageBefore(a, b *privateTypeUsage) bool {
	if a.file != b.file {
		return a.file < b.file
	}
	if a.line != b.line {
		return a.line < b.line
	}
	return a.name < b.name
}

func (i *typeReferenceIndex) warnUsage(usage *privateTypeUsage) {
	files := sortedSet(usage.files)
	limit := int(i.engine.config.Counts["private-type-files"])
	if len(files) <= limit {
		return
	}
	i.engine.report.Warnings = append(i.engine.report.Warnings, usage.warning(files, limit))
}

func (usage *privateTypeUsage) warning(files []string, limit int) Warning {
	message := fmt.Sprintf("Private type %s spans %d production files (limit: %d): %s; %d cross-file direct field accesses.", usage.name, len(files), limit, strings.Join(files, ", "), usage.fieldAccesses)
	return Warning{Code: "private-type-dispersion", File: usage.file, Line: usage.line, Message: message}
}
