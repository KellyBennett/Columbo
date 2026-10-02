package columbo

import (
	"fmt"
	"github.com/bmatcuk/doublestar/v4"
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
	"go/types"
	"golang.org/x/tools/go/packages"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type lexToken struct {
	pos, end token.Pos
	tok      token.Token
}
type file struct {
	fset      *token.FileSet
	path, rel string
	data      []byte
	ast       *ast.File
	tf        *token.File
	tokens    []lexToken
	included  bool
	pkg       *packages.Package
}
type declaration struct {
	fn                               *ast.FuncDecl
	file                             *file
	obj                              *types.Func
	symbol                           string
	signature                        *types.Signature
	params                           []parameter
	inputs                           map[*types.Var]string
	deps                             map[string]bool
	depReceipts                      []Source
	lines, complexity                int
	lineReceipts, complexityReceipts []Source
	candidate                        bool
}
type parameter struct {
	typ   types.Type
	field *ast.Field
	index int
	obj   *types.Var
}
type engine struct {
	root, dir    string
	config       Config
	fset         *token.FileSet
	files        []*file
	declarations []*declaration
	objects      map[*types.Func]*declaration
	calls        map[*ast.CallExpr]*declaration
	callOwner    map[*ast.CallExpr]*declaration
	interfaces   []types.Type
	private      map[*types.TypeName]bool
	identities   map[string]string
	report       Report
}

func moduleRoot(dir string) (string, error) {
	p := dir
	for {
		if st, e := os.Stat(filepath.Join(p, "go.mod")); e == nil && !st.IsDir() {
			return p, nil
		}
		parent := filepath.Dir(p)
		if parent == p {
			break
		}
		p = parent
	}
	return "", fmt.Errorf("no enclosing go.mod for %s", dir)
}
func load(dir string, patterns []string, c Config) (*engine, error) {
	root, e := moduleRoot(dir)
	if e != nil {
		return nil, e
	}
	a := newEngine(root, dir, c)
	pkgs, e := a.loadPackages(patterns)
	if e != nil {
		return nil, e
	}
	if e = a.loadSources(pkgs); e != nil {
		return nil, e
	}
	a.findPrivate()
	a.findInterfaces(pkgs)
	a.findCalls()
	return a, nil
}
func newEngine(root, dir string, c Config) *engine {
	return &engine{root: root, dir: dir, config: c, fset: token.NewFileSet(), objects: map[*types.Func]*declaration{}, calls: map[*ast.CallExpr]*declaration{}, callOwner: map[*ast.CallExpr]*declaration{}, private: map[*types.TypeName]bool{}, identities: map[string]string{}, report: emptyReport()}
}
func emptyReport() Report {
	return Report{Version: 1, Cases: []Case{}, Suppressions: []Suppression{}, Warnings: []Warning{}}
}
func (a *engine) packageConfig() *packages.Config {
	return &packages.Config{Dir: a.dir, Tests: true, Fset: a.fset, Mode: packageLoadMode, ParseFile: parsePhysicalFile}
}

const packageLoadMode = packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedImports | packages.NeedDeps | packages.NeedExportFile | packages.NeedTypes | packages.NeedSyntax | packages.NeedTypesInfo | packages.NeedModule

func parsePhysicalFile(fs *token.FileSet, p string, b []byte) (*ast.File, error) {
	return parser.ParseFile(fs, p, b, parser.ParseComments)
}
func (a *engine) loadPackages(patterns []string) ([]*packages.Package, error) {
	pkgs, e := packages.Load(a.packageConfig(), patterns...)
	if e != nil {
		return nil, e
	}
	if len(pkgs) == 0 {
		return nil, fmt.Errorf("no packages matched")
	}
	if e = packageErrors(pkgs); e != nil {
		return nil, e
	}
	return pkgs, nil
}
func packageErrors(pkgs []*packages.Package) error {
	var errors []string
	packages.Visit(pkgs, nil, func(p *packages.Package) {
		for _, e := range p.Errors {
			errors = append(errors, e.Error())
		}
	})
	if len(errors) == 0 {
		return nil
	}
	sort.Strings(errors)
	return fmt.Errorf("package loading failed:\n%s", strings.Join(errors, "\n"))
}

type sourceLoader struct {
	engine *engine
	seen   map[string]bool
}

func (a *engine) loadSources(pkgs []*packages.Package) error {
	l := &sourceLoader{engine: a, seen: map[string]bool{}}
	for _, p := range pkgs {
		if e := l.packageSources(p); e != nil {
			return e
		}
	}
	sort.Slice(a.files, func(i, j int) bool { return a.files[i].rel < a.files[j].rel })
	for _, f := range a.files {
		if e := a.fileDeclarations(f); e != nil {
			return e
		}
	}
	return nil
}
func (l *sourceLoader) packageSources(p *packages.Package) error {
	if p.Name == "main" && strings.HasSuffix(p.PkgPath, ".test") {
		return nil
	}
	if p.Module == nil || filepath.Clean(p.Module.Dir) != l.engine.root {
		return fmt.Errorf("selected package %s is outside invocation module", p.PkgPath)
	}
	original := originalSources(p)
	for _, f := range p.Syntax {
		if e := l.physicalFile(p, f, original); e != nil {
			return e
		}
	}
	return l.verifyOriginals(original)
}
func originalSources(p *packages.Package) map[string]bool {
	original := map[string]bool{}
	for _, p := range p.GoFiles {
		original[filepath.Clean(p)] = true
	}
	return original
}
func (l *sourceLoader) verifyOriginals(original map[string]bool) error {
	for p := range original {
		if !l.seen[p] {
			return fmt.Errorf("physical-source/type correspondence unavailable for %s", p)
		}
	}
	return nil
}
func (l *sourceLoader) physicalFile(p *packages.Package, f *ast.File, original map[string]bool) error {
	path := filepath.Clean(l.engine.fset.PositionFor(f.Pos(), false).Filename)
	if !original[path] || l.seen[path] {
		return nil
	}
	l.seen[path] = true
	sf, e := l.engine.readSource(p, f, path)
	if e != nil {
		return e
	}
	l.engine.files = append(l.engine.files, sf)
	return nil
}
func (a *engine) relativeSource(path string) (string, error) {
	rel, e := filepath.Rel(a.root, path)
	if e != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("source outside module: %s", path)
	}
	return filepath.ToSlash(rel), nil
}
func (a *engine) readSource(p *packages.Package, f *ast.File, path string) (*file, error) {
	rel, e := a.relativeSource(path)
	if e != nil {
		return nil, e
	}
	b, e := os.ReadFile(path)
	if e != nil {
		return nil, e
	}
	sf := &file{fset: a.fset, path: path, rel: rel, data: b, ast: f, tf: a.fset.File(f.Pos()), pkg: p, included: !generated(b)}
	sf.scan()
	return sf, sf.exclude(a.config.Exclude)
}
func (f *file) exclude(patterns []string) error {
	for _, p := range patterns {
		match, e := doublestar.Match(p, f.rel)
		if e != nil {
			return e
		}
		if match {
			f.included = false
		}
	}
	return nil
}
func (a *engine) fileDeclarations(f *file) error {
	ordinals := map[string]int{}
	for _, node := range f.ast.Decls {
		if fn, ok := node.(*ast.FuncDecl); ok {
			if e := a.addDeclaration(f, fn, ordinals); e != nil {
				return e
			}
		}
	}
	return nil
}
func declarationName(name string, ordinals map[string]int) string {
	if name != "init" && name != "_" {
		return name
	}
	ordinals[name]++
	return name + "#" + strconv.Itoa(ordinals[name])
}
func (a *engine) addDeclaration(f *file, fn *ast.FuncDecl, ordinals map[string]int) error {
	d := &declaration{fn: fn, file: f, inputs: map[*types.Var]string{}, deps: map[string]bool{}}
	if e := d.resolveSignature(); e != nil {
		return e
	}
	d.symbol = d.symbolName(declarationName(fn.Name.Name, ordinals))
	d.initializeInputs()
	a.declarations = append(a.declarations, d)
	if d.obj != nil {
		a.objects[d.obj.Origin()] = d
	}
	return nil
}
func (d *declaration) resolveSignature() error {
	d.obj, _ = d.file.pkg.TypesInfo.Defs[d.fn.Name].(*types.Func)
	if d.obj != nil {
		d.signature, _ = d.obj.Type().(*types.Signature)
	}
	if d.signature == nil {
		d.signature, _ = d.file.pkg.TypesInfo.TypeOf(d.fn.Type).(*types.Signature)
	}
	if d.signature == nil {
		return fmt.Errorf("missing physical signature for %s:%d", d.file.rel, d.file.tf.Offset(d.fn.Pos()))
	}
	return nil
}
func (d *declaration) symbolName(name string) string {
	prefix := d.file.pkg.PkgPath
	if d.signature.Recv() == nil {
		return prefix + "." + name
	}
	receiver := receiverName(d.signature.Recv().Type())
	if receiver == "" {
		return prefix + "." + name
	}
	return prefix + ".(" + receiver + ")." + name
}
func receiverName(t types.Type) string {
	t = types.Unalias(t)
	prefix := ""
	if p, ok := t.(*types.Pointer); ok {
		prefix = "*"
		t = types.Unalias(p.Elem())
	}
	if n, ok := t.(*types.Named); ok {
		return prefix + n.Obj().Name()
	}
	return ""
}
func (d *declaration) initializeInputs() {
	for _, f := range d.fn.Type.Params.List {
		d.fieldParameters(f)
	}
	d.addInput(d.signature.Recv())
}
func (d *declaration) fieldParameters(f *ast.Field) {
	for k := 0; k < max(1, len(f.Names)); k++ {
		idx := len(d.params)
		v := d.signature.Params().At(idx)
		d.params = append(d.params, parameter{v.Type(), f, idx, v})
		d.addInput(v)
	}
}
func (d *declaration) addInput(v *types.Var) {
	if v != nil && v.Name() != "" && v.Name() != "_" {
		d.inputs[v] = d.variable(v)
	}
}

