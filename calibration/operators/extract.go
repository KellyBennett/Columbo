package operators

import (
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"sort"
	"strings"

	"github.com/KellyBennett/Columbo/calibration/harness"
)

// Runs are chosen for the narrowest interface, the closest mechanical proxy for a responsibility seam.
type extractFunction struct {
	id, target string
}

func (o extractFunction) ID() string           { return o.id }
func (o extractFunction) Class() harness.Class { return harness.Cure }
func (o extractFunction) Target() string       { return o.target }

func (o extractFunction) Sites(c *harness.Corpus) []harness.Site {
	var out []harness.Site
	for _, f := range funcs(c) {
		if f.decl.Body != nil && f.obj != nil && f.decl.Type.TypeParams == nil && f.baselineHas(c, o.target, f.symbol) && len(segments(f)) > 0 {
			out = append(out, f.site())
		}
	}
	return out
}

func (o extractFunction) Doses(*harness.Corpus, harness.Site) []int { return []int{1, 2, 3} }

func (o extractFunction) Apply(c *harness.Corpus, site harness.Site, dose int) (harness.Applied, error) {
	f, ok := findFn(c, site.Symbol)
	if !ok {
		return harness.Applied{}, errors.New("site vanished")
	}
	chosen := chooseSegments(f, dose, o.target, c)
	if len(chosen) < dose {
		return harness.Applied{}, fmt.Errorf("only %d disjoint segments", len(chosen))
	}
	names := extractNames(f, dose)
	qual := qualifier(f.unit)
	var ss []splice
	var helpers strings.Builder
	touched := []string{f.symbol}
	for i, seg := range chosen {
		call, helper := seg.render(f, names[i], qual)
		ss = append(ss, splice{f.offset(seg.stmts[0].Pos()), f.offset(seg.stmts[len(seg.stmts)-1].End()), call})
		helpers.WriteString(helper)
		touched = append(touched, symbolOf(f.pkg.PkgPath, &ast.FuncDecl{Recv: seg.recvField(f), Name: ast.NewIdent(names[i])}))
	}
	ss = append(ss, splice{f.offset(f.decl.End()), f.offset(f.decl.End()), helpers.String()})
	out, err := finish(f.path, applySplices(f.src, ss))
	if err != nil {
		return harness.Applied{}, err
	}
	kill, reason, err := cureForecast(c, out, f, names, chosen, o.target)
	if err != nil {
		return harness.Applied{}, err
	}
	return harness.Applied{
		Edit: harness.Edit{f.rel: out},
		Expectation: harness.Expectation{
			Touched:    touched,
			Prediction: harness.Prediction{Kill: &kill, Reason: reason},
		},
	}, nil
}

func cureForecast(c *harness.Corpus, out []byte, f fn, names []string, segs []segment, target string) (bool, string, error) {
	lineLimit := int(threshold(c, "function-lines", 10))
	ccLimit := int(threshold(c, "cognitive-complexity", 7))
	lines, cc, err := metricsOf(out, f.decl.Name.Name, f.decl.Recv != nil)
	if err != nil {
		return false, "", err
	}
	parts := []string{fmt.Sprintf("parent %d lines cc %d", lines, cc)}
	ok := lines <= lineLimit && cc <= ccLimit
	for i, n := range names {
		hl, hc, err := metricsOf(out, n, segs[i].usesRecv)
		if err != nil {
			return false, "", err
		}
		parts = append(parts, fmt.Sprintf("%s %d lines cc %d", n, hl, hc))
		ok = ok && hl <= lineLimit && hc <= ccLimit
	}
	helpersFit := true
	for i, n := range names {
		hl, hc, _ := metricsOf(out, n, segs[i].usesRecv)
		if target == "long-function" && hl > lineLimit || target == "high-cognitive-complexity" && hc > ccLimit {
			helpersFit = false
		}
	}
	if target == "long-function" {
		ok = lines <= lineLimit && helpersFit
	} else if target == "high-cognitive-complexity" {
		ok = cc <= ccLimit && helpersFit
	}
	return ok, strings.Join(parts, "; ") + fmt.Sprintf(" (limits %d lines, cc %d)", lineLimit, ccLimit), nil
}

type segment struct {
	stmts    []ast.Stmt
	block    []ast.Stmt
	in, out  []*types.Var
	declares bool
	suffix   bool
	usesRecv bool
	lines    int
	width    int
}

func (s segment) recvField(f fn) *ast.FieldList {
	if s.usesRecv {
		return f.decl.Recv
	}
	return nil
}

