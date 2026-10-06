package columbo

import (
	"go/ast"
	"go/token"
	"maps"
	"slices"
)

type selectionLoop struct {
	init, post ast.Stmt
	cond       ast.Expr
	body       *ast.BlockStmt
	mayExit    bool
}

func (loop selectionLoop) run(scan *selectionScan, env selectionEnv) []selectionPath {
	scan.statement(loop.init, env)
	probe := *scan
	probe.probing = true
	head, _ := loop.fixedPoint(&probe, env)
	head, paths := loop.fixedPoint(scan, head)
	return loop.exits(head, paths)
}
func (loop selectionLoop) fixedPoint(scan *selectionScan, entry selectionEnv) (selectionEnv, []selectionPath) {
	head := entry.clone()
	for {
		scan.calls(loop.cond, head)
		paths := scan.block(loop.body.List, head.clone())
		next := joinSelectionEnvs(append([]selectionEnv{entry}, loop.backEdges(scan, paths)...))
		if sameSelectionEnv(head, next, scan.probing) {
			return head, paths
		}
		head = next
	}
}
func (loop selectionLoop) backEdges(scan *selectionScan, paths []selectionPath) []selectionEnv {
	var edges []selectionEnv
	for _, path := range paths {
		if path.exit != token.ILLEGAL && path.exit != token.CONTINUE {
			continue
		}
		scan.statement(loop.post, path.env)
		edges = append(edges, path.env)
	}
	return edges
}
func (loop selectionLoop) exits(head selectionEnv, paths []selectionPath) []selectionPath {
	var exits []selectionEnv
	if loop.mayExit {
		exits = append(exits, head)
	}
	for _, path := range paths {
		if path.exit == token.BREAK {
			exits = append(exits, path.env)
		}
	}
	if len(exits) == 0 {
		return nil
	}
	return []selectionPath{{env: joinSelectionEnvs(exits)}}
}
func joinSelectionEnvs(paths []selectionEnv) selectionEnv {
	result := selectionEnv{}
	for _, env := range paths {
		for obj := range env {
			result[obj] = mergeSelectedValues(reachingSelectedValues(obj, paths))
		}
	}
	return result
}
func sameSelectionEnv(left, right selectionEnv, originsOnly bool) bool {
	if len(left) != len(right) {
		return false
	}
	for obj, value := range left {
		other, ok := right[obj]
		if !ok || !value.same(other, originsOnly) {
			return false
		}
	}
	return true
}
func (v selectedValue) same(other selectedValue, originsOnly bool) bool {
	if v.unknown != other.unknown || !maps.Equal(v.origins, other.origins) {
		return false
	}
	return originsOnly || sameSelectionTraces(v.traces, other.traces)
}
func sameSelectionTraces(left, right map[*selectionSite][]ast.Node) bool {
	if len(left) != len(right) {
		return false
	}
	for site, flow := range left {
		other, ok := right[site]
		if !ok || !sameSelectionFlow(flow, other) {
			return false
		}
	}
	return true
}
func sameSelectionFlow(left, right []ast.Node) bool {
	if len(left) != len(right) {
		return false
	}
	for _, node := range left {
		if !slices.Contains(right, node) {
			return false
		}
	}
	return true
}

func newSelectionLoop(n *ast.ForStmt) selectionLoop {
	return selectionLoop{init: n.Init, cond: n.Cond, post: n.Post, body: n.Body, mayExit: n.Cond != nil}
}
