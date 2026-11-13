package operators

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/KellyBennett/Columbo/calibration/harness"
	"github.com/mibk/dupl/syntax"
	"github.com/mibk/dupl/syntax/golang"
)

const duplicateSmell = "duplicate-code"

var copyName = regexp.MustCompile(`Dup[0-9]*$`)

// canonical renders two declarations equal exactly when one is a copy of the other up to local names.
func canonical(f fn) string {
	var b strings.Builder
	locals := map[types.Object]int{}
	visit := func(n ast.Node) bool {
		if n == nil {
			return false
		}
		b.WriteString(reflect.TypeOf(n).Elem().Name())
		switch n := n.(type) {
		case *ast.Ident:
			obj := f.pkg.TypesInfo.Defs[n]
			if obj == nil {
				obj = f.pkg.TypesInfo.Uses[n]
			}
			if obj != nil && local(f, obj) {
				if _, ok := locals[obj]; !ok {
					locals[obj] = len(locals)
				}
				fmt.Fprintf(&b, ":$%d", locals[obj])
			} else {
				b.WriteString(":" + n.Name)
			}
		case *ast.BasicLit:
			b.WriteString(":" + n.Value)
		}
		v := reflect.ValueOf(n).Elem()
		for i := 0; i < v.NumField(); i++ {
			if tok, ok := v.Field(i).Interface().(token.Token); ok {
				b.WriteString("/" + tok.String())
			}
		}
		if ct, ok := n.(*ast.ChanType); ok {
			fmt.Fprintf(&b, "/%d", ct.Dir)
		}
		b.WriteString(" ")
		return true
	}
	if f.decl.Recv != nil {
		ast.Inspect(f.decl.Recv.List[0].Type, visit)
	}
	ast.Inspect(f.decl.Type, visit)
	ast.Inspect(f.decl.Body, visit)
	return b.String()
}

func twins(c *harness.Corpus, f fn) []fn {
	if f.decl.Body == nil || f.decl.Type.TypeParams != nil {
		return nil
	}
	want := canonical(f)
	var out []fn
	for _, g := range funcs(c) {
		if g.symbol != f.symbol && g.pkg == f.pkg && g.decl.Body != nil && (g.decl.Recv == nil) == (f.decl.Recv == nil) && canonical(g) == want {
			out = append(out, g)
		}
	}
	return out
}

type duplicateBlock struct{}

func (duplicateBlock) ID() string           { return "duplicate-block" }
func (duplicateBlock) Class() harness.Class { return harness.Inject }
func (duplicateBlock) Target() string       { return duplicateSmell }
func (duplicateBlock) Doses(*harness.Corpus, harness.Site) []int {
	return []int{1}
}

func (o duplicateBlock) Sites(c *harness.Corpus) []harness.Site {
	var out []harness.Site
	for _, f := range funcs(c) {
		if f.decl.Body != nil && len(f.decl.Body.List) > 0 && !c.Baseline.DuplicateFragments[f.symbol] {
			out = append(out, f.site())
		}
	}
	return out
}

func (o duplicateBlock) Apply(c *harness.Corpus, site harness.Site, _ int) (harness.Applied, error) {
	f, ok := findFn(c, site.Symbol)
	if !ok {
		return harness.Applied{}, errors.New("site vanished")
	}
	name := collabFreeName(f, collabLowerFirst(f.decl.Name.Name)+"Dup")
	start, end := f.offset(f.decl.Pos()), f.offset(f.decl.End())
	nameAt := f.offset(f.decl.Name.Pos()) - start
	text := string(f.src[start:end])
	text = text[:nameAt] + name + text[nameAt+len(f.decl.Name.Name):]
	out, err := finish(f.path, applySplices(f.src, []splice{{end, end, "\n\n" + text}}))
	if err != nil {
		return harness.Applied{}, err
	}
	tokens, err := dupNodes(out, name)
	if err != nil {
		return harness.Applied{}, err
	}
	limit := int(threshold(c, "duplicate-tokens", 50))
	kill := tokens >= limit
	return harness.Applied{
		Edit: harness.Edit{f.rel: out},
		Expectation: harness.Expectation{
			Touched: []string{f.symbol, symbolOf(f.pkg.PkgPath, &ast.FuncDecl{Recv: f.decl.Recv, Name: ast.NewIdent(name)})},
			Prediction: harness.Prediction{Kill: &kill, Reason: fmt.Sprintf(
				"copied declaration has %d dupl nodes (minimum %d)", tokens, limit)},
		},
	}, nil
}

