package operators

import (
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"slices"
	"sort"
	"strings"

	"github.com/KellyBennett/Columbo/calibration/harness"
)

// introduceParameterObject is the Introduce Parameter Object cure for a baseline Data Clump
// case. It reads the case's parameter receipts, declares a named struct for the clump next
// to the anchor declaration, and moves every supporting declaration and its call sites onto it.
type introduceParameterObject struct{}

func (introduceParameterObject) ID() string                                { return "introduce-parameter-object" }
func (introduceParameterObject) Class() harness.Class                      { return harness.Cure }
func (introduceParameterObject) Target() string                            { return "data-clump" }
func (introduceParameterObject) Doses(*harness.Corpus, harness.Site) []int { return []int{1} }

// Sites lists one site per baseline Data Clump case. Offset is the case's ordinal among the
// clump cases anchored on the same declaration, ordered by case ID.
func (o introduceParameterObject) Sites(c *harness.Corpus) []harness.Site {
	var out []harness.Site
	ordinal := map[string]int{}
	for _, f := range c.Baseline.Findings {
		if f.Smell != o.Target() || len(c.Baseline.Parameters[f.CaseID]) == 0 {
			continue
		}
		out = append(out, harness.Site{Package: pkgOfSymbol(f.Symbol), Symbol: f.Symbol, Offset: ordinal[f.Symbol], File: f.File})
		ordinal[f.Symbol]++
	}
	return out
}

func (o introduceParameterObject) Apply(c *harness.Corpus, site harness.Site, _ int) (harness.Applied, error) {
	var cases []string
	for _, f := range c.Baseline.Findings {
		if f.Smell == o.Target() && f.Symbol == site.Symbol && len(c.Baseline.Parameters[f.CaseID]) > 0 {
			cases = append(cases, f.CaseID)
		}
	}
	if site.Offset >= len(cases) {
		return harness.Applied{}, errors.New("case vanished")
	}
	p, err := programOf(c)
	if err != nil {
		return harness.Applied{}, err
	}
	cl, err := planClump(p, c.Baseline.Parameters[cases[site.Offset]])
	if err != nil {
		return harness.Applied{}, err
	}
	edit, err := cl.apply(p)
	if err != nil {
		return harness.Applied{}, err
	}
	var touched []string
	for _, m := range cl.members {
		touched = append(touched, m.f.symbol)
	}
	return harness.Applied{Edit: edit, Expectation: harness.Expectation{Touched: touched}}, nil
}

func pkgOfSymbol(symbol string) string {
	if i := strings.Index(symbol, ".("); i >= 0 {
		return symbol[:i]
	}
	slash := strings.LastIndex(symbol, "/")
	if i := strings.Index(symbol[slash+1:], "."); i >= 0 {
		return symbol[:slash+1+i]
	}
	return symbol
}

// clumpPlan is the parameter object for one case: its fields, and for each supporting
// declaration which of its parameters map to which field.
type clumpPlan struct {
	name     string
	exported bool
	fields   []clumpField
	members  []clumpMember
}

type clumpField struct {
	// key is the parameter name the field was named after, before export casing.
	name, typ, key string
}

type clumpMember struct {
	f fn
	// fieldOf maps a parameter index to the field it moves into.
	fieldOf map[int]int
	first   int
	param   string
}

