package columbo

import "go/ast"

func (helper *guardedHelper) site(guard ast.Expr) advisorySite {
	helper.scan.helpers[helper.target.symbol] = helper.target
	site := helper.scan.site(helper.caller, helper.call, &guardedMatch{guard: guard})
	site.representation = "guarded-helper-call"
	site.receipts = append(site.receipts, helper.callReceipt())
	return site
}
func (helper *guardedHelper) callReceipt() Source {
	return helper.scan.owner.categoryReceipt("guarded-helper-call", helper.call, Detail{Subject: "caller repeats boundary in " + helper.target.symbol + "; outer guard is not proven redundant"})
}

func guardedHelperDeclarations(groups map[guardedUpdateKey][]advisorySite, helpers map[string]*declaration) {
	for _, sites := range groups {
		for index := range sites {
			owner := helpers[sites[index].symbol]
			if owner != nil && sites[index].representation == "guarded-update" {
				sites[index].receipts = append(sites[index].receipts, owner.categoryReceipt("guarded-helper-declaration", owner.fn, Detail{Subject: "sole pointer parameter maps to caller subject; one-hop evidence"}))
			}
		}
	}
}
