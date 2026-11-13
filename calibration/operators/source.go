package operators

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/printer"
	"go/scanner"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/KellyBennett/Columbo/calibration/harness"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/imports"
)

type unit struct {
	pkg  *packages.Package
	file *ast.File
	rel  string
	path string
	src  []byte
}

type fn struct {
	unit
	decl   *ast.FuncDecl
	symbol string
	obj    *types.Func
}

func units(c *harness.Corpus) []unit {
	var out []unit
	for _, p := range c.Packages {
		out = append(out, loadPackageUnits(c.Root, p)...)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].rel < out[j].rel })
	return out
}

func loadPackageUnits(root string, p *packages.Package) []unit {
	var out []unit
	for i, f := range p.Syntax {
		path := p.CompiledGoFiles[i]
		rel, err := filepath.Rel(root, path)
		if err != nil || strings.HasPrefix(rel, "..") {
			continue
		}
		src, err := os.ReadFile(path)
		if err != nil || generated(f) {
			continue
		}
		out = append(out, unit{pkg: p, file: f, rel: filepath.ToSlash(rel), path: path, src: src})
	}
	return out
}

func generated(f *ast.File) bool {
	for _, g := range f.Comments {
		if g.Pos() > f.Package {
			break
		}
		for _, c := range g.List {
			if strings.HasPrefix(c.Text, "// Code generated ") && strings.HasSuffix(c.Text, " DO NOT EDIT.") {
				return true
			}
		}
	}
	return false
}

func funcs(c *harness.Corpus) []fn {
	var out []fn
	for _, u := range units(c) {
		for _, d := range u.file.Decls {
			if d, ok := d.(*ast.FuncDecl); ok {
				obj, _ := u.pkg.TypesInfo.Defs[d.Name].(*types.Func)
				out = append(out, fn{unit: u, decl: d, symbol: symbolOf(u.pkg.PkgPath, d), obj: obj})
			}
		}
	}
	return out
}

func findFn(c *harness.Corpus, symbol string) (fn, bool) {
	for _, f := range funcs(c) {
		if f.symbol == symbol {
			return f, true
		}
	}
	return fn{}, false
}

func (f fn) site() harness.Site {
	return harness.Site{Package: f.pkg.PkgPath, Symbol: f.symbol, Offset: f.offset(f.decl.Pos()), File: f.rel}
}

// symbolOf matches Columbo's qualified-symbol format so findings join on it.
func symbolOf(pkgPath string, d *ast.FuncDecl) string {
	if d.Recv == nil || len(d.Recv.List) == 0 {
		return pkgPath + "." + d.Name.Name
	}
	return pkgPath + ".(" + receiverName(d.Recv.List[0].Type) + ")." + d.Name.Name
}

func receiverName(e ast.Expr) string {
	switch e := e.(type) {
	case *ast.StarExpr:
		return "*" + receiverName(e.X)
	case *ast.ParenExpr:
		return receiverName(e.X)
	case *ast.IndexExpr:
		return receiverName(e.X)
	case *ast.IndexListExpr:
		return receiverName(e.X)
	case *ast.Ident:
		return e.Name
	}
	return "?"
}

func (u unit) offset(p token.Pos) int { return u.pkg.Fset.Position(p).Offset }

func (u unit) text(n ast.Node) string {
	return string(u.src[u.offset(n.Pos()):u.offset(n.End())])
}

func (u unit) baselineHas(c *harness.Corpus, smell, symbol string) bool {
	for _, f := range c.Baseline.Findings {
		if f.Smell == smell && f.Symbol == symbol {
			return true
		}
	}
	return false
}

func threshold(c *harness.Corpus, name string, fallback float64) float64 {
	if v, ok := c.Baseline.Thresholds[name]; ok {
		return v
	}
	return fallback
}

type splice struct {
	start, end int
	text       string
}

func applySplices(src []byte, ss []splice) []byte {
	sort.Slice(ss, func(i, j int) bool { return ss[i].start > ss[j].start })
	out := append([]byte{}, src...)
	for _, s := range ss {
		out = append(out[:s.start], append([]byte(s.text), out[s.end:]...)...)
	}
	return out
}

// finish lets operators emit unformatted text and reference packages the file has not imported.
func finish(path string, src []byte) ([]byte, error) {
	out, err := imports.Process(path, src, &imports.Options{Comments: true, TabIndent: true, TabWidth: 8, FormatOnly: false})
	if err != nil {
		return nil, fmt.Errorf("format %s: %w", filepath.Base(path), err)
	}
	return out, nil
}

func nodeString(fset *token.FileSet, n any) string {
	var b bytes.Buffer
	_ = printer.Fprint(&b, fset, n)
	return b.String()
}

// bodyLines reimplements Columbo's function-lines metric.
func bodyLines(src []byte, body *ast.BlockStmt, fset *token.FileSet) int {
	if body == nil {
		return 0
	}
	file := fset.File(body.Pos())
	start, end := file.Offset(body.Lbrace)+1, file.Offset(body.Rbrace)
	sub := src[start:end]
	fs := token.NewFileSet()
	tf := fs.AddFile("", -1, len(sub))
	var s scanner.Scanner
	s.Init(tf, sub, nil, 0)
	lines := map[int]bool{}
	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		if tok == token.SEMICOLON || tok == token.COMMENT || tok == token.LBRACE || tok == token.RBRACE {
			continue
		}
		first := tf.Line(pos)
		last := first + strings.Count(lit, "\n")
		if lit == "" {
			last = first
		}
		for l := first; l <= last; l++ {
			lines[file.Line(body.Lbrace)+l-1] = true
		}
	}
	return len(lines)
}

func lineAware(src []byte, start, end int) splice {
	ls := start
	for ls > 0 && (src[ls-1] == ' ' || src[ls-1] == '\t') {
		ls--
	}
	le := end
	for le < len(src) && (src[le] == ' ' || src[le] == '\t' || src[le] == '\r') {
		le++
	}
	if (ls == 0 || src[ls-1] == '\n') && (le == len(src) || src[le] == '\n') {
		if le < len(src) {
			le++
		}
		return splice{ls, le, ""}
	}
	return splice{ls, end, " "}
}
