package columbo

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/ast"
	"go/types"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

type inspectionSpan struct {
	Path        string
	FileSHA     string
	StartLine   int
	EndLine     int
	StartOffset int
	EndOffset   int
	SHA         string
}
type inspectionFile struct {
	Path      string
	SHA       string
	Size      int64
	Package   string
	Selected  bool
	Target    bool
	Generated bool
	Scope     string
}
type inspectionDeclaration struct {
	inspectionSpan
	Symbol      string
	Kind        string
	ReceiverKey string
	Calls       []inspectionCall
}
type inspectionFacts struct {
	ConfigFile   string
	ConfigSHA    string
	SourceSHA    string
	Root         string
	GoVersion    string
	Environment  map[string]string
	Patterns     []string
	Config       Config
	Files        []inspectionFile
	Declarations []inspectionDeclaration
	Types        []inspectionType
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

func (a *engine) inspectionFacts(patterns []string) (inspectionFacts, error) {
	facts := inspectionFacts{Root: a.root, GoVersion: runtime.Version(), Patterns: patterns, Config: a.config, Environment: map[string]string{}, Declarations: []inspectionDeclaration{}, Types: []inspectionType{}}
	for _, key := range []string{"GOOS", "GOARCH", "CGO_ENABLED", "GOFLAGS", "GOWORK", "GOTOOLCHAIN", "GOEXPERIMENT", "GOENV", "GOROOT", "GOPATH", "GOMODCACHE", "GO111MODULE", "PATH"} {
		facts.Environment[key] = os.Getenv(key)
	}
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
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("nonregular source path refused: %s", path)
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
	item := inspectionFile{Path: rel, SHA: inspectionHash(data), Size: int64(len(data)), Generated: strings.HasSuffix(rel, ".go") && generated(data)}
	f := selected[rel]
	if f != nil && string(data) != string(f.data) {
		return inspectionFile{}, fmt.Errorf("source changed during inspection: %s", rel)
	}
	item.classifyInspectionSource(f)
	return item, nil
}

func (item *inspectionFile) classifyInspectionSource(f *file) {
	item.Scope = "non-Go source input"
	if strings.HasSuffix(item.Path, ".go") {
		item.Scope = "not-selected-runtime: build, package-pattern or module boundary"
	}
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
