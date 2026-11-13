package operators

import (
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"github.com/KellyBennett/Columbo/calibration/harness"
)

// Neither hoisting style moves behavior, so Columbo should still see the envy.
type hoistFields struct {
	id, follows string
	getters     bool
}

func (o hoistFields) ID() string           { return o.id }
func (o hoistFields) Class() harness.Class { return harness.Evade }
func (o hoistFields) Target() string       { return envySmell }
func (o hoistFields) Follows() string      { return o.follows }

func (o hoistFields) Sites(c *harness.Corpus) []harness.Site {
	var out []harness.Site
	for _, f := range funcs(c) {
		if _, err := o.plan(c, f); err == nil {
			out = append(out, f.site())
		}
	}
	return out
}

func (o hoistFields) Doses(*harness.Corpus, harness.Site) []int { return []int{1} }

func (o hoistFields) Apply(c *harness.Corpus, site harness.Site, _ int) (harness.Applied, error) {
	f, ok := findFn(c, site.Symbol)
	if !ok {
		return harness.Applied{}, errors.New("site vanished")
	}
	h, err := o.plan(c, f)
	if err != nil {
		return harness.Applied{}, err
	}
	var ss []splice
	var top, getters strings.Builder
	root := h.group.root.Name()
	replaced := 0
	for _, fld := range h.fields {
		read := root + "." + fld.obj.Name()
		with := fld.local
		if o.getters {
			with = root + "." + fld.getter + "()"
			fmt.Fprintf(&getters, "\n\nfunc (%s) %s() %s {\nreturn r.%s\n}", h.getterRecv(), fld.getter, types.TypeString(fld.obj.Type(), qualifier(f.unit)), fld.obj.Name())
		} else {
			fmt.Fprintf(&top, "%s := %s\n", fld.local, read)
		}
		for _, use := range fld.uses {
			ss = append(ss, splice{f.offset(use.Pos()), f.offset(use.End()), with})
			if !inFuncLit(use, h.parents) {
				replaced++
			}
		}
	}
	ss = append(ss,
		splice{f.offset(f.decl.Body.Lbrace) + 1, f.offset(f.decl.Body.Lbrace) + 1, "\n" + top.String()},
		splice{f.offset(f.decl.End()), f.offset(f.decl.End()), getters.String()})
	out, err := finish(f.path, applySplices(f.src, ss))
	if err != nil {
		return harness.Applied{}, err
	}
	count := h.group.count
	if !o.getters {
		count += len(h.fields) - replaced
	}
	l := limitsOf(c)
	caught := l.qualifies(&accessGroup{count: count}, h.own)
	return harness.Applied{
		Edit: harness.Edit{f.rel: out},
		Expectation: harness.Expectation{
			Touched: []string{f.symbol},
			Catches: []string{envySmell},
			Prediction: harness.Prediction{
				Kill:   &caught,
				Reason: fmt.Sprintf("%s keeps %d counted accesses after rewriting %d counted reads of %d fields (limit %d)", root, count, replaced, len(h.fields), l.foreign),
			},
		},
	}, nil
}

func (o hoistFields) plan(c *harness.Corpus, f fn) (hoistPlan, error) {
	h, err := planHoist(c, f)
	if err == nil && o.getters && h.fields[0].getter == "" {
		err = errors.New("envied type is not local, so it cannot take getters")
	}
	return h, err
}

type hoistPlan struct {
	f       fn
	group   *accessGroup
	own     int
	fields  []*hoistField
	parents map[ast.Node]ast.Node
}

type hoistField struct {
	obj           *types.Var
	local, getter string
	uses          []*ast.SelectorExpr
}

func (h hoistPlan) getterRecv() string {
	return "r " + types.TypeString(h.group.root.Type(), qualifier(h.f.unit))
}

func planHoist(c *harness.Corpus, f fn) (hoistPlan, error) {
	if f.decl.Body == nil || f.obj == nil {
		return hoistPlan{}, errors.New("no body")
	}
	p := profileOf(f.pkg.TypesInfo, f.decl.Body, recvVarOf(f))
	groups := p.qualifying(limitsOf(c))
	sortedGroups(groups)
	sig := f.obj.Type().(*types.Signature)
	parents := parentMap(f.decl.Body)
	for _, g := range groups {
		if !g.bare() || !isSigParam(sig, g.root) {
			continue
		}
		fields, err := hoistable(f, g.root, parents)
		if err != nil || len(fields) == 0 {
			continue
		}
		h := hoistPlan{f: f, group: g, own: p.own, fields: fields, parents: parents}
		h.name()
		return h, nil
	}
	return hoistPlan{}, errors.New("no hoistable envied parameter")
}