func (s segment) render(f fn, name string, qual types.Qualifier) (call, helper string) {
	sig := f.obj.Type().(*types.Signature)
	var params, args []string
	for _, v := range s.in {
		params = append(params, v.Name()+" "+types.TypeString(v.Type(), qual))
		args = append(args, v.Name())
	}
	prefix, recv := "", ""
	if s.usesRecv {
		recv = "(" + f.text(f.decl.Recv.List[0]) + ") "
		prefix = f.decl.Recv.List[0].Names[0].Name + "."
	}
	invoke := fmt.Sprintf("%s%s(%s)", prefix, name, strings.Join(args, ", "))
	body := string(f.src[f.offset(s.stmts[0].Pos()):f.offset(s.stmts[len(s.stmts)-1].End())])
	var results []string
	switch {
	case s.suffix && sig.Results().Len() > 0:
		call = "return " + invoke
		results = []string{strings.TrimSuffix(strings.TrimPrefix(f.text(f.decl.Type.Results), "("), ")")}
	case len(s.out) > 0:
		outs := make([]string, len(s.out))
		for i, v := range s.out {
			outs[i] = v.Name()
			results = append(results, types.TypeString(v.Type(), qual))
		}
		op := "="
		if s.declares {
			op = ":="
		}
		call = fmt.Sprintf("%s %s %s", strings.Join(outs, ", "), op, invoke)
		body += "\nreturn " + strings.Join(outs, ", ")
	default:
		call = invoke
	}
	res := ""
	if len(results) > 0 {
		res = " (" + strings.Join(results, ", ") + ")"
	}
	helper = fmt.Sprintf("\n\nfunc %s%s(%s)%s {\n%s\n}", recv, name, strings.Join(params, ", "), res, body)
	return call, helper
}

func chooseSegments(f fn, k int, target string, c *harness.Corpus) []segment {
	all := segments(f)
	lineLimit := int(threshold(c, "function-lines", 10))
	sort.SliceStable(all, func(i, j int) bool {
		a, b := all[i], all[j]
		aFits, bFits := a.lines <= lineLimit, b.lines <= lineLimit
		if aFits != bFits {
			return aFits
		}
		if a.width != b.width {
			return a.width < b.width
		}
		if a.lines != b.lines {
			return a.lines > b.lines
		}
		return a.stmts[0].Pos() < b.stmts[0].Pos()
	})
	var chosen []segment
	for _, s := range all {
		if len(chosen) == k {
			break
		}
		if !overlapsAny(s, chosen) {
			chosen = append(chosen, s)
		}
	}
	sort.Slice(chosen, func(i, j int) bool { return chosen[i].stmts[0].Pos() < chosen[j].stmts[0].Pos() })
	return chosen
}

func overlapsAny(s segment, others []segment) bool {
	for _, o := range others {
		if s.stmts[0].Pos() <= o.stmts[len(o.stmts)-1].End() && o.stmts[0].Pos() <= s.stmts[len(s.stmts)-1].End() {
			return true
		}
	}
	return false
}

const minSegmentLines = 3

func segments(f fn) []segment {
	if namedResults(f.decl) {
		return nil
	}
	var out []segment
	var visit func(list []ast.Stmt, top bool)
	visit = func(list []ast.Stmt, top bool) {
		for i := range list {
			for j := i + 1; j <= len(list); j++ {
				if s, ok := makeSegment(f, list, i, j, top); ok {
					out = append(out, s)
				}
			}
		}
		for _, s := range list {
			for _, child := range childLists(s) {
				visit(child, false)
			}
		}
	}
	visit(f.decl.Body.List, true)
	return out
}

func childLists(s ast.Stmt) [][]ast.Stmt {
	var out [][]ast.Stmt
	switch s := s.(type) {
	case *ast.BlockStmt:
		out = append(out, s.List)
	case *ast.IfStmt:
		out = append(out, s.Body.List)
		if e, ok := s.Else.(*ast.BlockStmt); ok {
			out = append(out, e.List)
		} else if e, ok := s.Else.(*ast.IfStmt); ok {
			out = append(out, childLists(e)...)
		}
	case *ast.ForStmt:
		out = append(out, s.Body.List)
	case *ast.RangeStmt:
		out = append(out, s.Body.List)
	case *ast.SwitchStmt:
		for _, cc := range s.Body.List {
			out = append(out, cc.(*ast.CaseClause).Body)
		}
	case *ast.TypeSwitchStmt:
		for _, cc := range s.Body.List {
			out = append(out, cc.(*ast.CaseClause).Body)
		}
	}
	return out
}

