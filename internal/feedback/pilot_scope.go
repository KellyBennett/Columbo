package feedback

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"maps"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

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

func intendToAddUntracked(ctx context.Context, dir string) error {
	_, err := git(ctx, dir, "add", "--intent-to-add", "--", ".")
	return err
}

func candidatePaths(ctx context.Context, dir, baseline string) ([]string, error) {
	if err := intendToAddUntracked(ctx, dir); err != nil {
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

func (p listedPackage) imports() []string {
	return slices.Concat(p.Imports, p.TestImports, p.XTestImports)
}

func (p listedPackage) embeds() []string {
	return slices.Concat(p.EmbedFiles, p.TestEmbedFiles, p.XTestEmbedFiles)
}

func affectedScope(ctx context.Context, options PilotOptions, checkoutDir string, changed []string) TestScope {
	inModule, reason := options.scopedPaths(changed)
	if reason != "" {
		return moduleScope(reason)
	}
	graph, err := loadPackageGraph(ctx, options, filepath.Join(checkoutDir, filepath.Clean(options.Module)))
	if err != nil {
		return moduleScope(err.Error())
	}
	return graph.scope(inModule)
}

func (o PilotOptions) scopedPaths(changed []string) ([]string, string) {
	switch {
	case o.FullTests:
		return nil, "--full-tests"
	case len(o.Checks) > 0:
		return nil, customChecks
	case len(changed) == 0:
		return nil, "no changed files"
	}
	return modulePaths(filepath.Clean(o.Module), changed)
}

func modulePaths(module string, changed []string) ([]string, string) {
	var inModule []string
	for _, path := range changed {
		rel, err := filepath.Rel(module, filepath.FromSlash(path))
		inside := err == nil && !escapes(rel)
		if reason := wholeModuleReason(path, inside); reason != "" {
			return nil, reason
		}
		if inside {
			inModule = append(inModule, rel)
		}
	}
	return inModule, ""
}

func escapes(rel string) bool {
	return rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func wholeModuleReason(path string, inModule bool) string {
	switch {
	case slices.Contains([]string{"go.mod", "go.sum", "go.work"}, filepath.Base(path)):
		return path + " changes module requirements"
	case !inModule && filepath.Ext(path) == ".go":
		return path + " is Go source outside the module"
	}
	return ""
}

type packageGraph struct {
	root      string
	byDir     map[string]string
	embedders map[string][]string
	importers map[string][]string
}

func loadPackageGraph(ctx context.Context, options PilotOptions, dir string) (*packageGraph, error) {
	packages, err := listPackages(ctx, options, dir)
	if err != nil {
		return nil, errors.New("go list failed: " + err.Error())
	}
	root, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return nil, err
	}
	return newPackageGraph(root, packages), nil
}

func newPackageGraph(root string, packages []listedPackage) *packageGraph {
	graph := &packageGraph{root: root, byDir: map[string]string{}, embedders: map[string][]string{}, importers: map[string][]string{}}
	for _, pkg := range packages {
		graph.add(pkg)
	}
	return graph
}

func (g *packageGraph) add(pkg listedPackage) {
	for _, imported := range pkg.imports() {
		g.importers[imported] = append(g.importers[imported], pkg.ImportPath)
	}
	dir, err := filepath.EvalSymlinks(pkg.Dir)
	if err != nil {
		return
	}
	g.byDir[dir] = pkg.ImportPath
	for _, embedded := range pkg.embeds() {
		file := filepath.Join(dir, embedded)
		g.embedders[file] = append(g.embedders[file], pkg.ImportPath)
	}
}

func (g *packageGraph) scope(changed []string) TestScope {
	owners := g.owners(changed)
	if len(owners) == 0 {
		return moduleScope("no package encloses the changed files")
	}
	return TestScope{Packages: slices.Sorted(maps.Keys(g.dependents(owners))), Reason: "affected packages"}
}

func (g *packageGraph) owners(changed []string) []string {
	var owners []string
	for _, rel := range changed {
		file := filepath.Join(g.root, rel)
		owners = append(owners, g.embedders[file]...)
		if pkg, ok := g.enclosing(file); ok {
			owners = append(owners, pkg)
		}
	}
	return owners
}

func (g *packageGraph) enclosing(file string) (string, bool) {
	for dir := filepath.Dir(file); strings.HasPrefix(dir, g.root); dir = filepath.Dir(dir) {
		if pkg, ok := g.byDir[dir]; ok {
			return pkg, true
		}
		if dir == g.root {
			break
		}
	}
	return "", false
}

func (g *packageGraph) dependents(queue []string) map[string]bool {
	affected := map[string]bool{}
	for len(queue) > 0 {
		pkg := queue[0]
		queue = queue[1:]
		if !affected[pkg] {
			affected[pkg] = true
			queue = append(queue, g.importers[pkg]...)
		}
	}
	return affected
}

func listPackages(ctx context.Context, options PilotOptions, dir string) ([]listedPackage, error) {
	ctx, cancel := context.WithTimeout(ctx, options.CheckTimeout)
	defer cancel()
	stdout, err := options.goList(ctx, dir)
	if err != nil {
		return nil, err
	}
	return decodePackages(stdout)
}

func (o PilotOptions) goList(ctx context.Context, dir string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "go", "list", "-e", "-json=ImportPath,Dir,Imports,TestImports,XTestImports,EmbedFiles,TestEmbedFiles,XTestEmbedFiles", "./...")
	var stdout, stderr bytes.Buffer
	cmd.Dir, cmd.Env, cmd.Stdout, cmd.Stderr = dir, trialEnvironment(o.PassEnv...), &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, errors.New(strings.TrimSpace(err.Error() + " " + stderr.String()))
	}
	return stdout.Bytes(), nil
}

func decodePackages(data []byte) ([]listedPackage, error) {
	var packages []listedPackage
	for decoder := json.NewDecoder(bytes.NewReader(data)); ; {
		var pkg listedPackage
		if err := decoder.Decode(&pkg); err == io.EOF {
			return packages, nil
		} else if err != nil {
			return nil, err
		}
		packages = append(packages, pkg)
	}
}
