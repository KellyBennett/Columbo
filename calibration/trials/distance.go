package trials

import (
	"bytes"
	"go/scanner"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/KellyBennett/Columbo/calibration/harness"
)

// Distance compares an agent's fix and the mutant it started from with the original module.
// Each count is the token insertions and deletions, summed over every Go file any version changed.
type Distance struct {
	Fixed  int
	Mutant int
}

// Normalized is 0 when the fix restored the original, 1 when it got no closer than the mutant, and above 1 when it moved further away.
func (d Distance) Normalized() float64 {
	if d.Mutant == 0 {
		return 0
	}
	return float64(d.Fixed) / float64(d.Mutant)
}

// MeasureDistance reads three module directories; a file missing from one of them counts as empty there.
func MeasureDistance(original, mutant, fixed string) (Distance, error) {
	versions := []map[string][]byte{}
	for _, dir := range []string{original, mutant, fixed} {
		files, err := goFiles(dir)
		if err != nil {
			return Distance{}, err
		}
		versions = append(versions, files)
	}
	var d Distance
	for _, rel := range changedPaths(versions) {
		o, m, f := tokens(versions[0][rel]), tokens(versions[1][rel]), tokens(versions[2][rel])
		d.Mutant += harness.EditDistance(m, o)
		d.Fixed += harness.EditDistance(f, o)
	}
	return d, nil
}

func changedPaths(versions []map[string][]byte) []string {
	seen := map[string]bool{}
	var out []string
	for _, v := range versions {
		for rel := range v {
			if seen[rel] {
				continue
			}
			seen[rel] = true
			if !bytes.Equal(versions[0][rel], versions[1][rel]) || !bytes.Equal(versions[0][rel], versions[2][rel]) {
				out = append(out, rel)
			}
		}
	}
	sort.Strings(out)
	return out
}

func goFiles(root string) (map[string][]byte, error) {
	out := map[string][]byte{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != root && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".go") || !d.Type().IsRegular() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		out[filepath.ToSlash(rel)] = data
		return nil
	})
	return out, err
}

// tokens keeps comments because a prose-comment mutant changes nothing else, and folds every semicolon into one token so line breaks do not count.
func tokens(src []byte) []string {
	if src == nil {
		return nil
	}
	var s scanner.Scanner
	fset := token.NewFileSet()
	s.Init(fset.AddFile("", fset.Base(), len(src)), src, func(token.Position, string) {}, scanner.ScanComments)
	var out []string
	for {
		_, tok, lit := s.Scan()
		switch {
		case tok == token.EOF:
			return out
		case tok == token.SEMICOLON:
			out = append(out, ";")
		case tok == token.COMMENT:
			out = append(out, "comment:"+strings.TrimSpace(lit))
		case tok == token.IDENT || tok.IsLiteral():
			out = append(out, tok.String()+":"+lit)
		default:
			out = append(out, tok.String())
		}
	}
}
