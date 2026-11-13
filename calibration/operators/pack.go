package operators

import (
	"errors"
	"go/ast"
	"go/types"
	"slices"
	"strings"

	"github.com/KellyBennett/Columbo/calibration/harness"
)

// clumpEvade rewrites the parameter run an inline-parameter-object mutant left behind:
// a consecutive run whose types match a same-package struct's fields in order. A site is
// the struct type, and every function in its package that carries the run is rewritten.
type clumpEvade struct {
	id, follows, target string
	catches             []string
	rewrite             func(r *rewrite, p *program, run paramRun) (argMap, error)
}

// argMap respells one call's argument texts for a rewritten signature.
type argMap func(cs clumpCallSite, args []string) []string

func packAnonymousStruct(id, follows, target string) clumpEvade {
	return clumpEvade{id: id, follows: follows, target: target, catches: []string{"data-clump", "long-parameter-list"}, rewrite: packRun}
}

func reorderParams(id, follows, target string) clumpEvade {
	return clumpEvade{id: id, follows: follows, target: target, catches: []string{"data-clump", "long-parameter-list"}, rewrite: reverseRun}
}

func (o clumpEvade) ID() string                                { return o.id }
func (o clumpEvade) Class() harness.Class                      { return harness.Evade }
func (o clumpEvade) Target() string                            { return o.target }
func (o clumpEvade) Follows() string                           { return o.follows }
func (o clumpEvade) Doses(*harness.Corpus, harness.Site) []int { return []int{1} }

func (o clumpEvade) Sites(c *harness.Corpus) []harness.Site {
	p, err := programOf(c)
	if err != nil {
		return nil
	}
	var out []harness.Site
	for _, st := range paramStructs(p, int(threshold(c, "data-clump-size", 3))) {
		if len(paramRuns(p, st)) > 0 {
			out = append(out, st.site())
		}
	}
	return out
}

func (o clumpEvade) Apply(c *harness.Corpus, site harness.Site, _ int) (harness.Applied, error) {
	p, err := programOf(c)
	if err != nil {
		return harness.Applied{}, err
	}
	st, ok := findStruct(p, site.Symbol)
	if !ok {
		return harness.Applied{}, errors.New("site vanished")
	}
	touched := append([]string{st.symbol}, support(p, st.pkgPath(), st.fieldTypes(), nil).symbols...)
	r := newRewrite()
	changed := 0
	var calls []clumpCallSite
	maps := map[string]argMap{}
	for _, run := range paramRuns(p, st) {
		m, err := o.rewrite(r, p, run)
		if err != nil {
			continue
		}
		changed++
		maps[funcKey(run.f.obj)] = m
		calls = append(calls, p.calls[funcKey(run.f.obj)]...)
		touched = append(touched, run.f.symbol)
	}
	if changed == 0 {
		return harness.Applied{}, errors.New("no run could be rewritten")
	}
	sortCallsInnermostFirst(calls)
	for _, cs := range calls {
		obj := cs.pkg.TypesInfo.Uses[calleeIdent(cs.call.Fun)].(*types.Func)
		replaceArgs(r, cs, maps[funcKey(obj)](cs, argTexts(r, cs)))
	}
	edit, err := r.edit()
	if err != nil {
		return harness.Applied{}, err
	}
	return harness.Applied{Edit: edit, Expectation: harness.Expectation{Touched: dedupe(touched), Catches: o.catches}}, nil
}

// paramRun is a run of consecutive named parameters whose types match a struct's fields
// in order, which is the shape inline-parameter-object leaves.
type paramRun struct {
	f        fn
	start, n int
}

func paramRuns(p *program, st paramStruct) []paramRun {
	want := st.fieldTypes()
	var out []paramRun
	for _, f := range p.funcs() {
		if f.pkg.PkgPath != st.pkgPath() || p.rewritable(f) != nil {
			continue
		}
		ps := params(f)
		ts := paramTypes(f)
		sig := f.obj.Type().(*types.Signature)
		for i := 0; i+len(want) <= len(ts); i++ {
			if !slices.Equal(ts[i:i+len(want)], want) || (sig.Variadic() && i+len(want) == len(ts)) {
				continue
			}
			named := true
			for _, q := range ps[i : i+len(want)] {
				named = named && q.named()
			}
			if named {
				out = append(out, paramRun{f: f, start: i, n: len(want)})
			}
			break
		}
	}
	return out
}

func (run paramRun) params() []clumpParam { return params(run.f)[run.start : run.start+run.n] }

// packRun replaces the run with one anonymous struct parameter. Anonymous struct types
// with unexported field names differ across packages, so every caller must share the
// function's package.
func packRun(r *rewrite, p *program, run paramRun) (argMap, error) {
	f := run.f
	for _, cs := range p.calls[funcKey(f.obj)] {
		if cs.pkg.PkgPath != f.pkg.PkgPath {
			return nil, errors.New("caller outside the package")
		}
	}
	name := clumpFresh("p", usedNames(f.decl))
	qp := run.params()
	vars := map[*types.Var]string{}
	anon := func(u unit) string {
		qual := qualifier(u)
		fields := make([]string, len(qp))
		for i, q := range qp {
			fields[i] = q.name.Name + " " + types.TypeString(q.v.Type(), qual)
		}
		return "struct{ " + strings.Join(fields, "; ") + " }"
	}
	for _, q := range qp {
		vars[q.v] = name + "." + q.name.Name
	}
	ast.Inspect(f.decl.Body, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok {
			if v, ok := f.pkg.TypesInfo.Uses[id].(*types.Var); ok && vars[v] != "" {
				r.add(f.unit, f.offset(id.Pos()), f.offset(id.End()), vars[v])
			}
		}
		return true
	})
	var list []string
	for i, q := range params(f) {
		switch {
		case i == run.start:
			list = append(list, name+" "+anon(f.unit))
		case i > run.start && i < run.start+run.n:
		default:
			list = append(list, paramText(f, q, false))
		}
	}
	replaceParams(r, f, list)
	return func(cs clumpCallSite, args []string) []string {
		packed := anon(cs.unit) + "{" + strings.Join(args[run.start:run.start+run.n], ", ") + "}"
		return append(append(append([]string{}, args[:run.start]...), packed), args[run.start+run.n:]...)
	}, nil
}

// reverseRun reverses the run in the signature and at every call site. Reversing moves
// argument evaluation, so every argument in the run must be free of effects.
func reverseRun(r *rewrite, p *program, run paramRun) (argMap, error) {
	f := run.f
	for _, cs := range p.calls[funcKey(f.obj)] {
		for _, a := range cs.call.Args[run.start : run.start+run.n] {
			if !pure(a) {
				return nil, errors.New("argument with effects in the run")
			}
		}
	}
	ps := params(f)
	var list []string
	for i, q := range ps {
		if i >= run.start && i < run.start+run.n {
			q = ps[run.start+run.start+run.n-1-i]
		}
		list = append(list, paramText(f, q, false))
	}
	replaceParams(r, f, list)
	return func(_ clumpCallSite, args []string) []string {
		out := append([]string{}, args...)
		slices.Reverse(out[run.start : run.start+run.n])
		return out
	}, nil
}

func dedupe(ss []string) []string {
	slices.Sort(ss)
	return slices.Compact(ss)
}