func planClump(p *program, receipts []harness.ParameterReceipt) (clumpPlan, error) {
	var order []string
	bySymbol := map[string][]harness.ParameterReceipt{}
	for _, r := range receipts {
		if _, seen := bySymbol[r.Symbol]; !seen {
			order = append(order, r.Symbol)
		}
		bySymbol[r.Symbol] = append(bySymbol[r.Symbol], r)
	}
	var cl clumpPlan
	for mi, sym := range order {
		f, ok := p.fn(sym)
		if !ok {
			return clumpPlan{}, fmt.Errorf("%s not found", sym)
		}
		if err := p.rewritable(f); err != nil {
			return clumpPlan{}, fmt.Errorf("%s: %w", sym, err)
		}
		rs := bySymbol[sym]
		sort.Slice(rs, func(i, j int) bool { return rs[i].Index < rs[j].Index })
		ps := params(f)
		sig := f.obj.Type().(*types.Signature)
		m := clumpMember{f: f, fieldOf: map[int]int{}, first: rs[0].Index}
		taken := map[int]bool{}
		for _, r := range rs {
			if r.Index >= len(ps) || (sig.Variadic() && r.Index == len(ps)-1) {
				return clumpPlan{}, fmt.Errorf("%s: parameter %d is variadic or missing", sym, r.Index)
			}
			if mi == 0 {
				cl.fields = append(cl.fields, clumpField{typ: types.TypeString(ps[r.Index].v.Type(), qualifier(f.unit))})
				m.fieldOf[r.Index] = len(cl.fields) - 1
				continue
			}
			k := slotFor(bySymbol[order[0]], r.Type, taken)
			if k < 0 {
				return clumpPlan{}, fmt.Errorf("%s: type %s has no field", sym, r.Type)
			}
			taken[k] = true
			m.fieldOf[r.Index] = k
		}
		cl.members = append(cl.members, m)
	}
	cl.nameFields()
	cl.realign()
	cl.nameStruct()
	return cl, nil
}

// slotFor returns the first anchor receipt of the given type not yet taken.
func slotFor(anchor []harness.ParameterReceipt, typ string, taken map[int]bool) int {
	for k, r := range anchor {
		if r.Type == typ && !taken[k] {
			return k
		}
	}
	return -1
}

// paramName is the name a member gives the parameter it moves into field k, or "".
func (m clumpMember) paramName(k int) string {
	for idx, fk := range m.fieldOf {
		if q := params(m.f)[idx]; fk == k && q.named() {
			return q.name.Name
		}
	}
	return ""
}

// nameFields names each field by the parameter name most members use for it, so the
// object reads in the members' own vocabulary.
func (cl *clumpPlan) nameFields() {
	for _, m := range cl.members {
		cl.exported = cl.exported || m.f.decl.Name.IsExported()
	}
	used := map[string]bool{}
	for k := range cl.fields {
		votes := map[string]int{}
		name := ""
		for _, m := range cl.members {
			if n := m.paramName(k); n != "" {
				votes[n]++
				if name == "" || votes[n] > votes[name] {
					name = n
				}
			}
		}
		cl.fields[k].key = name
		if name == "" {
			name = fmt.Sprintf("v%d", k)
		}
		if cl.exported {
			name = upperFirst(name)
		}
		cl.fields[k].name = fresh(name, used)
	}
}

// realign fixes a member whose receipts picked a same-typed parameter other than the one
// named like the field, as Columbo selects the first matching parameters in order.
func (cl *clumpPlan) realign() {
	for mi := range cl.members {
		m := &cl.members[mi]
		ps := params(m.f)
		for idx, k := range m.fieldOf {
			want := cl.fields[k].key
			if want == "" || (ps[idx].named() && ps[idx].name.Name == want) {
				continue
			}
			for j, q := range ps {
				_, mapped := m.fieldOf[j]
				if !mapped && q.named() && q.name.Name == want && canonical(q.v.Type()) == canonical(ps[idx].v.Type()) {
					delete(m.fieldOf, idx)
					m.fieldOf[j] = k
					break
				}
			}
		}
		m.first = len(ps)
		for idx := range m.fieldOf {
			m.first = min(m.first, idx)
		}
	}
}

// nameStruct names the object after its fields and each member's new parameter after it.
func (cl *clumpPlan) nameStruct() {
	var b strings.Builder
	for _, f := range cl.fields {
		b.WriteString(upperFirst(f.name))
	}
	base := b.String()
	if !cl.exported {
		base = lowerFirst(base)
	}
	scope := cl.members[0].f.pkg.Types.Scope()
	cl.name = base
	for n := 2; scope.Lookup(cl.name) != nil || token.IsKeyword(cl.name); n++ {
		cl.name = fmt.Sprintf("%s%d", base, n)
	}
	for i := range cl.members {
		used := usedNames(cl.members[i].f.decl)
		used[cl.name] = true
		pname := lowerFirst(cl.name)
		if pname == cl.name {
			pname = "p"
		}
		cl.members[i].param = fresh(pname, used)
	}
}

