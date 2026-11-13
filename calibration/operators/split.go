package operators

import (
	"errors"
	"fmt"
	"go/ast"
	"go/types"
	"sort"
	"strings"
	"unicode"

	"github.com/KellyBennett/Columbo/calibration/harness"
)

type splitForwarding struct {
	id, follows, target string
	catches             []string
	class               harness.Class
}

func (o splitForwarding) ID() string           { return o.id }
func (o splitForwarding) Class() harness.Class { return o.class }
func (o splitForwarding) Target() string       { return o.target }
func (o splitForwarding) Follows() string      { return o.follows }

func (o splitForwarding) Sites(c *harness.Corpus) []harness.Site {
	var out []harness.Site
	for _, f := range funcs(c) {
		if o.class == harness.Inject && (!f.baselineHas(c, "long-function", f.symbol) || f.baselineHas(c, o.target, f.symbol)) {
			continue
		}
		if len(o.Doses(c, f.site())) > 0 {
			out = append(out, f.site())
		}
	}
	return out
}

func (o splitForwarding) Doses(c *harness.Corpus, site harness.Site) []int {
	f, ok := findFn(c, site.Symbol)
	if !ok {
		return nil
	}
	var doses []int
	for _, k := range []int{2, 3, 4} {
		if _, err := splitPlan(f, k); err == nil {
			doses = append(doses, k)
		}
	}
	return doses
}

func (o splitForwarding) Apply(c *harness.Corpus, site harness.Site, dose int) (harness.Applied, error) {
	f, ok := findFn(c, site.Symbol)
	if !ok {
		return harness.Applied{}, errors.New("site vanished")
	}
	cuts, err := splitPlan(f, dose)
	if err != nil {
		return harness.Applied{}, err
	}
	sig := f.obj.Type().(*types.Signature)
	qual := qualifier(f.unit)
	names := helperNames(f, dose)
	recv, callPrefix := "", ""
	if f.decl.Recv != nil {
		recv = "(" + f.text(f.decl.Recv.List[0]) + ") "
		callPrefix = f.decl.Recv.List[0].Names[0].Name + "."
	}
	stmts := f.decl.Body.List
	flows := chunkFlows(f, stmts, cuts)
	var body, helpers strings.Builder
	touched := []string{f.symbol}
	for i := 0; i < dose; i++ {
		chunk := stmts[cuts[i]:cuts[i+1]]
		fl := flows[i]
		params := []string{strings.TrimSuffix(strings.TrimPrefix(f.text(f.decl.Type.Params), "("), ")")}
		args := []string{forwardArgs(f)}
		for _, v := range fl.in {
			params = append(params, v.Name()+" "+types.TypeString(v.Type(), qual))
			args = append(args, v.Name())
		}
		call := fmt.Sprintf("%s%s(%s)", callPrefix, names[i], joinNonEmpty(args))
		var results []string
		text := string(f.src[f.offset(chunk[0].Pos()):f.offset(chunk[len(chunk)-1].End())])
		switch {
		case i == dose-1 && sig.Results().Len() > 0:
			body.WriteString("return " + call + "\n")
			results = []string{strings.TrimSuffix(strings.TrimPrefix(f.text(f.decl.Type.Results), "("), ")")}
		case len(fl.out) > 0:
			outs := make([]string, len(fl.out))
			for j, v := range fl.out {
				outs[j] = v.Name()
				results = append(results, types.TypeString(v.Type(), qual))
			}
			op := "="
			if fl.declares {
				op = ":="
			}
			fmt.Fprintf(&body, "%s %s %s\n", strings.Join(outs, ", "), op, call)
			text += "\nreturn " + strings.Join(outs, ", ")
		default:
			body.WriteString(call + "\n")
		}
		res := ""
		if len(results) > 0 {
			res = " (" + strings.Join(results, ", ") + ")"
		}
		fmt.Fprintf(&helpers, "\n\nfunc %s%s(%s)%s {\n%s\n}", recv, names[i], joinNonEmpty(params), res, text)
		touched = append(touched, symbolOf(f.pkg.PkgPath, &ast.FuncDecl{Recv: f.decl.Recv, Name: ast.NewIdent(names[i])}))
	}
	ss := []splice{
		{f.offset(f.decl.Body.Lbrace) + 1, f.offset(f.decl.Body.Rbrace), "\n" + body.String()},
		{f.offset(f.decl.End()), f.offset(f.decl.End()), helpers.String()},
	}
	out, err := finish(f.path, applySplices(f.src, ss))
	if err != nil {
		return harness.Applied{}, err
	}
	return harness.Applied{
		Edit:        harness.Edit{f.rel: out},
		Expectation: harness.Expectation{Touched: touched, Catches: o.catches},
	}, nil
}

func joinNonEmpty(parts []string) string {
	var out []string
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, ", ")
}

type flow struct {
	in, out  []*types.Var
	declares bool
}

