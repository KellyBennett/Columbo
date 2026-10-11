package columbo

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/types"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

type inspectionSpan struct {
	Path        string `json:"path"`
	FileSHA     string `json:"file_sha256"`
	StartLine   int    `json:"start_line"`
	EndLine     int    `json:"end_line"`
	StartOffset int    `json:"start_offset"`
	EndOffset   int    `json:"end_offset"`
	SHA         string `json:"range_sha256"`
}
type inspectionFile struct {
	Path      string `json:"path"`
	SHA       string `json:"sha256"`
	Package   string `json:"package,omitempty"`
	Selected  bool   `json:"selected_runtime"`
	Target    bool   `json:"ordinary_target"`
	Generated bool   `json:"generated"`
	Scope     string `json:"scope"`
}
type inspectionDeclaration struct {
	inspectionSpan
	Symbol      string           `json:"symbol"`
	Kind        string           `json:"kind"`
	ReceiverKey string           `json:"receiver_key,omitempty"`
	Calls       []inspectionCall `json:"calls"`
}
type inspectionFacts struct {
	ConfigFile   string                  `json:"config_file"`
	ConfigSHA    string                  `json:"config_sha256"`
	Version      int                     `json:"version"`
	Root         string                  `json:"root"`
	GoVersion    string                  `json:"go_version"`
	Environment  map[string]string       `json:"environment"`
	Patterns     []string                `json:"patterns"`
	Config       Config                  `json:"config"`
	Files        []inspectionFile        `json:"files"`
	Declarations []inspectionDeclaration `json:"declarations"`
	Types        []inspectionType        `json:"types"`
	Limits       []string                `json:"limits"`
}

