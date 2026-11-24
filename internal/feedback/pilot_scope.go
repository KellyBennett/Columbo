package feedback

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

// TestScope records which packages the default test check covered in one revision.
type TestScope struct {
	Packages []string `json:"packages,omitempty"`
	Reason   string   `json:"reason"`
}

const customChecks = "custom --check commands run verbatim"

func moduleScope(reason string) TestScope { return TestScope{Reason: reason} }

func (s TestScope) describe() string {
	if s.Reason == customChecks {
		return s.Reason
	}
	if len(s.Packages) == 0 {
		return "whole module (" + s.Reason + ")"
	}
	return strings.Join(s.Packages, ", ")
}

func (o PilotOptions) checkCommands(scope TestScope) []string {
	if len(o.Checks) > 0 {
		return o.Checks
	}
	test := "go test ./..."
	if len(scope.Packages) > 0 {
		quoted := make([]string, len(scope.Packages))
		for i, pkg := range scope.Packages {
			quoted[i] = quoteShell(pkg)
		}
		test = "go test " + strings.Join(quoted, " ")
	}
	return []string{"go build ./...", "go vet ./...", test}
}

func changedPaths(ctx context.Context, dir string, args ...string) ([]string, error) {
	names, err := git(ctx, dir, append([]string{"diff", "--no-ext-diff", "--no-textconv", "--no-renames", "--name-only", "-z"}, args...)...)
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(strings.Split(names, "\x00"), func(name string) bool { return name == "" }), nil
}

func candidatePaths(ctx context.Context, dir, baseline string) ([]string, error) {
	if _, err := git(ctx, dir, "add", "--intent-to-add", "--", "."); err != nil {
		return nil, err
	}
	return changedPaths(ctx, dir, baseline, "--", ".")
}

func unionPaths(sets ...[]string) []string {
	var paths []string
	for _, set := range sets {
		paths = append(paths, set...)
	}
	slices.Sort(paths)
	return slices.Compact(paths)
}

type listedPackage struct {
	ImportPath, Dir                    string
	Imports, TestImports, XTestImports []string
}

func affectedScope(ctx context.Context, options PilotOptions, checkoutDir string, changed []string) TestScope {
	switch {
	case options.FullTests:
		return moduleScope("--full-tests")
	case len(options.Checks) > 0:
		return moduleScope(customChecks)
	case len(changed) == 0:
		return moduleScope("no changed files")
	}
	module := filepath.Clean(options.Module)
	for _, path := range changed {
		rel, err := filepath.Rel(module, filepath.FromSlash(path))
		switch {
		case err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)):
			return moduleScope(path + " is outside the module")
		case slices.Contains([]string{"go.mod", "go.sum", "go.work"}, filepath.Base(path)):
			return moduleScope(path + " changes module requirements")
		case filepath.Ext(path) != ".go":
			return moduleScope(path + " is not a Go source file")
		}
	}
	packages, err := listPackages(ctx, options, filepath.Join(checkoutDir, module))
	if err != nil {
		return moduleScope("go list failed: " + err.Error())
	}
	root, err := filepath.EvalSymlinks(checkoutDir)
	if err != nil {
		return moduleScope(err.Error())
	}
	changedDirs := map[string]bool{}
	for _, path := range changed {
		changedDirs[filepath.Join(root, filepath.Dir(filepath.FromSlash(path)))] = true
	}
	importers := map[string][]string{}
	var queue []string
	for _, pkg := range packages {
		for _, imported := range slices.Concat(pkg.Imports, pkg.TestImports, pkg.XTestImports) {
			importers[imported] = append(importers[imported], pkg.ImportPath)
		}
		if dir, err := filepath.EvalSymlinks(pkg.Dir); err == nil && changedDirs[dir] {
			queue = append(queue, pkg.ImportPath)
		}
	}
	if len(queue) == 0 {
		return moduleScope("no package matches the changed files")
	}
	affected := map[string]bool{}
	for len(queue) > 0 {
		pkg := queue[0]
		queue = queue[1:]
		if !affected[pkg] {
			affected[pkg] = true
			queue = append(queue, importers[pkg]...)
		}
	}
	scope := TestScope{Reason: "affected packages"}
	for _, pkg := range packages {
		if affected[pkg.ImportPath] {
			scope.Packages = append(scope.Packages, pkg.ImportPath)
		}
	}
	slices.Sort(scope.Packages)
	return scope
}

func listPackages(ctx context.Context, options PilotOptions, dir string) ([]listedPackage, error) {
	ctx, cancel := context.WithTimeout(ctx, options.CheckTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "list", "-e", "-json=ImportPath,Dir,Imports,TestImports,XTestImports", "./...")
	var stdout, stderr bytes.Buffer
	cmd.Dir, cmd.Env, cmd.Stdout, cmd.Stderr = dir, trialEnvironment(options.PassEnv...), &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, errors.New(strings.TrimSpace(err.Error() + " " + stderr.String()))
	}
	var packages []listedPackage
	for decoder := json.NewDecoder(&stdout); ; {
		var pkg listedPackage
		if err := decoder.Decode(&pkg); err == io.EOF {
			return packages, nil
		} else if err != nil {
			return nil, err
		}
		packages = append(packages, pkg)
	}
}
