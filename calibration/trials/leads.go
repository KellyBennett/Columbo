package trials

import (
	"fmt"
	"os"
	"slices"
	"sort"

	"github.com/KellyBennett/Columbo/calibration/harness"
	"github.com/KellyBennett/Columbo/calibration/operators"
)

// Reading is how the harness got from a lead's text to a refactoring.
type Reading string

const (
	// Says means the lead's text names the change, as an instruction, a suggestion, or a condition.
	Says Reading = "says"
	// PointsToward means the lead asks the reader to inspect something or answers a question, names what to look for,
	// and the harness judged one refactoring at the case's own code the obvious action on finding it. The judgment is
	// the harness's, not the lead's words.
	PointsToward Reading = "points toward"
)

// Refactoring is the cure the harness maps a lead to. Name is empty when the lead points at no single refactoring;
// Operator is empty when the harness has no cure that performs it.
type Refactoring struct {
	Name     string
	Operator string
	Reading  Reading
}

// leadRefactorings maps every lead Columbo writes into case_guidance to the cure it most plausibly points toward. An
// inspection or question that names nothing specific to find, or whose answer acts on a test, a contract, or code outside
// the case, maps to none.
var leadRefactorings = map[string]Refactoring{
	"Look for cohesive responsibilities with different reasons to change.": {"Extract Function", "extract-function", PointsToward},
	"Move behavior toward the object that owns its data.":                  {"Move Function", "move-function", Says},

	"Examine whether inputs form a cohesive domain object.":    {"Introduce Parameter Object", "introduce-parameter-object", PointsToward},
	"Separate responsibilities that require different inputs.": {"Extract Function", "extract-function", Says},

	"Clarify early exits and simplify nested decisions.":        {"Replace Nested Conditional with Guard Clauses", "", Says},
	"Examine whether variants deserve meaningful abstractions.": {"Replace Conditional with Polymorphism", "replace-conditional-with-polymorphism", PointsToward},

	"Does the caller know details its dependency could own?": {"Move Function", "move-function", PointsToward},
	"Is policy repeated across boundaries?":                  {"Extract Function", "delegate-duplicate", PointsToward},
	"Are behavioral assumptions explicit in the contract?":   {"Introduce Assertion", "", PointsToward},
	"Do these dependencies support one cohesive operation?":  {"Extract Function", "split-by-collaborators", PointsToward},
	"Compare supplied arguments and consumed results with types appearing only in signatures or discarded results; origin lists overlap and do not alter the score.": {},

	"Review each qualifying foreign value and its contributing accesses.": {},
	"Consider moving cohesive behavior toward the owner of that data.":    {"Move Function", "move-function", Says},
	"A qualifying value's type is declared in another package, so Go cannot take new methods on it. Consider a local type that wraps it and owns this behavior, or a value holding only the fields this code reads.": {"Introduce Local Extension", "", Says},

	"Inspect the participating signatures and struct types for a shared domain concept.": {"Introduce Parameter Object", "introduce-parameter-object", PointsToward},
	"Introduce a parameter object only if the values belong together.":                   {"Introduce Parameter Object", "introduce-parameter-object", Says},
	"Every member of this clump has a primitive type. Consider a distinct named type for each kind of value, such as one ID type per identifier; the type checker would then see the clump and reject values passed in each other's place.": {"Replace Primitive with Object", "", Says},

	"Review the qualifying helpers with their shared inputs and dependencies.": {},
	"Establish cohesive boundaries with distinct ownership and collaboration.": {},

	"Compare every duplicate-fragment receipt and identify knowledge that must change together.": {"Extract Function", "delegate-duplicate", PointsToward},
	"Share an operation only when the copies own the same policy.":                               {"Extract Function", "delegate-duplicate", Says},

	"Examine whether callers can send the same message to interchangeable role players instead of selecting behavior themselves.":                                                                         {"Replace Conditional with Polymorphism", "replace-conditional-with-polymorphism", PointsToward},
	"If the variants select implementations, use a factory or construction boundary to select once and return a common behavioral role; callers then send messages through that role.":                    {"Replace Constructor with Factory Function", "extract-factory", Says},
	"If independent operations inspect the same variants, evaluate a visitor or adapter with one dispatch boundary; keep operation-specific behavior, traversal policy, and results with each operation.": {"Introduce Visitor", "", Says},
	"If branches merely map variants to data, consider a data-driven representation owned near the variant definition.":                                                                                   {"Replace Conditional with Lookup Table", "", Says},
	"Move behavior toward the object or role that has the knowledge required to perform it.":                                                                                                              {"Move Function", "move-function", Says},
	"The selected implementations already respond to the same required messages. Consider expressing that collaboration as a role owned by the consumer.":                                                 {"Extract Interface", "", Says},

	"Consider moving implementation selection into a factory or composition boundary that returns the role.":      {"Replace Constructor with Factory Function", "extract-factory", Says},
	"Consider accepting the role as a dependency instead of deciding which concrete player to use here.":          {"Parameterize Function", "", Says},
	"Keep collaboration expressed in terms of the role while centralizing knowledge of concrete implementations.": {"Replace Constructor with Factory Function", "extract-factory", PointsToward},

	"If a comment preserves an intentional omission or guards against a future fix, write a regression test that fails when the unwanted behavior is introduced. Comments are not a reliable enforcement mechanism.": {},
	"Express intent through clear names, cohesive structure, and executable contracts; preserve useful design rationale in external documentation.":                                                                  {},
	// Deleting the comment is not a catalog refactoring, and since the case is the comment, clearing it is close to true by construction.
	"Remove obsolete explanations and commented-out code after preserving any required behavior in tests.": {"Delete Comment", "delete-prose-comments", Says},
}