func inspectionHash(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
func loadInspectionConfig(path string, explicit bool) (Config, string, error) {
	config, data, read, err := loadConfigSource(path, explicit)
	if !read {
		return config, "", err
	}
	return config, inspectionHash(data), err
}
func (f *file) inspectionSpan(n ast.Node) inspectionSpan {
	s := f.receipt("inspection", n.Pos(), n.End(), Detail{})
	return inspectionSpan{f.rel, inspectionHash(f.data), s.StartLine, s.EndLine, s.StartOffset, s.EndOffset, inspectionHash(f.data[s.StartOffset:s.EndOffset])}
}
func inspectionTypeKey(t types.Type) string {
	n, ok := stripPointer(t).(*types.Named)
	if !ok || n.Obj().Pkg() == nil {
		return ""
	}
	return n.Origin().Obj().Pkg().Path() + "." + n.Origin().Obj().Name()
}
func (d *declaration) inspectionDeclaration() inspectionDeclaration {
	out := inspectionDeclaration{inspectionSpan: d.file.inspectionSpan(d.fn), Symbol: d.symbol, Kind: "function", Calls: []inspectionCall{}}
	if d.hasReceiver() {
		out.Kind = "method"
		out.ReceiverKey = inspectionTypeKey(d.signature.Recv().Type())
	}
	return out
}

type inspectionInput struct {
	root           string
	configRelative string
	configSHA      string
	config         Config
}

func validateInspectionInput(dir, config string) (inspectionInput, error) {
	root, err := filepath.Abs(dir)
	if err != nil {
		return inspectionInput{}, err
	}
	real, err := filepath.EvalSymlinks(root)
	if err != nil {
		return inspectionInput{}, err
	}
	if real != root {
		return inspectionInput{}, fmt.Errorf("symlink source root refused: %s", root)
	}
	configPath := config
	if configPath == "" {
		configPath = filepath.Join(root, ".columbo.yml")
	}
	if !filepath.IsAbs(configPath) {
		configPath = filepath.Join(root, configPath)
	}
	configRelative, err := filepath.Rel(root, configPath)
	if err != nil || configRelative == ".." || strings.HasPrefix(configRelative, ".."+string(filepath.Separator)) {
		return inspectionInput{}, fmt.Errorf("inspection configuration must be source-manifest bound")
	}
	cfg, configSHA, err := loadInspectionConfig(configPath, config != "")
	if err != nil {
		return inspectionInput{}, err
	}
	return inspectionInput{root: root, configRelative: configRelative, configSHA: configSHA, config: cfg}, nil
}

func WriteInspectionFacts(dir, config string, patterns []string, out io.Writer) error {
	input, err := validateInspectionInput(dir, config)
	if err != nil {
		return err
	}
	if len(patterns) == 0 {
		patterns = []string{"./..."}
	}
	engine, err := load(input.root, patterns, input.config)
	if err != nil {
		return err
	}
	if engine.root != input.root {
		return fmt.Errorf("root must be module root: %s", engine.root)
	}
	facts, err := engine.inspectionFacts(patterns)
	if err != nil {
		return err
	}
	facts.ConfigFile = filepath.ToSlash(input.configRelative)
	facts.ConfigSHA = input.configSHA
	return json.NewEncoder(out).Encode(facts)
}
func (a *engine) inspectionFacts(patterns []string) (inspectionFacts, error) {
	facts := inspectionFacts{Version: 1, Root: a.root, GoVersion: runtime.Version(), Patterns: patterns, Config: a.config, Environment: map[string]string{}, Declarations: []inspectionDeclaration{}, Types: []inspectionType{}}
	for _, key := range []string{"GOOS", "GOARCH", "CGO_ENABLED", "GOFLAGS", "GOWORK", "GOTOOLCHAIN", "GOEXPERIMENT", "GOENV", "GOROOT", "GOPATH", "GOMODCACHE", "GO111MODULE", "PATH"} {
		facts.Environment[key] = os.Getenv(key)
	}
	facts.Limits = []string{"Experimental navigation only; no invariant, field flow, defect or benefit is inferred.", "Methods are source-defined selected-runtime siblings, never invented promoted methods.", "Storage follows declared local embedding; held fields do not expand recursively.", "Call targets are static local source witnesses, not alias origin, interface dispatch or effects. One-hop expansion stops at the target body.", "Non-selected build variants are listed separately and never combined into runtime types."}
	var err error
	facts.Files, err = a.inspectionFiles()
	if err != nil {
		return facts, err
	}
	for _, f := range a.files {
		facts.Types = append(facts.Types, f.inspectionTypes()...)
	}
	for _, d := range a.declarations {
		entry := d.inspectionDeclaration()
		if d.hasReceiver() {
			entry.Calls = a.inspectionCalls(d)
		}
		facts.Declarations = append(facts.Declarations, entry)
	}
	return facts, nil
}
func (a *engine) inspectionFiles() ([]inspectionFile, error) {
	selected := map[string]*file{}
	for _, f := range a.files {
		selected[f.rel] = f
	}
	result := []inspectionFile{}
	err := filepath.WalkDir(a.root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && entry.Name() == ".git" {
			return filepath.SkipDir
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink source path refused: %s", path)
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		item, err := a.inspectionEntry(path, selected)
		if err != nil {
			return err
		}
		result = append(result, item)
		return nil
	})
	sort.Slice(result, func(i, j int) bool { return result[i].Path < result[j].Path })
	return result, err
}

func (a *engine) inspectionEntry(path string, selected map[string]*file) (inspectionFile, error) {
	rel, err := a.relativeSource(path)
	if err != nil {
		return inspectionFile{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return inspectionFile{}, err
	}
	item := inspectionFile{Path: rel, SHA: inspectionHash(data), Generated: generated(data)}
	f := selected[rel]
	if f != nil && string(data) != string(f.data) {
		return inspectionFile{}, fmt.Errorf("source changed during inspection: %s", rel)
	}
	item.classifyInspectionSource(f)
	return item, nil
}

func (item *inspectionFile) classifyInspectionSource(f *file) {
	item.Scope = "not-selected-runtime: build, package-pattern or module boundary"
	if strings.HasSuffix(item.Path, "_test.go") {
		item.Scope = "test source: not selected"
	}
	if f == nil {
		return
	}
	item.Package = f.packagePath()
	item.Selected = true
	item.Target = f.included
	item.Scope = "ordinary-target"
	if !f.included {
		item.Scope = "selected-runtime supporting context: generated or configured exclusion"
	}
}
