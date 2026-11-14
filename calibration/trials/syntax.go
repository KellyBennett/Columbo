package trials

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"strings"
)

// funcDecl is one top-level function or method, located by module-relative file and directory.
type funcDecl struct{ file, dir, recv, name string }

func (d funcDecl) String() string {
	if d.recv != "" {
		return path.Join(d.dir, d.recv+"."+d.name)
	}
	return path.Join(d.dir, d.name)
}

// goSyntax is the top-level functions of a module tree and the identifiers its test files use, read without type checking.
// A file that does not parse contributes nothing.
type goSyntax struct {
	prod, test []funcDecl
	// used holds dir + "." + identifier for every identifier in a test file.
	used map[string]bool
}

func readSyntax(root string) (goSyntax, error) {
	files, err := goFiles(root)
	if err != nil {
		return goSyntax{}, err
	}
	g := goSyntax{used: map[string]bool{}}
	fset := token.NewFileSet()
	for _, rel := range sortedKeys(files) {
		f, err := parser.ParseFile(fset, rel, files[rel], parser.SkipObjectResolution)
		if err != nil {
			continue
		}
		dir, test := path.Dir(rel), strings.HasSuffix(rel, "_test.go")
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			d := funcDecl{file: rel, dir: dir, recv: receiverName(fn), name: fn.Name.Name}
			if test {
				g.test = append(g.test, d)
			} else {
				g.prod = append(g.prod, d)
			}
		}
		if test {
			ast.Inspect(f, func(n ast.Node) bool {
				if id, ok := n.(*ast.Ident); ok {
					g.used[dir+"."+id.Name] = true
				}
				return true
			})
		}
	}
	return g, nil
}

func receiverName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return ""
	}
	t := fn.Recv.List[0].Type
	for {
		switch x := t.(type) {
		case *ast.StarExpr:
			t = x.X
		case *ast.IndexExpr:
			t = x.X
		case *ast.IndexListExpr:
			t = x.X
		case *ast.Ident:
			return x.Name
		default:
			return ""
		}
	}
}

func declared(in []funcDecl, d funcDecl) bool {
	for _, e := range in {
		if e.dir == d.dir && e.recv == d.recv && e.name == d.name {
			return true
		}
	}
	return false
}

func declaresName(in []funcDecl, dir, name string) bool {
	for _, e := range in {
		if e.dir == dir && e.name == name {
			return true
		}
	}
	return false
}

// keepsTests holds when the fix's tests still declare every test function the mutant's tests did, and the fix still
// declares, by name, every production function or method the mutant's tests use.
func keepsTests(before, after goSyntax) bool {
	for _, d := range before.test {
		if d.recv == "" && strings.HasPrefix(d.name, "Test") && !declared(after.test, d) {
			return false
		}
	}
	for _, d := range before.prod {
		if before.used[d.dir+"."+d.name] && !declaresName(after.prod, d.dir, d.name) {
			return false
		}
	}
	return true
}