var generatedRE = regexp.MustCompile(`^// Code generated .* DO NOT EDIT\.$`)

func generated(b []byte) bool {
	fs := token.NewFileSet()
	f := fs.AddFile("", -1, len(b))
	var s scanner.Scanner
	s.Init(f, b, nil, scanner.ScanComments)
	for {
		_, t, l := s.Scan()
		if t == token.COMMENT {
			if generatedRE.MatchString(l) {
				return true
			}
			continue
		}
		return false
	}
}
func (f *file) scan() {
	var s scanner.Scanner
	s.Init(f.tf, f.data, nil, scanner.ScanComments)
	for {
		p, t, l := s.Scan()
		if t == token.EOF {
			break
		}
		n := len(l)
		if n == 0 {
			n = len(t.String())
		}
		if t == token.SEMICOLON && l == "\n" {
			n = 0
		}
		f.tokens = append(f.tokens, lexToken{p, p + token.Pos(n), t})
	}
}
func (f *file) receipt(kind string, start, end token.Pos, detail Detail) Source {
	detail.Expansion = []string{}
	detail.ExpansionSites = []Site{}
	p := f.tf.PositionFor(start, false)
	q := f.tf.PositionFor(end, false)
	if end > start {
		q = f.tf.PositionFor(end-1, false)
	}
	return Source{kind, f.rel, p.Line, q.Line, f.tf.Offset(start), f.tf.Offset(end), detail}
}
func (d *declaration) variable(v *types.Var) string {
	return fmt.Sprintf("%s:%s@%d", d.symbol, v.Name(), d.file.fset.PositionFor(v.Pos(), false).Offset)
}
func (d *declaration) declReceipt() Source {
	return d.file.receipt("declaration", d.fn.Pos(), d.fn.End(), Detail{Subject: d.symbol, Value: nil, Nesting: nil})
}
func (a *engine) newCase(d *declaration, smell, key string) (*Case, error) {
	severity := a.config.Severity[smell]
	if severity == "off" {
		return nil, nil
	}
	id, raw := identity(smell, d.file.rel, d.symbol, key)
	if prev, ok := a.identities[id]; ok && prev != raw {
		return nil, fmt.Errorf("case identity collision: %s", id)
	}
	a.identities[id] = raw
	c := &Case{ID: id, Smell: smell, Verdict: strings.ToUpper(severity), Symbol: d.symbol, File: d.file.rel, StartLine: d.file.tf.Line(d.fn.Pos()), EndLine: d.file.tf.Line(d.fn.End() - 1), Clues: []Clue{}, Clusters: []Cluster{}, Leads: []string{}, Avoid: []string{}, Receipts: []any{d.declReceipt()}, PolicyReviews: []PolicyReview{}}
	guidance(c)
	if smell == "cosmetic-extraction" && policyStatus == "provisional" {
		c.PolicyReviews = append(c.PolicyReviews, policy)
	}
	return c, nil
}
func appendSources(c *Case, ss []Source) {
	for _, s := range ss {
		c.Receipts = append(c.Receipts, s)
	}
}
func (a *engine) findPrivate() {
	cross := map[string]bool{}
	key := func(t *types.TypeName) string {
		p := a.fset.PositionFor(t.Pos(), false)
		return fmt.Sprintf("%s:%d", p.Filename, p.Offset)
	}
	for _, f := range a.files {
		ast.Inspect(f.ast, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok {
				if t, ok := f.pkg.TypesInfo.Uses[id].(*types.TypeName); ok {
					if a.fset.PositionFor(t.Pos(), false).Filename != f.path {
						cross[key(t)] = true
					}
				}
			}
			return true
		})
	}
	for _, f := range a.files {
		for _, obj := range f.pkg.TypesInfo.Defs {
			if t, ok := obj.(*types.TypeName); ok && !t.Exported() {
				a.private[t] = !cross[key(t)]
			}
		}
	}
}