// dupNodes counts serialized dupl syntax nodes, the unit Columbo's duplicate-tokens threshold compares against.
func dupNodes(src []byte, name string) (int, error) {
	dir, err := harness.TempDir("calibration-dupl-")
	if err != nil {
		return 0, err
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "copy.go")
	if err := os.WriteFile(path, src, 0o644); err != nil {
		return 0, err
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, src, parser.SkipObjectResolution)
	if err != nil {
		return 0, err
	}
	at := -1
	for _, d := range file.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Name.Name == name {
			at = fset.Position(fd.Pos()).Offset
		}
	}
	root, err := golang.Parse(path)
	if err != nil {
		return 0, err
	}
	for _, n := range syntax.Serialize(root) {
		if n.Type == golang.FuncDecl && n.Pos == at {
			return n.Owns + 1, nil
		}
	}
	return 0, fmt.Errorf("%s not found in dupl tree", name)
}

type renameAndRelabel struct{}

func (renameAndRelabel) ID() string           { return "rename-and-relabel" }
func (renameAndRelabel) Class() harness.Class { return harness.Evade }
func (renameAndRelabel) Target() string       { return duplicateSmell }
func (renameAndRelabel) Follows() string      { return "duplicate-block" }
func (renameAndRelabel) Doses(*harness.Corpus, harness.Site) []int {
	return []int{1}
}

func (o renameAndRelabel) Sites(c *harness.Corpus) []harness.Site {
	var out []harness.Site
	for _, f := range funcs(c) {
		if copyName.MatchString(f.decl.Name.Name) && len(twins(c, f)) > 0 {
			out = append(out, f.site())
		}
	}
	return out
}

func (o renameAndRelabel) Apply(c *harness.Corpus, site harness.Site, _ int) (harness.Applied, error) {
	f, ok := findFn(c, site.Symbol)
	if !ok {
		return harness.Applied{}, errors.New("site vanished")
	}
	originals := twins(c, f)
	if len(originals) == 0 {
		return harness.Applied{}, errors.New("no original to evade")
	}
	skip := map[*ast.BasicLit]bool{}
	ast.Inspect(f.decl, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.ArrayType:
			if lit, ok := n.Len.(*ast.BasicLit); ok {
				skip[lit] = true
			}
		case *ast.Field:
			if n.Tag != nil {
				skip[n.Tag] = true
			}
		}
		return true
	})
	renamed := map[types.Object]string{}
	guards := dupTypeSwitchSymbols(f)
	var ss []splice
	ast.Inspect(f.decl, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.Ident:
			if n == f.decl.Name || n.Name == "_" {
				return true
			}
			obj := f.pkg.TypesInfo.Defs[n]
			if obj == nil {
				obj = f.pkg.TypesInfo.Uses[n]
			}
			if g, ok := guards[n]; ok {
				obj = g
			}
			if _, isVar := obj.(*types.Var); !isVar || !local(f, obj) {
				return true
			}
			if _, ok := renamed[obj]; !ok {
				renamed[obj] = collabFreeName(f, obj.Name()+"Alt")
			}
			ss = append(ss, splice{f.offset(n.Pos()), f.offset(n.End()), renamed[obj]})
		case *ast.BasicLit:
			if text, ok := relabel(n); ok && !skip[n] {
				ss = append(ss, splice{f.offset(n.Pos()), f.offset(n.End()), text})
			}
		}
		return true
	})
	out, err := finish(f.path, applySplices(f.src, ss))
	if err != nil {
		return harness.Applied{}, err
	}
	touched := []string{f.symbol}
	for _, g := range originals {
		touched = append(touched, g.symbol)
	}
	return harness.Applied{
		Edit:        harness.Edit{f.rel: out},
		Expectation: harness.Expectation{Touched: touched, Catches: []string{duplicateSmell}},
	}, nil
}

