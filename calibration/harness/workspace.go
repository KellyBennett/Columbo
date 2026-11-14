package harness

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/tools/go/packages"
)

type Validity string

const (
	ValidityTest  Validity = "test"
	ValidityBuild Validity = "build"
)

type Env struct {
	Columbo  string
	Validity Validity
	Timeout  time.Duration
}

func TempDir(pattern string) (string, error) {
	dir, err := os.MkdirTemp("", pattern)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(dir)
}

// Nested modules outside testdata fall outside ./... and only slow every mutant; those under testdata are fixtures and must come along.
func CopyModule(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		if d.IsDir() {
			if rel != "." && (skippedDir(d.Name()) || isModuleRoot(path) && !underTestdata(rel)) {
				return filepath.SkipDir
			}
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		if !d.Type().IsRegular() || strings.HasSuffix(d.Name(), ".sqlite") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dst, rel), data, 0o644)
	})
}

func skippedDir(name string) bool {
	return name == ".git" || name == "node_modules" || name == ".tools" || name == ".claude"
}

func underTestdata(rel string) bool {
	for _, part := range strings.Split(filepath.ToSlash(rel), "/") {
		if part == "testdata" {
			return true
		}
	}
	return false
}

func isModuleRoot(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "go.mod"))
	return err == nil
}

func WriteEdit(root string, edit Edit) error {
	for rel, data := range edit {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if data == nil {
			if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func LoadPackages(root string) ([]*packages.Package, error) {
	cfg := &packages.Config{
		Dir:   root,
		Mode:  packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedModule | packages.NeedImports,
		Tests: false,
	}
	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, err
	}
	for _, p := range pkgs {
		if len(p.Errors) > 0 {
			return nil, fmt.Errorf("load %s: %v", p.PkgPath, p.Errors[0])
		}
	}
	return pkgs, nil
}

func (e Env) run(ctx context.Context, dir string, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOFLAGS=-buildvcs=false", "GOWORK=off", "TMPDIR="+os.TempDir())
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Run()
	return out.String(), err
}

func (e Env) context() (context.Context, context.CancelFunc) {
	timeout := e.Timeout
	if timeout == 0 {
		timeout = 10 * time.Minute
	}
	return context.WithTimeout(context.Background(), timeout)
}

// Check reports why a copy is not a valid mutant, or "" when it builds and its tests pass.
func (e Env) Check(dir string, pkgs []string) string {
	ctx, cancel := e.context()
	defer cancel()
	if out, err := e.run(ctx, dir, "go", "vet", "./..."); err != nil {
		return "vet: " + firstLines(out, 3)
	}
	if e.Validity == ValidityBuild {
		return ""
	}
	args := append([]string{"test"}, pkgs...)
	if len(pkgs) == 0 {
		args = append(args, "./...")
	}
	if out, err := e.run(ctx, dir, "go", args...); err != nil {
		return "test: " + firstLines(out, 3)
	}
	return ""
}

func (e Env) Analyze(dir string) (Snapshot, error) {
	ctx, cancel := e.context()
	defer cancel()
	tmp, err := TempDir("calibration-snapshot-")
	if err != nil {
		return Snapshot{}, err
	}
	defer os.RemoveAll(tmp)
	out := filepath.Join(tmp, "snapshot.sqlite")
	text, err := e.run(ctx, dir, e.Columbo, "--staged=false", "--no-history", "--output", out, "./...")
	var exit *exec.ExitError
	if err != nil && !(errors.As(err, &exit) && exit.ExitCode() == 1) {
		return Snapshot{}, fmt.Errorf("columbo: %v: %s", err, firstLines(text, 5))
	}
	snap, err := ReadSnapshot(out)
	snap.Output = text
	return snap, err
}

func firstLines(s string, n int) string {
	lines := strings.SplitN(strings.TrimSpace(s), "\n", n+1)
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, " | ")
}