func (a *engine) findInterfaces(pkgs []*packages.Package) {
	seen := map[types.Type]bool{}
	var walk func(types.Type)
	walk = func(t types.Type) {
		if t == nil {
			return
		}
		t = types.Unalias(t)
		if seen[t] {
			return
		}
		seen[t] = true
		switch t := t.(type) {
		case *types.Named:
			if _, ok := t.Underlying().(*types.Interface); ok {
				a.interfaces = append(a.interfaces, t)
			}
			walk(t.Underlying())
		case *types.Interface:
			a.interfaces = append(a.interfaces, t)
			for i := 0; i < t.NumMethods(); i++ {
				walk(t.Method(i).Type())
			}
		case *types.Pointer:
			walk(t.Elem())
		case *types.Slice:
			walk(t.Elem())
		case *types.Array:
			walk(t.Elem())
		case *types.Map:
			walk(t.Key())
			walk(t.Elem())
		case *types.Chan:
			walk(t.Elem())
		case *types.Struct:
			for i := 0; i < t.NumFields(); i++ {
				walk(t.Field(i).Type())
			}
		case *types.Signature:
			for i := 0; i < t.Params().Len(); i++ {
				walk(t.Params().At(i).Type())
			}
			for i := 0; i < t.Results().Len(); i++ {
				walk(t.Results().At(i).Type())
			}
		}
	}
	packages.Visit(pkgs, func(p *packages.Package) bool {
		if p.Types != nil {
			s := p.Types.Scope()
			for _, name := range s.Names() {
				walk(s.Lookup(name).Type())
			}
		}
		if p.TypesInfo != nil {
			for _, tv := range p.TypesInfo.Types {
				walk(tv.Type)
			}
		}
		return true
	}, nil)
}
func unparen(e ast.Expr) ast.Expr {
	for {
		p, ok := e.(*ast.ParenExpr)
		if !ok {
			return e
		}
		e = p.X
	}
}
func callObject(info *types.Info, c *ast.CallExpr) *types.Func {
	e := unparen(c.Fun)
	switch x := e.(type) {
	case *ast.IndexExpr:
		e = unparen(x.X)
	case *ast.IndexListExpr:
		e = unparen(x.X)
	}
	switch x := e.(type) {
	case *ast.Ident:
		f, _ := info.Uses[x].(*types.Func)
		return f
	case *ast.SelectorExpr:
		if sel := info.Selections[x]; sel != nil {
			if _, yes := types.Unalias(stripPointer(sel.Recv())).Underlying().(*types.Interface); yes {
				return nil
			}
		}
		f, _ := info.Uses[x.Sel].(*types.Func)
		return f
	}
	return nil
}
func stripPointer(t types.Type) types.Type {
	for {
		t = types.Unalias(t)
		p, ok := t.(*types.Pointer)
		if !ok {
			return t
		}
		t = p.Elem()
	}
}