type LeadOutcome string

const (
	LeadCleared  LeadOutcome = "cleared"
	LeadConflict LeadOutcome = "conflict"
	LeadSurvived LeadOutcome = "survived"
	LeadInvalid  LeadOutcome = "invalid"
	// LeadNoSite means the cure has no application aimed at the case; cure operators take sites only where their own target smell is reported.
	LeadNoSite LeadOutcome = "no-site"
	// LeadRefused means the cure had a site at one of the case's declarations and declined to rewrite it.
	LeadRefused       LeadOutcome = "refused"
	LeadNoOperator    LeadOutcome = "no-operator"
	LeadNoRefactoring LeadOutcome = "names-no-refactoring"
	LeadUnclassified  LeadOutcome = "unclassified"
)

// rank orders applied outcomes so a lead reports the best any application of its refactoring reached.
var rank = map[LeadOutcome]int{LeadInvalid: 1, LeadSurvived: 2, LeadConflict: 3, LeadCleared: 4}

// LeadCheck is one lead on one natural case, with what applying its refactoring did to that case.
type LeadCheck struct {
	Case         harness.Finding
	Ordinal      int
	Lead         string
	Refactoring  Refactoring
	Outcome      LeadOutcome
	Applications int
	Dose         int
	Reason       string
}

// LeadChecker applies the cure each lead names at every natural case carrying that lead. No model runs.
type LeadChecker struct {
	Env     harness.Env
	Workers int
}

type application struct {
	op      harness.Operator
	site    harness.Site
	dose    int
	applied harness.Applied
	probe   harness.Probe
}

// refusal is a cure that declined a site, kept so a lead with no application can say why.
type refusal struct {
	op   string
	site harness.Site
	err  error
}

func (lc LeadChecker) Check(c *harness.Corpus) ([]LeadCheck, error) {
	checks := classify(c)
	apps, refusals, err := applications(c, checks)
	if err != nil {
		return nil, err
	}
	harness.ForEach(len(apps), lc.Workers, func(i int) {
		a := apps[i]
		a.probe = lc.Env.Probe(c.Root, a.applied.Edit)
		_ = os.RemoveAll(a.probe.Dir)
	})
	for i := range checks {
		if checks[i].Outcome == "" {
			judge(c, &checks[i], apps, refusals)
		}
	}
	return checks, nil
}

func classify(c *harness.Corpus) []LeadCheck {
	var out []LeadCheck
	for _, f := range c.Baseline.Findings {
		for i, lead := range c.Baseline.Guidance[f.CaseID].Leads {
			ref, known := leadRefactorings[lead]
			check := LeadCheck{Case: f, Ordinal: i, Lead: lead, Refactoring: ref}
			switch {
			case !known:
				check.Outcome, check.Reason = LeadUnclassified, "lead text is not in the harness's lead table"
			case ref.Name == "":
				check.Outcome = LeadNoRefactoring
			case ref.Operator == "":
				check.Outcome, check.Reason = LeadNoOperator, "the harness has no cure operator for "+ref.Name
			}
			out = append(out, check)
		}
	}
	return out
}