func chunkFlows(f fn, stmts []ast.Stmt, cuts []int) []flow {
	scope := f.pkg.TypesInfo.Scopes[f.decl.Type]
	topLocal := func(obj types.Object) (*types.Var, bool) {
		v, ok := obj.(*types.Var)
		return v, ok && v.Parent() == scope && !isParam(f, v)
	}
	k := len(cuts) - 1
	defs := make([]map[*types.Var]bool, k)
	refs := make([]map[*types.Var]bool, k)
	for i := 0; i < k; i++ {
		defs[i], refs[i] = map[*types.Var]bool{}, map[*types.Var]bool{}
		for _, s := range stmts[cuts[i]:cuts[i+1]] {
			ast.Inspect(s, func(n ast.Node) bool {
				id, ok := n.(*ast.Ident)
				if !ok {
					return true
				}
				if v, ok := topLocal(f.pkg.TypesInfo.Defs[id]); ok {
					defs[i][v] = true
				}
				if v, ok := topLocal(f.pkg.TypesInfo.Uses[id]); ok {
					refs[i][v] = true
				}
				return true
			})
		}
	}
	flows := make([]flow, k)
	for i := 0; i < k; i++ {
		earlier := map[*types.Var]bool{}
		for j := 0; j < i; j++ {
			for v := range defs[j] {
				earlier[v] = true
			}
		}
		later := map[*types.Var]bool{}
		for j := i + 1; j < k; j++ {
			for v := range refs[j] {
				later[v] = true
			}
		}
		for v := range refs[i] {
			if earlier[v] {
				flows[i].in = append(flows[i].in, v)
			}
		}
		for v := range later {
			if defs[i][v] {
				flows[i].out = append(flows[i].out, v)
				flows[i].declares = true
			} else if earlier[v] && refs[i][v] {
				flows[i].out = append(flows[i].out, v)
			}
		}
		sortVars(flows[i].in)
		sortVars(flows[i].out)
	}
	return flows
}

func isParam(f fn, v *types.Var) bool {
	return v.Pos() < f.decl.Body.Lbrace
}

func sortVars(vs []*types.Var) {
	sort.Slice(vs, func(i, j int) bool { return vs[i].Pos() < vs[j].Pos() })
}

func splitPlan(f fn, k int) ([]int, error) {
	d := f.decl
	if d.Body == nil || f.obj == nil || d.Type.TypeParams != nil || !forwardable(f) {
		return nil, errors.New("not splittable")
	}
	sig := f.obj.Type().(*types.Signature)
	if sig.Results().Len() > 0 && k < 3 {
		return nil, errors.New("a parent with results needs 3 helpers for 2 cluster members")
	}
	if namedResults(d) || paramsMutated(f) {
		return nil, errors.New("named results or mutated params")
	}
	stmts := d.Body.List
	var valid []int
	for i := 1; i < len(stmts); i++ {
		if boundaryOK(stmts, i) {
			valid = append(valid, i)
		}
	}
	if len(valid) < k-1 {
		return nil, errors.New("not enough boundaries")
	}
	cuts := []int{0}
	for j := 1; j < k; j++ {
		ideal := j * len(stmts) / k
		best := -1
		for _, v := range valid {
			if v > cuts[len(cuts)-1] && (best < 0 || abs(v-ideal) < abs(best-ideal)) {
				best = v
			}
		}
		if best < 0 {
			return nil, errors.New("no boundary left")
		}
		cuts = append(cuts, best)
	}
	return append(cuts, len(stmts)), nil
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func forwardable(f fn) bool {
	if f.decl.Recv != nil {
		names := f.decl.Recv.List[0].Names
		if len(names) == 0 || names[0].Name == "_" {
			return false
		}
	}
	for _, p := range f.decl.Type.Params.List {
		if len(p.Names) == 0 {
			return false
		}
		for _, n := range p.Names {
			if n.Name == "_" {
				return false
			}
		}
	}
	return true
}

func namedResults(d *ast.FuncDecl) bool {
	if d.Type.Results == nil {
		return false
	}
	for _, r := range d.Type.Results.List {
		if len(r.Names) > 0 {
			return true
		}
	}
	return false
}

func paramsMutated(f fn) bool {
	sig := f.obj.Type().(*types.Signature)
	vars := []*types.Var{}
	if sig.Recv() != nil {
		vars = append(vars, sig.Recv())
	}
	for i := 0; i < sig.Params().Len(); i++ {
		vars = append(vars, sig.Params().At(i))
	}
	for _, v := range vars {
		if _, mutated := usage(f, v); mutated {
			return true
		}
	}
	return false
}

func boundaryOK(stmts []ast.Stmt, cut int) bool {
	bad := false
	for _, s := range stmts[:cut] {
		ast.Inspect(s, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.FuncLit:
				return false
			case *ast.ReturnStmt, *ast.LabeledStmt, *ast.DeferStmt:
				bad = true
			case *ast.BranchStmt:
				bad = bad || n.Label != nil
			}
			return !bad
		})
	}
	return !bad
}

func forwardArgs(f fn) string {
	var args []string
	params := f.decl.Type.Params.List
	for i, p := range params {
		for _, n := range p.Names {
			arg := n.Name
			if _, variadic := p.Type.(*ast.Ellipsis); variadic && i == len(params)-1 {
				arg += "..."
			}
			args = append(args, arg)
		}
	}
	return strings.Join(args, ", ")
}

func helperNames(f fn, k int) []string {
	base := []rune(f.decl.Name.Name)
	base[0] = unicode.ToLower(base[0])
	var recvType types.Type
	if sig := f.obj.Type().(*types.Signature); sig.Recv() != nil {
		recvType = sig.Recv().Type()
	}
	names := make([]string, k)
	for i := range names {
		for n := 0; ; n++ {
			name := fmt.Sprintf("%sStep%d", string(base), i+1)
			if n > 0 {
				name = fmt.Sprintf("%sStep%d_%d", string(base), i+1, n)
			}
			if !taken(f, recvType, name) {
				names[i] = name
				break
			}
		}
	}
	return names
}

func taken(f fn, recv types.Type, name string) bool {
	if recv != nil {
		obj, _, _ := types.LookupFieldOrMethod(recv, true, f.pkg.Types, name)
		return obj != nil
	}
	return f.pkg.Types.Scope().Lookup(name) != nil
}
