package columbo

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KellyBennett/Columbo/internal/snapshotdb"
	"github.com/stretchr/testify/require"
)

func TestBoundaryCleanupPhysicalReceipts(t *testing.T) {
	for _, expression := range []string{"item", "(\"雪\" +\n \"value\")", "call(\n first,\n second,\n)"} {
		for _, newline := range []string{"\n", "\r\n"} {
			for _, directive := range []string{"", "//line virtual.go:900\n"} {
				text := strings.ReplaceAll("package fixture\n"+directive+"func inspect() {\n _ = "+expression+"\n}\n", "\n", newline)
				fset := token.NewFileSet()
				parsed, err := parser.ParseFile(fset, "physical.go", text, parser.ParseComments)
				require.NoError(t, err)
				fn := parsed.Decls[0].(*ast.FuncDecl)
				expr := fn.Body.List[0].(*ast.AssignStmt).Rhs[0]
				owner := &declaration{file: &file{fset: fset, tf: fset.File(parsed.Pos()), data: []byte(text), rel: "physical.go"}, symbol: "fixture.inspect", fn: fn}
				begin := strings.Index(text, "_ = ") + 4
				spelling := strings.ReplaceAll(expression, "\n", newline)
				end := begin + len(spelling)
				expected := Source{File: "physical.go", StartLine: strings.Count(text[:begin], "\n") + 1, EndLine: strings.Count(text[:end-1], "\n") + 1, StartOffset: begin, EndOffset: end, Spelling: spelling, Declaration: &DeclarationRef{File: "physical.go", Symbol: "fixture.inspect"}}
				for _, kind := range []string{"choice-seed", "choice-projection", "variant-condition", "field-comparison"} {
					expected.Kind = kind
					expected.Detail = Detail{Subject: owner.symbol, Expansion: []string{}, ExpansionSites: []Site{}, ExpansionDeclarations: []DeclarationRef{}}
					require.Equal(t, expected, (&choiceConstruction{owner: owner}).receipt(kind, expr))
					require.Equal(t, expected, (&tangleScan{owner: owner}).receipt(kind, expr))
				}
				site := &variantSite{owner: owner, arms: map[string][]Source{}}
				site.addArm("domain=value", expr)
				site.addArm("domain=value", expr)
				expected.Kind = "variant-arm"
				expected.Detail = Detail{Subject: "domain=value", Expansion: []string{}, ExpansionSites: []Site{}, ExpansionDeclarations: []DeclarationRef{}}
				require.Equal(t, []Source{expected, expected}, site.arms["domain=value"])
			}
		}
	}
}

func TestBoundaryCleanupAdmission(t *testing.T) {
	for _, invalid := range []string{"", "-"} {
		_, err := OpenSnapshot(invalid)
		require.EqualError(t, err, "snapshot requires a filesystem destination")
	}
	root := t.TempDir()
	missing := filepath.Join(root, "missing.sqlite")
	_, err := OpenSnapshot(missing)
	require.True(t, errors.Is(err, os.ErrNotExist))
	for _, name := range []string{"directory", "link"} {
		path := filepath.Join(root, name)
		if name == "directory" {
			require.NoError(t, os.Mkdir(path, 0700))
		} else {
			require.NoError(t, os.Symlink(missing, path))
		}
		_, outer := OpenSnapshot(path)
		_, inner := snapshotdb.Open(path)
		require.EqualError(t, outer, "snapshot destination must be a regular non-symlink file: "+path)
		require.EqualError(t, inner, outer.Error())
	}
	for _, suffix := range []string{"-journal", "-wal", "-shm"} {
		for _, kind := range []string{"file", "directory", "dangling-link"} {
			path := filepath.Join(t.TempDir(), "report ?#%.sqlite")
			require.NoError(t, WriteSnapshot(path, Report{}, "test"))
			sidecar := path + suffix
			switch kind {
			case "file":
				require.NoError(t, os.WriteFile(sidecar, []byte("sidecar"), 0600))
			case "directory":
				require.NoError(t, os.Mkdir(sidecar, 0700))
			case "dangling-link":
				require.NoError(t, os.Symlink(missing, sidecar))
			}
			_, outer := OpenSnapshot(path)
			_, inner := snapshotdb.Open(path)
			require.EqualError(t, outer, "snapshot has an unsupported sidecar: "+sidecar)
			require.EqualError(t, inner, outer.Error())
		}
	}
	path := filepath.Join(root, "regular.sqlite")
	require.NoError(t, WriteSnapshot(path, Report{}, "test"))
	cwd, err := os.Getwd()
	require.NoError(t, err)
	relative, err := filepath.Rel(cwd, path)
	require.NoError(t, err)
	absolute, err := validateSnapshotPath(relative)
	require.NoError(t, err)
	require.Equal(t, path, absolute)
	for _, suffix := range []string{"-shm", "-wal", "-journal"} {
		require.NoError(t, os.WriteFile(path+suffix, nil, 0600))
	}
	require.EqualError(t, snapshotNoSidecars(path), "snapshot has an unsupported sidecar: "+path+"-journal")
}

type boundaryLateSidecar struct {
	snapshotFilesystem
	output, suffix string
	published      *bool
}

func (f boundaryLateSidecar) Sync(path string) error {
	if err := f.snapshotFilesystem.Sync(path); err != nil {
		return err
	}
	return os.WriteFile(f.output+f.suffix, []byte("late sidecar"), 0600)
}
func (f boundaryLateSidecar) Publish(target snapshotPublicationTarget) error {
	*f.published = true
	return f.snapshotFilesystem.Publish(target)
}
func TestBoundaryCleanupPrepublicationRecheck(t *testing.T) {
	for _, suffix := range []string{"-journal", "-wal", "-shm"} {
		path := filepath.Join(t.TempDir(), "snapshot.sqlite")
		published := false
		operations := boundaryLateSidecar{output: path, suffix: suffix, published: &published}
		err := writeSnapshotWithIO(path, snapshotContents{version: "test"}, operations)
		require.EqualError(t, err, "snapshot has an unsupported sidecar: "+path+suffix)
		require.False(t, published)
		_, err = os.Lstat(path)
		require.True(t, errors.Is(err, os.ErrNotExist))
		matches, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".columbo-*"))
		require.NoError(t, err)
		require.Empty(t, matches)
	}
}