func makeSegment(f fn, list []ast.Stmt, i, j int, top bool) (segment, bool) {
	stmts := list[i:j]
	fset := f.pkg.Fset
	lines := fset.Position(stmts[len(stmts)-1].End()).Line - fset.Position(stmts[0].Pos()).Line + 1
	if lines < minSegmentLines || (top && i == 0 && j == len(list)) {
		return segment{}, false
	}
	suffix := top && j == len(list)
	if !movable(stmts, suffix) {
		return segment{}, false
	}
	seg := segment{stmts: stmts, block: list, suffix: suffix, lines: lines}
	start, end := stmts[0].Pos(), stmts[len(stmts)-1].End()
	recv := recvVar(f)
	in, out := map[*types.Var]bool{}, map[*types.Var]bool{}
	ok := true
	ast.Inspect(f.decl.Body, func(n ast.Node) bool {
		id, isIdent := n.(*ast.Ident)
		if !isIdent {
			return true
		}
		v, isVar := f.pkg.TypesInfo.Uses[id].(*types.Var)
		if !isVar || v.IsField() || v.Pkg() != f.pkg.Types || v.Parent() == f.pkg.Types.Scope() {
			return true
		}
		inside := id.Pos() >= start && id.Pos() < end
		declaredInside := v.Pos() >= start && v.Pos() < end
		switch {
		case inside && !declaredInside:
			if v == recv {
				seg.usesRecv = true
			} else {
				in[v] = true
			}
		case !inside && declaredInside && id.Pos() >= end:
			out[v] = true
			seg.declares = true
		case !inside && !declaredInside && id.Pos() >= end && assignedWithin(f, v, start, end):
			out[v] = true
		}
		return true
	})
	ast.Inspect(&ast.BlockStmt{List: stmts}, func(n ast.Node) bool {
		if u, isUnary := n.(*ast.UnaryExpr); isUnary && u.Op == token.AND {
			if id, isIdent := u.X.(*ast.Ident); isIdent {
				if v, isVar := f.pkg.TypesInfo.Uses[id].(*types.Var); isVar && v.Pos() < start {
					ok = false
				}
			}
		}
		return ok
	})
	if !ok || (seg.usesRecv && recv == nil) {
		return segment{}, false
	}
	if seg.usesRecv && assignedWithin(f, recv, start, end) {
		return segment{}, false
	}
	for v := range out {
		sameScope := v.Pos() >= list[0].Pos() || (top && v.Pos() < f.decl.Body.Lbrace)
		if seg.declares && v.Pos() < start && !sameScope {
			return segment{}, false
		}
	}
	if suffix && len(out) > 0 {
		return segment{}, false
	}
	for v := range in {
		seg.in = append(seg.in, v)
	}
	for v := range out {
		seg.out = append(seg.out, v)
	}
	sortVars(seg.in)
	sortVars(seg.out)
	seg.width = len(seg.in) + len(seg.out)
	return seg, true
}

func assignedWithin(f fn, v *types.Var, start, end token.Pos) bool {
	found := false
	ast.Inspect(f.decl.Body, func(n ast.Node) bool {
		if n == nil || found || n.End() <= start || n.Pos() >= end {
			return false
		}
		switch n := n.(type) {
		case *ast.AssignStmt:
			for _, l := range n.Lhs {
				found = found || (n.Pos() >= start && refersTo(f, l, v))
			}
		case *ast.IncDecStmt:
			found = found || (n.Pos() >= start && refersTo(f, n.X, v))
		case *ast.RangeStmt:
			found = found || (n.Pos() >= start && (refersTo(f, n.Key, v) || refersTo(f, n.Value, v)))
		}
		return true
	})
	return found
}

func movable(stmts []ast.Stmt, suffix bool) bool {
	ok := true
	var walk func(n ast.Node, loop int)
	walk = func(n ast.Node, loop int) {
		ast.Inspect(n, func(x ast.Node) bool {
			if !ok || x == nil {
				return false
			}
			switch x := x.(type) {
			case *ast.FuncLit:
				return false
			case *ast.ReturnStmt:
				ok = ok && suffix
			case *ast.DeferStmt, *ast.LabeledStmt, *ast.GoStmt:
				ok = false
			case *ast.BranchStmt:
				ok = ok && x.Label == nil && x.Tok != token.GOTO && (loop > 0 || x.Tok == token.FALLTHROUGH)
			case *ast.ForStmt:
				walk(x.Body, loop+1)
				return false
			case *ast.RangeStmt:
				walk(x.Body, loop+1)
				return false
			case *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
				var body *ast.BlockStmt
				switch s := x.(type) {
				case *ast.SwitchStmt:
					body = s.Body
				case *ast.TypeSwitchStmt:
					body = s.Body
				case *ast.SelectStmt:
					body = s.Body
				}
				for _, cl := range body.List {
					walkBreakable(cl, loop, walk, &ok)
				}
				return false
			}
			return true
		})
	}
	for _, s := range stmts {
		walk(s, 0)
	}
	return ok
}

func walkBreakable(clause ast.Node, loop int, walk func(ast.Node, int), ok *bool) {
	ast.Inspect(clause, func(x ast.Node) bool {
		if b, isBranch := x.(*ast.BranchStmt); isBranch && b.Tok == token.BREAK && b.Label == nil {
			return false
		}
		switch x.(type) {
		case *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt, *ast.ReturnStmt, *ast.BranchStmt, *ast.DeferStmt, *ast.LabeledStmt, *ast.GoStmt:
			walk(x, loop)
			return false
		case *ast.FuncLit:
			return false
		}
		return true
	})
}

func extractNames(f fn, k int) []string {
	base := f.decl.Name.Name
	var recvType types.Type
	if sig := f.obj.Type().(*types.Signature); sig.Recv() != nil {
		recvType = sig.Recv().Type()
	}
	names := make([]string, k)
	for i := range names {
		for n := 1; ; n++ {
			name := fmt.Sprintf("%sPart%d", lowerFirst(base), i+n)
			if !taken(f, recvType, name) && !taken(f, nil, name) {
				names[i] = name
				break
			}
		}
	}
	return names
}