// inFuncLit mirrors Columbo skipping nested function literals when it counts accesses.
func inFuncLit(n ast.Node, parents map[ast.Node]ast.Node) bool {
	for p := parents[n]; p != nil; p = parents[p] {
		if _, ok := p.(*ast.FuncLit); ok {
			return true
		}
	}
	return false
}

func isSigParam(sig *types.Signature, v *types.Var) bool {
	for i := 0; i < sig.Params().Len(); i++ {
		if sig.Params().At(i) == v {
			return true
		}
	}
	return false
}

func parentMap(root ast.Node) map[ast.Node]ast.Node {
	parents := map[ast.Node]ast.Node{}
	var stack []ast.Node
	ast.Inspect(root, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return false
		}
		if len(stack) > 0 {
			parents[n] = stack[len(stack)-1]
		}
		stack = append(stack, n)
		return true
	})
	return parents
}

func hoistable(f fn, root *types.Var, parents map[ast.Node]ast.Node) ([]*hoistField, error) {
	info := f.pkg.TypesInfo
	var order []*hoistField
	byObj := map[types.Object]*hoistField{}
	tainted := map[types.Object]bool{}
	var bad error
	ast.Inspect(f.decl.Body, func(n ast.Node) bool {
		id, ok := n.(*ast.Ident)
		if !ok || info.Uses[id] != root || bad != nil {
			return bad == nil
		}
		s, ok := parents[id].(*ast.SelectorExpr)
		if !ok || s.X != id {
			bad = errors.New("parameter used other than as a selector base")
			return false
		}
		sel := info.Selections[s]
		switch {
		case sel == nil:
			bad = errors.New("unresolved selector")
		case sel.Kind() == types.MethodVal:
			if isPointer(sel.Obj().Type().(*types.Signature).Recv().Type()) {
				bad = errors.New("pointer-receiver method may write fields")
			}
		case sel.Kind() == types.FieldVal:
			obj := sel.Obj()
			h := byObj[obj]
			if h == nil {
				h = &hoistField{obj: obj.(*types.Var)}
				byObj[obj] = h
				order = append(order, h)
			}
			h.uses = append(h.uses, s)
			if written(info, s, parents) {
				tainted[obj] = true
			}
		}
		return bad == nil
	})
	if bad != nil {
		return nil, bad
	}
	var out []*hoistField
	for _, h := range order {
		if !tainted[h.obj] {
			out = append(out, h)
		}
	}
	return out, nil
}

func written(info *types.Info, s *ast.SelectorExpr, parents map[ast.Node]ast.Node) bool {
	var cur ast.Node = s
	for {
		switch p := parents[cur].(type) {
		case *ast.SelectorExpr:
			if p.X != cur {
				return false
			}
			if sel := info.Selections[p]; sel == nil || sel.Kind() != types.FieldVal {
				return true
			}
		case *ast.IndexExpr:
			if p.X != cur {
				return false
			}
		case *ast.ParenExpr, *ast.StarExpr:
		case *ast.SliceExpr:
			return p.X == cur
		case *ast.UnaryExpr:
			return p.Op == token.AND
		case *ast.IncDecStmt:
			return true
		case *ast.AssignStmt:
			for _, l := range p.Lhs {
				if l == cur {
					return true
				}
			}
			return false
		case *ast.RangeStmt:
			return p.Key == cur || p.Value == cur
		default:
			return false
		}
		cur = parents[cur]
	}
}

func (h *hoistPlan) name() {
	taken := map[string]bool{}
	ast.Inspect(h.f.decl, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok {
			taken[id.Name] = true
		}
		return true
	})
	t, ok := localNamed(h.group.root.Type(), h.f.pkg.Types)
	for _, fld := range h.fields {
		fld.local = fresh(lowerFirst(fld.obj.Name()), func(n string) bool {
			return taken[n] || token.Lookup(n).IsKeyword() || types.Universe.Lookup(n) != nil
		})
		taken[fld.local] = true
		if !ok {
			continue
		}
		fld.getter = fresh("get"+upperFirst(fld.obj.Name()), func(n string) bool {
			obj, _, _ := types.LookupFieldOrMethod(t, true, h.f.pkg.Types, n)
			return obj != nil || taken[n]
		})
		taken[fld.getter] = true
	}
}

func fresh(base string, taken func(string) bool) string {
	name := base
	for i := 2; taken(name); i++ {
		name = fmt.Sprintf("%s%d", base, i)
	}
	return name
}