// Test variants contain distinct type-checker objects for the same physical declaration.
func (a *engine) target(obj *types.Func) *declaration {
	if obj == nil {
		return nil
	}
	obj = obj.Origin()
	if d := a.objects[obj]; d != nil {
		return d
	}
	pos := a.fset.PositionFor(obj.Pos(), false)
	for _, d := range a.declarations {
		if d.obj != nil && d.obj.Name() == obj.Name() && d.file.path == pos.Filename && d.file.tf.Offset(d.obj.Pos()) == pos.Offset {
			return d
		}
	}
	return nil
}

func (a *engine) findCalls() {
	counts := map[*declaration]int{}
	firstclass := map[*declaration]bool{}
	directIds := map[*ast.Ident]bool{}
	unique := map[*declaration]*ast.CallExpr{}
	for _, d := range a.declarations {
		if d.fn.Body == nil {
			continue
		}
		ast.Inspect(d.fn.Body, func(n ast.Node) bool {
			c, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			obj := callObject(d.file.pkg.TypesInfo, c)
			if obj == nil {
				return true
			}
			target := a.target(obj)
			if target == nil {
				return true
			}
			a.calls[c] = target
			a.callOwner[c] = d
			counts[target]++
			unique[target] = c
			e := unparen(c.Fun)
			switch x := e.(type) {
			case *ast.IndexExpr:
				e = unparen(x.X)
			case *ast.IndexListExpr:
				e = unparen(x.X)
			}
			switch x := e.(type) {
			case *ast.Ident:
				directIds[x] = true
			case *ast.SelectorExpr:
				directIds[x.Sel] = true
			}
			return true
		})
	}
	for _, f := range a.files {
		ast.Inspect(f.ast, func(n ast.Node) bool {
			// Package-level calls count too; a unique call outside a body
			// cannot become a lexical cluster member.
			if call, ok := n.(*ast.CallExpr); ok && a.calls[call] == nil {
				if obj := callObject(f.pkg.TypesInfo, call); obj != nil {
					if d := a.target(obj); d != nil {
						counts[d]++
						firstclass[d] = true
					}
				}
			}
			id, ok := n.(*ast.Ident)
			if !ok {
				return true
			}
			fn, ok := f.pkg.TypesInfo.Uses[id].(*types.Func)
			if !ok {
				return true
			}
			if d := a.target(fn); d != nil && !directIds[id] {
				firstclass[d] = true
			}
			return true
		})
	}
	for _, d := range a.declarations {
		d.candidate = d.file.included && d.fn.Body != nil && d.obj != nil && !d.obj.Exported() && d.fn.Name.Name != "_" && counts[d] == 1 && !firstclass[d]
		if !d.candidate || d.signature.Recv() == nil {
			continue
		}
		c := unique[d]
		owner := a.callOwner[c]
		e := unparen(c.Fun)
		switch x := e.(type) {
		case *ast.IndexExpr:
			e = x.X
		case *ast.IndexListExpr:
			e = x.X
		}
		selExpr, ok := e.(*ast.SelectorExpr)
		if !ok {
			continue
		}
		sel := owner.file.pkg.TypesInfo.Selections[selExpr]
		if sel == nil {
			continue
		}
		rt := stripPointer(sel.Recv())
		for _, t := range a.interfaces {
			iface, ok := t.Underlying().(*types.Interface)
			if !ok {
				continue
			}
			has := false
			for i := 0; i < iface.NumMethods(); i++ {
				if iface.Method(i).Name() == d.obj.Name() {
					has = true
				}
			}
			if has && (types.Implements(rt, iface) || types.Implements(types.NewPointer(rt), iface)) {
				d.candidate = false
				break
			}
		}
	}
}
func (a *engine) Analyze() (Report, error) {
	for _, d := range a.declarations {
		if !d.file.included {
			continue
		}
		d.measure(a)
		if e := a.ordinary(d); e != nil {
			return Report{}, e
		}
	}
	if e := a.clumps(); e != nil {
		return Report{}, e
	}
	if e := a.cosmetic(); e != nil {
		return Report{}, e
	}
	if e := a.suppressions(); e != nil {
		return Report{}, e
	}
	if a.config.History {
		a.history()
	}
	a.report.finish()
	return a.report, nil
}
func Analyze(dir string, patterns []string, c Config) (Report, error) {
	a, e := load(dir, patterns, c)
	if e != nil {
		return Report{}, e
	}
	return a.Analyze()
}