// A `switch x := v.(type)` symbol has no object of its own, so it and every per-clause object share one representative and one new name.
func dupTypeSwitchSymbols(f fn) map[*ast.Ident]types.Object {
	out := map[*ast.Ident]types.Object{}
	ast.Inspect(f.decl, func(n ast.Node) bool {
		ts, ok := n.(*ast.TypeSwitchStmt)
		if !ok {
			return true
		}
		assign, ok := ts.Assign.(*ast.AssignStmt)
		if !ok {
			return true
		}
		sym := assign.Lhs[0].(*ast.Ident)
		var rep types.Object
		for _, cc := range ts.Body.List {
			if obj := f.pkg.TypesInfo.Implicits[cc]; obj != nil {
				if rep == nil {
					rep = obj
				}
				ast.Inspect(cc, func(m ast.Node) bool {
					if id, ok := m.(*ast.Ident); ok && f.pkg.TypesInfo.Uses[id] == obj {
						out[id] = rep
					}
					return true
				})
			}
		}
		if rep != nil {
			out[sym] = rep
		}
		return true
	})
	return out
}

func relabel(lit *ast.BasicLit) (string, bool) {
	switch lit.Kind {
	case token.INT:
		v, err := strconv.ParseInt(lit.Value, 0, 64)
		if err != nil {
			return "", false
		}
		return strconv.FormatInt(v+1, 10), true
	case token.STRING:
		return lit.Value[:1] + "~" + lit.Value[1:], true
	}
	return "", false
}

// Only copies up to local names delegate; structural clones need a human to name the parameter that varies.
type delegateDuplicate struct{}

func (delegateDuplicate) ID() string           { return "delegate-duplicate" }
func (delegateDuplicate) Class() harness.Class { return harness.Cure }
func (delegateDuplicate) Target() string       { return duplicateSmell }
func (delegateDuplicate) Doses(*harness.Corpus, harness.Site) []int {
	return []int{1}
}

func (o delegateDuplicate) Sites(c *harness.Corpus) []harness.Site {
	var out []harness.Site
	for _, f := range funcs(c) {
		if f.baselineHas(c, o.Target(), f.symbol) {
			out = append(out, f.site())
		}
	}
	return out
}

func (o delegateDuplicate) Apply(c *harness.Corpus, site harness.Site, _ int) (harness.Applied, error) {
	f, ok := findFn(c, site.Symbol)
	if !ok {
		return harness.Applied{}, errors.New("site vanished")
	}
	copies := twins(c, f)
	if len(copies) == 0 {
		return harness.Applied{}, errors.New("structural clone only: no copy is identical up to local names, so delegation would change behavior")
	}
	edit := harness.Edit{}
	srcs := map[string][]splice{}
	paths := map[string]string{}
	touched := []string{f.symbol}
	for _, g := range copies {
		call, err := delegation(f, g)
		if err != nil {
			return harness.Applied{}, err
		}
		srcs[g.rel] = append(srcs[g.rel], splice{g.offset(g.decl.Body.Lbrace) + 1, g.offset(g.decl.Body.Rbrace), "\n" + call + "\n"})
		paths[g.rel] = g.path
		touched = append(touched, g.symbol)
	}
	for rel, ss := range srcs {
		u, _ := findUnit(c, rel)
		out, err := finish(paths[rel], applySplices(u.src, ss))
		if err != nil {
			return harness.Applied{}, err
		}
		edit[rel] = out
	}
	return harness.Applied{Edit: edit, Expectation: harness.Expectation{Touched: touched}}, nil
}

func delegation(f, g fn) (string, error) {
	if !forwardable(g) {
		return "", errors.New("copy has unnamed parameters")
	}
	callee := f.decl.Name.Name
	if f.decl.Recv != nil {
		if !types.Identical(f.obj.Type().(*types.Signature).Recv().Type(), g.obj.Type().(*types.Signature).Recv().Type()) {
			return "", errors.New("copies have different receiver types")
		}
		callee = g.decl.Recv.List[0].Names[0].Name + "." + callee
	}
	call := fmt.Sprintf("%s(%s)", callee, forwardArgs(g))
	if g.decl.Type.Results != nil && len(g.decl.Type.Results.List) > 0 {
		return "return " + call, nil
	}
	return call, nil
}
