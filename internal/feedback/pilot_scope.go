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
	ImportPath, Dir                             string
	Imports, TestImports, XTestImports          []string
	EmbedFiles, TestEmbedFiles, XTestEmbedFiles []string
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
	var inModule []string
	for _, path := range changed {
		rel, err := filepath.Rel(module, filepath.FromSlash(path))
		outside := err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator))
		switch {
		case slices.Contains([]string{"go.mod", "go.sum", "go.work"}, filepath.Base(path)):
			return moduleScope(path + " changes module requirements")
		case outside && filepath.Ext(path) == ".go":
			return moduleScope(path + " is Go source outside the module")
		case !outside:
			inModule = append(inModule, rel)
		}
	}
	packages, err := listPackages(ctx, options, filepath.Join(checkoutDir, module))
	if err != nil {
		return moduleScope("go list failed: " + err.Error())
	}
	root, err := filepath.EvalSymlinks(filepath.Join(checkoutDir, module))
	if err != nil {
		return moduleScope(err.Error())
	}
	byDir, embedders, importers := map[string]string{}, map[string][]string{}, map[string][]string{}
	for _, pkg := range packages {
		for _, imported := range slices.Concat(pkg.Imports, pkg.TestImports, pkg.XTestImports) {
			importers[imported] = append(importers[imported], pkg.ImportPath)
		}
		dir, err := filepath.EvalSymlinks(pkg.Dir)
		if err != nil {
			continue
		}
		byDir[dir] = pkg.ImportPath
		for _, embedded := range slices.Concat(pkg.EmbedFiles, pkg.TestEmbedFiles, pkg.XTestEmbedFiles) {
			file := filepath.Join(dir, embedded)
			embedders[file] = append(embedders[file], pkg.ImportPath)
		}
	}
	var queue []string
	for _, rel := range inModule {
		file := filepath.Join(root, rel)
		queue = append(queue, embedders[file]...)
		for dir := filepath.Dir(file); strings.HasPrefix(dir, root); dir = filepath.Dir(dir) {
			if pkg, ok := byDir[dir]; ok {
				queue = append(queue, pkg)
				break
			}
			if dir == root {
				break
			}
		}
	}
	if len(queue) == 0 {
		return moduleScope("no package encloses the changed files")
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
	cmd := exec.CommandContext(ctx, "go", "list", "-e", "-json=ImportPath,Dir,Imports,TestImports,XTestImports,EmbedFiles,TestEmbedFiles,XTestEmbedFiles", "./...")
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