func (cl clumpPlan) apply(p *program) (harness.Edit, error) {
	r := newRewrite()
	anchor := cl.members[0].f
	var decl strings.Builder
	fmt.Fprintf(&decl, "type %s struct {\n", cl.name)
	for _, f := range cl.fields {
		fmt.Fprintf(&decl, "%s %s\n", f.name, f.typ)
	}
	decl.WriteString("}\n\n")
	at := anchor.offset(anchor.decl.Pos())
	if anchor.decl.Doc != nil {
		at = anchor.offset(anchor.decl.Doc.Pos())
	}
	r.add(anchor.unit, at, at, decl.String())
	var calls []callSite
	byKey := map[string]clumpMember{}
	for _, m := range cl.members {
		f := m.f
		ps := params(f)
		moved := map[*types.Var]string{}
		for idx, k := range m.fieldOf {
			moved[ps[idx].v] = m.param + "." + cl.fields[k].name
		}
		ast.Inspect(f.decl.Body, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok {
				if v, ok := f.pkg.TypesInfo.Uses[id].(*types.Var); ok && moved[v] != "" {
					r.add(f.unit, f.offset(id.Pos()), f.offset(id.End()), moved[v])
				}
			}
			return true
		})
		unnamed := ps[0].name == nil
		var list []string
		for i, q := range ps {
			_, isMoved := m.fieldOf[i]
			switch {
			case i == m.first && unnamed:
				list = append(list, cl.name)
			case i == m.first:
				list = append(list, m.param+" "+cl.name)
			case !isMoved:
				list = append(list, paramText(f, q, unnamed))
			}
		}
		replaceParams(r, f, list)
		k := funcKey(f.obj)
		byKey[k] = m
		calls = append(calls, p.calls[k]...)
	}
	sortCallsInnermostFirst(calls)
	for _, cs := range calls {
		obj := cs.pkg.TypesInfo.Uses[calleeIdent(cs.call.Fun)].(*types.Func)
		m := byKey[funcKey(obj)]
		args, err := cl.callArgs(r, cs, m)
		if err != nil {
			return nil, err
		}
		replaceArgs(r, cs, args)
	}
	return r.edit()
}

// callArgs packs the moved arguments into one keyed literal at the first moved position.
// Keys follow argument order, so evaluation order is unchanged unless an unmoved argument
// sits between moved ones, which then must be free of effects.
func (cl clumpPlan) callArgs(r *rewrite, cs callSite, m clumpMember) ([]string, error) {
	qual, ok := qualifierFor(cs.unit, m.f.pkg.PkgPath)
	if !ok || (qual != "" && !cl.exported) {
		return nil, errors.New("call site cannot name the parameter object")
	}
	last := 0
	for idx := range m.fieldOf {
		last = max(last, idx)
	}
	var out, kv []string
	for i, a := range cs.call.Args {
		k, isMoved := m.fieldOf[i]
		text := r.nodeText(cs.unit, a)
		switch {
		case isMoved:
			kv = append(kv, cl.fields[k].name+": "+text)
		case i > m.first && i < last && !pure(a):
			return nil, errors.New("argument with effects between moved arguments")
		default:
			out = append(out, text)
		}
		if i == last {
			lit := qual + cl.name + "{" + strings.Join(kv, ", ") + "}"
			if whole, ok := cl.forwarded(cs, m); ok {
				lit = whole
			}
			out = slices.Insert(out, m.first, lit)
		}
	}
	return out, nil
}

// forwarded reports the enclosing member's own parameter object when a call inside that
// member passes exactly its moved parameters, each into the same field.
func (cl clumpPlan) forwarded(cs callSite, m clumpMember) (string, bool) {
	for _, outer := range cl.members {
		if outer.f.rel != cs.rel || cs.call.Pos() < outer.f.decl.Pos() || cs.call.End() > outer.f.decl.End() {
			continue
		}
		ps := params(outer.f)
		for idx, k := range m.fieldOf {
			id, ok := ast.Unparen(cs.call.Args[idx]).(*ast.Ident)
			if !ok {
				return "", false
			}
			match := false
			for j, fk := range outer.fieldOf {
				match = match || (fk == k && cs.pkg.TypesInfo.Uses[id] == ps[j].v)
			}
			if !match {
				return "", false
			}
		}
		return outer.param, true
	}
	return "", false
}
