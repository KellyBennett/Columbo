package operators

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"path/filepath"
	"strconv"
	"unicode"

	"github.com/KellyBennett/Columbo/calibration/harness"
	"golang.org/x/tools/go/packages"
)

// renameLocal renames the first local variable a function declares, at every use.
type renameLocal struct{}

func (renameLocal) ID() string                                { return "rename-local" }
func (renameLocal) Class() harness.Class                      { return harness.Neutral }
func (renameLocal) Target() string                            { return "" }
func (renameLocal) Doses(*harness.Corpus, harness.Site) []int { return []int{1} }

func (renameLocal) Sites(c *harness.Corpus) []harness.Site {
	var out []harness.Site
	for _, f := range funcs(c) {
		if id := firstLocal(f); id != nil {
			site := f.site()
			site.Offset = f.offset(id.Pos())
			out = append(out, site)
		}
	}
	return out
}

func (renameLocal) Apply(c *harness.Corpus, site harness.Site, _ int) (harness.Applied, error) {
	f, ok := findFn(c, site.Symbol)
	if !ok {
		return harness.Applied{}, errors.New("site vanished")
	}
	id := firstLocal(f)
	if id == nil || f.offset(id.Pos()) != site.Offset {
		return harness.Applied{}, errors.New("local vanished")
	}
	taken := identNames(f.file)
	edit, err := renameObject([]unit{f.unit}, f.pkg.TypesInfo.Defs[id], fresh(id.Name, taken))
	if err != nil {
		return harness.Applied{}, err
	}
	return harness.Applied{Edit: edit, Expectation: harness.Expectation{Touched: []string{f.symbol}}}, nil
}

func firstLocal(f fn) *ast.Ident {
	if f.decl.Body == nil {
		return nil
	}
	var first *ast.Ident
	ast.Inspect(f.decl.Body, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && first == nil && id.Name != "_" {
			if v, ok := f.pkg.TypesInfo.Defs[id].(*types.Var); ok && !v.IsField() {
				first = id
			}
		}
		return first == nil
	})
	return first
}

// renameDecl renames one unexported function or method and every reference in its package,
// in-package tests included. It skips a name that a file outside that type-checked set
// mentions, such as another platform's file, and a method name any interface in the package
// declares, because renaming either could change what compiles or what satisfies an
// interface.
type renameDecl struct{}

func (renameDecl) ID() string                                { return "rename-decl" }
func (renameDecl) Class() harness.Class                      { return harness.Neutral }
func (renameDecl) Target() string                            { return "" }
func (renameDecl) Doses(*harness.Corpus, harness.Site) []int { return []int{1} }

func (renameDecl) Sites(c *harness.Corpus) []harness.Site {
	scopes := map[*packages.Package]*packageNames{}
	var out []harness.Site
	for _, f := range funcs(c) {
		names, ok := scopes[f.pkg]
		if !ok {
			names = scopeOf(c, f.pkg)
			scopes[f.pkg] = names
		}
		if names != nil && renamable(f, *names) {
			out = append(out, f.site())
		}
	}
	return out
}

func (renameDecl) Apply(c *harness.Corpus, site harness.Site, _ int) (harness.Applied, error) {
	f, ok := findFn(c, site.Symbol)
	if !ok {
		return harness.Applied{}, errors.New("site vanished")
	}
	names := scopeOf(c, f.pkg)
	if names == nil || !renamable(f, *names) {
		return harness.Applied{}, errors.New("no longer renamable")
	}
	target := names.objectAt(f.path, f.offset(f.decl.Name.Pos()))
	name := fresh(f.decl.Name.Name, names.all)
	edit, err := renameObject(packageUnits(c.Root, names.pkg), target, name)
	if err != nil {
		return harness.Applied{}, err
	}
	renamed := symbolOf(f.pkg.PkgPath, &ast.FuncDecl{Recv: f.decl.Recv, Name: ast.NewIdent(name)})
	return harness.Applied{Edit: edit, Expectation: harness.Expectation{
		Touched: []string{renamed},
		Renames: map[string]string{f.symbol: renamed},
	}}, nil
}

func renamable(f fn, names packageNames) bool {
	name := f.decl.Name.Name
	switch {
	case f.obj == nil || name == "_" || unicode.IsUpper([]rune(name)[0]):
		return false
	case f.decl.Recv == nil && (name == "init" || name == "main" && f.pkg.Name == "main"):
		return false
	case f.decl.Recv != nil && names.interfaceMethods[name]:
		return false
	}
	return !names.opaque && !names.unchecked[name]
}

