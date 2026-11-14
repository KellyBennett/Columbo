package harness

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/pmezard/go-difflib/difflib"
)

// Measure returns the edit's unified patch and its Size: AST nodes inserted or deleted across the top-level declarations it changes.
func Measure(root string, edit Edit) ([]byte, int, error) {
	var patch bytes.Buffer
	size := 0
	paths := make([]string, 0, len(edit))
	for p := range edit {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, rel := range paths {
		before, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil && !os.IsNotExist(err) {
			return nil, 0, err
		}
		after := edit[rel]
		text, err := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
			A: difflib.SplitLines(string(before)), B: difflib.SplitLines(string(after)),
			FromFile: "a/" + rel, ToFile: "b/" + rel, Context: 2,
		})
		if err != nil {
			return nil, 0, err
		}
		patch.WriteString(text)
		if strings.HasSuffix(rel, ".go") {
			n, err := declDistance(before, after)
			if err != nil {
				return nil, 0, fmt.Errorf("%s: %w", rel, err)
			}
			size += n
		}
	}
	return append([]byte{}, patch.Bytes()...), size, nil
}

func declDistance(before, after []byte) (int, error) {
	b, err := declSequences(before)
	if err != nil {
		return 0, err
	}
	a, err := declSequences(after)
	if err != nil {
		return 0, err
	}
	total := EditDistance(commentSequence(before), commentSequence(after))
	for name, seq := range b {
		total += EditDistance(seq, a[name])
	}
	for name, seq := range a {
		if _, ok := b[name]; !ok {
			total += len(seq)
		}
	}
	return total, nil
}

func declSequences(src []byte) (map[string][]string, error) {
	out := map[string][]string{}
	if src == nil {
		return out, nil
	}
	f, err := parser.ParseFile(token.NewFileSet(), "", src, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	for i, d := range f.Decls {
		out[declName(d, i)] = nodeSequence(d)
	}
	return out, nil
}

func declName(d ast.Decl, i int) string {
	switch d := d.(type) {
	case *ast.FuncDecl:
		if d.Recv != nil && len(d.Recv.List) > 0 {
			return "func " + types(d.Recv.List[0].Type) + "." + d.Name.Name
		}
		return "func " + d.Name.Name
	case *ast.GenDecl:
		var names []string
		for _, s := range d.Specs {
			switch s := s.(type) {
			case *ast.TypeSpec:
				names = append(names, s.Name.Name)
			case *ast.ValueSpec:
				for _, n := range s.Names {
					names = append(names, n.Name)
				}
			case *ast.ImportSpec:
				names = append(names, s.Path.Value)
			}
		}
		return d.Tok.String() + " " + strings.Join(names, ",")
	}
	return fmt.Sprint("decl", i)
}

func types(e ast.Expr) string {
	switch e := e.(type) {
	case *ast.StarExpr:
		return "*" + types(e.X)
	case *ast.Ident:
		return e.Name
	case *ast.IndexExpr:
		return types(e.X)
	case *ast.IndexListExpr:
		return types(e.X)
	}
	return "?"
}

func nodeSequence(n ast.Node) []string {
	var seq []string
	ast.Inspect(n, func(n ast.Node) bool {
		if n == nil {
			return false
		}
		if _, isComment := n.(*ast.CommentGroup); isComment {
			return false
		}
		label := reflect.TypeOf(n).Elem().Name()
		switch n := n.(type) {
		case *ast.Ident:
			label += ":" + n.Name
		case *ast.BasicLit:
			label += ":" + n.Value
		}
		seq = append(seq, label)
		return true
	})
	return seq
}

// EditDistance counts the insertions and deletions that turn a into b.
func EditDistance(a, b []string) int {
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			if a[i-1] == b[j-1] {
				cur[j] = prev[j-1] + 1
			} else {
				cur[j] = max(prev[j], cur[j-1])
			}
		}
		prev, cur = cur, prev
	}
	return len(a) + len(b) - 2*prev[len(b)]
}

// Per-declaration node sequences never visit comments outside declarations, such as file-level prose.
func commentSequence(src []byte) []string {
	if src == nil {
		return nil
	}
	f, err := parser.ParseFile(token.NewFileSet(), "", src, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return nil
	}
	var out []string
	for _, g := range f.Comments {
		for _, c := range g.List {
			out = append(out, c.Text)
		}
	}
	return out
}