// applications applies each needed cure at every site and dose, keeping those aimed at a case whose lead names it.
func applications(c *harness.Corpus, checks []LeadCheck) ([]*application, []refusal, error) {
	cases := map[string][]harness.Finding{}
	for _, ch := range checks {
		if ch.Outcome == "" {
			cases[ch.Refactoring.Operator] = append(cases[ch.Refactoring.Operator], ch.Case)
		}
	}
	ids := make([]string, 0, len(cases))
	for id := range cases {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var out []*application
	var refused []refusal
	for _, id := range ids {
		ops, err := operators.Select([]string{id})
		if err != nil {
			return nil, nil, err
		}
		op := ops[0]
		for _, site := range op.Sites(c) {
			for _, dose := range op.Doses(c, site) {
				applied, err := op.Apply(c, site, dose)
				if err != nil {
					refused = append(refused, refusal{id, site, err})
					continue
				}
				a := &application{op: op, site: site, dose: dose, applied: applied}
				if slices.ContainsFunc(cases[id], a.aimedAt) {
					out = append(out, a)
				}
			}
		}
	}
	return out, refused, nil
}

func declsOf(f harness.Finding) []harness.Decl {
	return append([]harness.Decl{{File: f.File, Symbol: f.Symbol}}, f.Support...)
}

// aimedAt holds when the cure rewrites one of the case's declarations in one of its files.
func (a *application) aimedAt(f harness.Finding) bool {
	for _, d := range declsOf(f) {
		if slices.Contains(a.applied.Touched, d.Symbol) {
			if _, edited := a.applied.Edit[d.File]; edited {
				return true
			}
		}
	}
	return false
}

func judge(c *harness.Corpus, check *LeadCheck, apps []*application, refused []refusal) {
	best := LeadOutcome("")
	for _, a := range apps {
		if a.op.ID() != check.Refactoring.Operator || !a.aimedAt(check.Case) {
			continue
		}
		check.Applications++
		got, reason := a.outcomeFor(c, check.Case)
		if rank[got] > rank[best] {
			best, check.Dose, check.Reason = got, a.dose, reason
		}
	}
	if check.Applications == 0 {
		check.Outcome, check.Reason = LeadNoSite, fmt.Sprintf("%s has no site at this case", check.Refactoring.Operator)
		for _, r := range refused {
			if r.op == check.Refactoring.Operator && slices.ContainsFunc(declsOf(check.Case), func(d harness.Decl) bool { return d.Symbol == r.site.Symbol }) {
				check.Outcome, check.Reason = LeadRefused, fmt.Sprintf("%s refused: %v", r.op, r.err)
				break
			}
		}
		return
	}
	check.Outcome = best
}

// outcomeFor scores the cure against the lead's own case, which may be a different smell from the one the operator targets.
func (a *application) outcomeFor(c *harness.Corpus, f harness.Finding) (LeadOutcome, string) {
	if a.probe.Invalid != "" {
		return LeadInvalid, a.probe.Invalid
	}
	o := harness.Outcome{Class: harness.Cure, Target: f.Smell}
	exp := a.applied.Expectation
	exp.Target = f.Smell
	harness.Score(&o, exp, c.Baseline.Findings, a.probe.Snapshot.Findings)
	if !gone(f, o.Removed, a.probe.Snapshot.Findings) {
		return LeadSurvived, "case remains"
	}
	switch o.Status {
	case harness.Killed:
		return LeadCleared, o.Reason
	case harness.Conflict:
		return LeadConflict, o.Reason
	}
	return LeadSurvived, o.Reason
}

// gone needs both signals. File-wide prose cases share a package-clause symbol, so Diff may name a sibling as the removed one;
// a Data Clump case can change its ID while it stays, so a missing ID alone is not enough either.
func gone(f harness.Finding, removed, after []harness.Finding) bool {
	if slices.ContainsFunc(after, func(a harness.Finding) bool { return a.CaseID == f.CaseID }) {
		return false
	}
	return slices.ContainsFunc(removed, func(r harness.Finding) bool { return r.Smell == f.Smell && r.Symbol == f.Symbol })
}