// packageNames is a package type-checked with its in-package test files, plus every
// identifier spelled anywhere in its directory.
type packageNames struct {
	pkg              *packages.Package
	all              map[string]bool
	unchecked        map[string]bool
	interfaceMethods map[string]bool
	opaque           bool
}

// scopeOf loads p again with its in-package tests, which the harness's own load leaves out,
// and parses the directory's remaining .go files (other platforms, external tests) for names
// only. It returns nil when the package does not load.
func scopeOf(c *harness.Corpus, p *packages.Package) *packageNames {
	cfg := &packages.Config{
		Dir:   c.Root,
		Tests: true,
		Mode:  packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo,
	}
	loaded, err := packages.Load(cfg, p.PkgPath)
	if err != nil {
		return nil
	}
	var typed *packages.Package
	for _, q := range loaded {
		if q.ID == p.PkgPath+" ["+p.PkgPath+".test]" || q.ID == p.PkgPath && typed == nil {
			typed = q
		}
	}
	if typed == nil || len(typed.Errors) > 0 || len(typed.CompiledGoFiles) == 0 {
		return nil
	}
	n := &packageNames{pkg: typed, all: map[string]bool{}, unchecked: map[string]bool{}, interfaceMethods: map[string]bool{}}
	checked := map[string]bool{}
	for i, f := range typed.Syntax {
		checked[typed.CompiledGoFiles[i]] = true
		n.add(f, false)
	}
	paths, _ := filepath.Glob(filepath.Join(filepath.Dir(typed.CompiledGoFiles[0]), "*.go"))
	for _, path := range paths {
		if checked[path] {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
		if err != nil {
			n.opaque = true
			continue
		}
		n.add(f, f.Name.Name != typed.Name+"_test")
	}
	return n
}

func (n *packageNames) add(f *ast.File, unchecked bool) {
	for name := range identNames(f) {
		n.all[name] = true
		if unchecked {
			n.unchecked[name] = true
		}
	}
	ast.Inspect(f, func(node ast.Node) bool {
		if it, ok := node.(*ast.InterfaceType); ok {
			for _, m := range it.Methods.List {
				for _, id := range m.Names {
					n.interfaceMethods[id.Name] = true
				}
			}
		}
		return true
	})
}

func (n *packageNames) objectAt(path string, offset int) types.Object {
	for id, obj := range n.pkg.TypesInfo.Defs {
		if obj != nil && n.pkg.Fset.Position(id.Pos()).Filename == path && n.pkg.Fset.Position(id.Pos()).Offset == offset {
			return obj
		}
	}
	return nil
}

func identNames(f *ast.File) map[string]bool {
	out := map[string]bool{}
	ast.Inspect(f, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok {
			out[id.Name] = true
		}
		return true
	})
	return out
}

// fresh returns a name no identifier in scope already spells, so the rename can neither
// collide with nor shadow anything.
func fresh(base string, taken map[string]bool) string {
	name := base + "Renamed"
	for i := 2; taken[name]; i++ {
		name = base + "Renamed" + strconv.Itoa(i)
	}
	return name
}

// renameObject respells every definition and use of target across files, matching
// instantiated generic functions and methods by their origin.
func renameObject(files []unit, target types.Object, name string) (harness.Edit, error) {
	if target == nil {
		return nil, errors.New("no object to rename")
	}
	edit := harness.Edit{}
	for _, u := range files {
		var ss []splice
		ast.Inspect(u.file, func(n ast.Node) bool {
			id, ok := n.(*ast.Ident)
			if !ok {
				return true
			}
			obj := u.pkg.TypesInfo.Defs[id]
			if obj == nil {
				obj = u.pkg.TypesInfo.Uses[id]
			}
			if obj != nil && origin(obj) == target {
				ss = append(ss, splice{u.offset(id.Pos()), u.offset(id.End()), name})
			}
			return true
		})
		if len(ss) == 0 {
			continue
		}
		out, err := finish(u.path, applySplices(u.src, ss))
		if err != nil {
			return nil, err
		}
		edit[u.rel] = out
	}
	if len(edit) == 0 {
		return nil, errors.New("rename touched nothing")
	}
	return edit, nil
}

func origin(o types.Object) types.Object {
	switch o := o.(type) {
	case *types.Func:
		return o.Origin()
	case *types.Var:
		return o.Origin()
	}
	return o
}
