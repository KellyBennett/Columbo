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
	depTypePackages                  map[string]bool
	depReceipts                      []Source
	depUses                          *dependencyUses
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
	if e := a.loadUniverse(patterns); e != nil {
		return nil, e
	}
	return a, nil
}

func (a *engine) loadUniverse(patterns []string) error {
	pkgs, err := a.loadPackages(patterns)
	if err != nil {
		return err
	}
	if err := a.loadSources(pkgs); err != nil {
		return err
	}
	a.findPrivate()
	a.findInterfaces(pkgs)
	a.findCalls()
	return nil
}
func newEngine(root, dir string, c Config) *engine {
	return &engine{root: root, dir: dir, config: c, fset: token.NewFileSet(), identities: map[string]string{}, report: emptyReport()}
}
func emptyReport() Report {
	return Report{Version: 1, Cases: []Case{}, Suppressions: []Suppression{}, Warnings: []Warning{}}
}
func (a *engine) packageConfig() *packages.Config {
	return &packages.Config{Dir: a.dir, Tests: false, Fset: a.fset, Mode: packageLoadMode, ParseFile: parsePhysicalFile}
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
	selected, err := selectedPackage(p, l.engine.root)
	if err != nil || !selected {
		return err
	}
	original := originalSources(p)
	for _, f := range p.Syntax {
		if e := l.physicalFile(p, f, original); e != nil {
			return e
		}
	}
	return l.verifyOriginals(original)
}
func selectedPackage(p *packages.Package, root string) (bool, error) {
	if p.Name == "main" && strings.HasSuffix(p.PkgPath, ".test") {
		return false, nil
	}
	if p.Module == nil || filepath.Clean(p.Module.Dir) != root {
		return false, fmt.Errorf("selected package %s is outside invocation module", p.PkgPath)
	}
	return true, nil
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
	path := l.engine.physicalPath(f)
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
func (a *engine) physicalPath(node ast.Node) string {
	return filepath.Clean(a.fset.PositionFor(node.Pos(), false).Filename)
}
func (a *engine) readSource(p *packages.Package, astFile *ast.File, path string) (*file, error) {
	rel, err := a.relativeSource(path)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	source := a.sourceFile(p, astFile, path, rel)
	return source, source.initialize(data, a.config.Exclude)
}
func (a *engine) sourceFile(p *packages.Package, astFile *ast.File, path, rel string) *file {
	source := &file{fset: a.fset, ast: astFile, path: path, rel: rel, pkg: p}
	source.tf = source.tokenFile()
	return source
}
func (f *file) tokenFile() *token.File { return f.fset.File(f.ast.Pos()) }
func (f *file) initialize(data []byte, excludes []string) error {
	f.data = data
	f.included = !generated(data)
	f.scan()
	return f.exclude(excludes)
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
func newDeclaration(f *file, fn *ast.FuncDecl, ordinals map[string]int) (*declaration, error) {
	d := &declaration{fn: fn, file: f, inputs: map[*types.Var]string{}, deps: map[string]bool{}}
	if err := d.resolveSignature(); err != nil {
		return nil, err
	}
	d.assignSymbol(ordinals)
	d.initializeInputs()
	return d, nil
}
func (d *declaration) assignSymbol(ordinals map[string]int) {
	d.symbol = d.symbolName(declarationName(d.functionName().Name, ordinals))
}
func (a *engine) addDeclaration(f *file, fn *ast.FuncDecl, ordinals map[string]int) error {
	d, err := newDeclaration(f, fn, ordinals)
	if err != nil {
		return err
	}
	a.declarations = append(a.declarations, d)
	a.indexDeclaration(d)
	return nil
}
func (a *engine) indexDeclaration(d *declaration) {
	if d.obj == nil {
		return
	}
	if a.objects == nil {
		a.objects = map[*types.Func]*declaration{}
	}
	a.objects[d.obj.Origin()] = d
}
func (f *file) functionObject(name *ast.Ident) *types.Func {
	obj, _ := f.typeInfo().Defs[name].(*types.Func)
	return obj
}
func signatureOf(obj *types.Func) *types.Signature {
	if obj == nil {
		return nil
	}
	signature, _ := obj.Type().(*types.Signature)
	return signature
}
func (f *file) expressionSignature(expr ast.Expr) *types.Signature {
	signature, _ := f.typeInfo().TypeOf(expr).(*types.Signature)
	return signature
}
func (d *declaration) syntaxSignature() *types.Signature {
	return d.file.expressionSignature(d.fn.Type)
}
func (d *declaration) resolveSignature() error {
	d.obj = d.file.functionObject(d.functionName())
	d.signature = signatureOf(d.obj)
	if d.signature == nil {
		d.signature = d.syntaxSignature()
	}
	if d.signature == nil {
		return d.signatureError()
	}
	return nil
}
func (d *declaration) signatureError() error {
	return fmt.Errorf("missing physical signature for %s:%d", d.file.rel, d.declReceipt().StartOffset)
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
func (d *declaration) parameterFields() []*ast.Field          { return d.fn.Type.Params.List }
func (d *declaration) parameterVariable(index int) *types.Var { return d.signature.Params().At(index) }
func (d *declaration) initializeInputs() {
	for _, f := range d.parameterFields() {
		d.fieldParameters(f)
	}
	d.addInput(d.signature.Recv())
}
func (d *declaration) fieldParameters(f *ast.Field) {
	for k := 0; k < max(1, len(f.Names)); k++ {
		idx := len(d.params)
		v := d.parameterVariable(idx)
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

type sourceTokens struct{ scanner scanner.Scanner }

func newSourceTokens(f *token.File, data []byte) *sourceTokens {
	stream := &sourceTokens{}
	stream.scanner.Init(f, data, nil, scanner.ScanComments)
	return stream
}
func generated(b []byte) bool {
	f := token.NewFileSet().AddFile("", -1, len(b))
	return newSourceTokens(f, b).generated()
}
func (s *sourceTokens) generated() bool {
	for {
		_, tok, literal := s.scanner.Scan()
		if tok != token.COMMENT {
			return false
		}
		if generatedRE.MatchString(literal) {
			return true
		}
	}
}
func (s *sourceTokens) next() (lexToken, bool) {
	pos, tok, literal := s.scanner.Scan()
	return lexeme(pos, tok, literal), tok != token.EOF
}
func lexeme(pos token.Pos, tok token.Token, literal string) lexToken {
	width := len(literal)
	if width == 0 {
		width = len(tok.String())
	}
	if tok == token.SEMICOLON && literal == "\n" {
		width = 0
	}
	return lexToken{pos, pos + token.Pos(width), tok}
}
func (f *file) scan() {
	stream := newSourceTokens(f.tf, f.data)
	for {
		tok, ok := stream.next()
		if !ok {
			return
		}
		f.tokens = append(f.tokens, tok)
	}
}
func (f *file) receipt(kind string, start, end token.Pos, detail Detail) Source {
	detail = detail.withoutExpansion()
	p := f.tf.PositionFor(start, false)
	q := f.tf.PositionFor(end, false)
	if end > start {
		q = f.tf.PositionFor(end-1, false)
	}
	return Source{Kind: kind, File: f.rel, StartLine: p.Line, EndLine: q.Line, StartOffset: f.tf.Offset(start), EndOffset: f.tf.Offset(end), Detail: detail}
}
func (d *declaration) variable(v *types.Var) string {
	return d.file.variableIdentity(d.symbol, v)
}
func (f *file) variableIdentity(symbol string, v *types.Var) string {
	return fmt.Sprintf("%s:%s@%d", symbol, v.Name(), f.fset.PositionFor(v.Pos(), false).Offset)
}
func (f *file) line(pos token.Pos) int { return f.tf.PositionFor(pos, false).Line }
func (d *declaration) declReceipt() Source {
	return d.source("declaration", d.fn.Pos(), d.fn.End(), Detail{Subject: d.symbol, Value: nil, Nesting: nil})
}
func (a *engine) newCase(d *declaration, smell, key string) (*Case, error) {
	if a.config.Severity[smell] == "off" {
		return nil, nil
	}
	return a.identifiedCase(a.sourceCase(smell, d.declReceipt()), key)
}
func (a *engine) sourceCase(smell string, source Source) *Case {
	severity := a.config.Severity[smell]
	if severity == "off" {
		return nil
	}
	return caseFromSource(smell, severity, source)
}
func (a *engine) identifiedCase(c *Case, key string) (*Case, error) {
	if c == nil {
		return nil, nil
	}
	id, raw := identity(c.Smell, c.File, c.Symbol, key)
	if err := a.claimIdentity(id, raw); err != nil {
		return nil, err
	}
	c.ID = id
	return c, nil
}
func (a *engine) claimIdentity(id, raw string) error {
	if prev, ok := a.identities[id]; ok && prev != raw {
		return fmt.Errorf("case identity collision: %s", id)
	}
	a.identities[id] = raw
	return nil
}
func appendSources(c *Case, ss []Source) {
	for _, s := range ss {
		c.Receipts = append(c.Receipts, s)
	}
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
		if d.matchesObject(obj, pos) {
			return d
		}
	}
	return nil
}

func (a *engine) Analyze() (Report, error) {
	for _, stage := range []func() error{a.inspectDeclarations, a.clumps, a.cosmetic, a.duplicates, a.repeatedVariants, a.selectionUses, a.comments, a.suppressions} {
		if e := stage(); e != nil {
			return Report{}, e
		}
	}
	return a.finishReport(), nil
}
func (a *engine) finishReport() Report {
	if a.config.History {
		a.history()
	}
	a.report.finish()
	a.report.finishRoles()
	a.report.correlate()
	a.collectAdvisories()
	a.collectDeclarationEvidence()
	return a.report
}
func (a *engine) collectAdvisories() {
	a.report.Choices, a.report.Tangles = a.choiceSets(), a.tangles()
	a.report.GuardedUpdates = a.guardedUpdateAdvisories()
	a.report.ConditionalOverwrites = a.overwriteAdvisories()
	a.report.CategorySplit = a.categorySplitAdvisories()
	a.collectStages()
}
func (a *engine) inspectDeclarations() error {
	for _, d := range a.declarations {
		if !d.file.included {
			continue
		}
		d.measure(a)
		if e := a.ordinary(d); e != nil {
			return e
		}
	}
	return nil
}
func Analyze(dir string, patterns []string, c Config) (Report, error) {
	a, e := load(dir, patterns, c)
	if e != nil {
		return Report{}, e
	}
	return a.Analyze()
}

func (d *declaration) bodyRange() (token.Pos, token.Pos) {
	if d.fn.Body == nil {
		return 0, 0
	}
	return d.fn.Body.Lbrace, d.fn.Body.Rbrace
}
func (f *file) tokenEnd(pos token.Pos) token.Pos {
	for _, t := range f.tokens {
		if t.pos == pos {
			return t.end
		}
	}
	return pos + 1
}
func (f *file) lineReceipt(line int, kind string) Source {
	start := f.tf.LineStart(line)
	end := f.tf.Pos(f.tf.Size())
	if line < f.tf.LineCount() {
		end = f.tf.LineStart(line + 1)
	}
	r := f.receipt("metric-contribution", start, end, Detail{Subject: kind, Value: 1, Nesting: nil})
	r.EndLine = line
	return r
}
func (p parameter) detail(sig *types.Signature) Detail {
	return Detail{Subject: strconv.Itoa(p.index) + ":" + canonicalType(p.typ, sig), Value: p.arity(), Nesting: nil}
}
func (d *declaration) parameterReceipt(p parameter) Source {
	return p.receipt(d.file, d.signature).forDeclaration(d.ref())
}

func (p parameter) arity() int                          { return max(1, len(p.field.Names)) }
func (p parameter) sourceRange() (token.Pos, token.Pos) { return p.field.Pos(), p.field.End() }
func (p parameter) receipt(f *file, signature *types.Signature) Source {
	start, end := p.sourceRange()
	return f.receipt("parameter", start, end, p.detail(signature))
}

func privateType(obj *types.TypeName, packagePath string) bool {
	return obj != nil && !obj.Exported() && obj.Pkg() != nil && obj.Pkg().Path() == packagePath
}
func (t lexToken) within(start, end token.Pos) bool { return t.pos > start && t.pos < end }
func (t lexToken) countsAsCode() bool {
	return t.tok != token.COMMENT && t.tok != token.LBRACE && t.tok != token.RBRACE && t.tok != token.SEMICOLON
}

func (f *file) typeInfo() *types.Info     { return f.pkg.TypesInfo }
func (f *file) packagePath() string       { return f.pkg.PkgPath }
func (d *declaration) hasReceiver() bool  { return d.signature.Recv() != nil }
func (d *declaration) methodName() string { return d.obj.Name() }
func (d *declaration) hasBody() bool      { return d.fn.Body != nil }
func (d *declaration) inspectBody(visit func(ast.Node) bool) {
	if d.hasBody() {
		ast.Inspect(d.fn.Body, visit)
	}
}

func (a *engine) typePosition(obj *types.TypeName) token.Position {
	return a.fset.PositionFor(obj.Pos(), false)
}
func (a *engine) typeKey(obj *types.TypeName) string {
	p := a.typePosition(obj)
	return fmt.Sprintf("%s:%d", p.Filename, p.Offset)
}

func (d *declaration) matchesObject(obj *types.Func, pos token.Position) bool {
	return d.obj != nil && d.obj.Name() == obj.Name() && d.file.path == pos.Filename && d.file.tf.Offset(d.obj.Pos()) == pos.Offset
}

func (d *declaration) bodyNode() ast.Node       { return d.fn.Body }
func (d *declaration) functionName() *ast.Ident { return d.fn.Name }
func (d *declaration) callSite(call *ast.CallExpr) Site {
	site := d.file.callSite(call)
	ref := d.ref()
	site.Owner = &ref
	return site
}
func (f *file) tokenPositions(start, end token.Pos) []token.Pos {
	out := []token.Pos{}
	for _, t := range f.tokens {
		if t.pos >= start && t.pos < end {
			out = append(out, t.pos)
		}
	}
	return out
}

func (d *declaration) inputIdentity(expr ast.Expr) (string, bool) {
	variable := inputVariable(d.file.typeInfo(), expr)
	if variable == nil {
		return "", false
	}
	key, ok := d.inputs[variable]
	return key, ok
}

func (f *file) callSite(call *ast.CallExpr) Site {
	return Site{File: f.rel, CallOffset: f.tf.Offset(call.Pos())}
}
func (d *declaration) helperCallReceipt(call *ast.CallExpr, helper string) Source {
	return d.source("helper-call", call.Pos(), call.End(), Detail{Subject: helper})
}
func (d *declaration) identifierReceipt(id *ast.Ident, key string) Source {
	return d.source("parameter", id.Pos(), id.End(), Detail{Subject: key})
}

func (d *declaration) ref() DeclarationRef {
	return DeclarationRef{File: d.file.rel, Symbol: d.symbol}
}

type evidenceSelection map[DeclarationRef]bool

func (a *engine) collectDeclarationEvidence() {
	selection := evidenceSelection{}
	for _, c := range a.report.Cases {
		selection.addCase(c)
	}
	selection.addRoles(a.report.Roles)
	selection.addAdvisories(a.report)
	a.report.Declarations = append(selection.declarations(a.declarations), selection.packageClauses(a.files)...)
}
func (selection evidenceSelection) addCase(c Case) {
	for _, support := range c.SupportingDeclarations {
		selection[support.Declaration] = true
	}
}
func (selection evidenceSelection) declarations(declarations []*declaration) []DeclarationEvidence {
	out := []DeclarationEvidence{}
	for _, d := range declarations {
		if selection[d.ref()] {
			out = append(out, d.evidence())
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].before(out[j]) })
	return out
}
func (evidence DeclarationEvidence) before(other DeclarationEvidence) bool {
	if evidence.Ref.File != other.Ref.File {
		return evidence.Ref.File < other.Ref.File
	}
	return evidence.Ref.Symbol < other.Ref.Symbol
}
func (d *declaration) evidence() DeclarationEvidence {
	return DeclarationEvidence{Ref: d.ref(), Source: d.declReceipt(), Dependencies: d.dependencyEvidence()}
}
func (d *declaration) dependencyInventory() map[string]bool {
	inventory := map[string]bool{}
	for _, source := range d.depReceipts {
		inventory[source.DependencyIdentity] = true
	}
	for identity := range d.deps {
		inventory[identity] = true
	}
	return inventory
}
func (d *declaration) dependencyEvidence() []DependencyEvidence {
	dependencies := []DependencyEvidence{}
	for _, identity := range sortedSet(d.dependencyInventory()) {
		dependencies = append(dependencies, DependencyEvidence{identity, d.deps[identity]})
	}
	return dependencies
}

func (d *declaration) source(kind string, start, end token.Pos, detail Detail) Source {
	return d.file.receipt(kind, start, end, detail).forDeclaration(d.ref())
}
func (d *declaration) lineContribution(line int, kind string, trace *expansion) Source {
	return d.file.lineReceipt(line, kind).forDeclaration(d.ref()).withExpansion(trace)
}
func (d *declaration) memberAt(site Site) Member {
	ref := d.ref()
	return Member{Helper: d.symbol, CallOffset: site.CallOffset, Declaration: &ref}
}
func (d *declaration) memberReference(clusterKey string, site Site) MemberRef {
	return MemberRef{ClusterKey: clusterKey, Helper: d.ref(), CallOffset: site.CallOffset}
}
func (d *declaration) rootExpansion() expansion {
	return expansion{names: []string{d.symbol}, sites: []Site{}, declarations: []DeclarationRef{d.ref()}}
}

func (selection evidenceSelection) packageClauses(files []*file) []DeclarationEvidence {
	out := []DeclarationEvidence{}
	for _, f := range files {
		if selection[f.packageClauseRef()] {
			out = append(out, f.packageClauseEvidence())
		}
	}
	return out
}

func (d *declaration) hasParameter(object types.Object) bool {
	parameters := d.signature.Params()
	for index := 0; index < parameters.Len(); index++ {
		if parameters.At(index) == object {
			return true
		}
	}
	return false
}

func (owner *declaration) nodeSource(kind string, node ast.Node, detail Detail) Source {
	receipt := owner.source(kind, node.Pos(), node.End(), detail)
	receipt.Spelling = string(owner.file.data[receipt.StartOffset:receipt.EndOffset])
	return receipt
}
