package operators

import (
	"errors"
	"fmt"
	"go/types"

	"github.com/KellyBennett/Columbo/calibration/harness"
)

const envySmell = "feature-envy"

// moveMethod is the inverse of Move Function. Without collaborator it turns a method into
// a free function that takes the receiver as its first parameter. With collaborator it
// moves the method onto the same-package type of one of its parameters, the plan's
// "envied owner's collaborator". Either way the old receiver's own accesses become
// foreign accesses. Collaborator dose is the parameter index plus one.
type moveMethod struct {
	id           string
	collaborator bool
}

func (o moveMethod) ID() string           { return o.id }
func (o moveMethod) Class() harness.Class { return harness.Inject }
func (o moveMethod) Target() string       { return envySmell }

func (o moveMethod) Sites(c *harness.Corpus) []harness.Site {
	l := limitsOf(c)
	var out []harness.Site
	for _, f := range funcs(c) {
		recv := recvVarOf(f)
		if recv == nil || f.decl.Body == nil || f.baselineHas(c, envySmell, f.symbol) {
			continue
		}
		if profileOf(f.pkg.TypesInfo, f.decl.Body, recv).own < l.foreign-1 {
			continue
		}
		if len(o.Doses(c, f.site())) > 0 {
			out = append(out, f.site())
		}
	}
	return out
}

func (o moveMethod) Doses(c *harness.Corpus, site harness.Site) []int {
	f, ok := findFn(c, site.Symbol)
	if !ok {
		return nil
	}
	if !o.collaborator {
		if _, err := planRelocation(c, f, toFunction); err != nil {
			return nil
		}
		return []int{1}
	}
	recvT, _ := localNamed(recvVarOf(f).Type(), f.pkg.Types)
	sig := f.obj.Type().(*types.Signature)
	var doses []int
	for i := 0; i < sig.Params().Len(); i++ {
		if t, ok := localNamed(sig.Params().At(i).Type(), f.pkg.Types); !ok || t == recvT {
			continue
		}
		if _, err := planRelocation(c, f, i); err == nil {
			doses = append(doses, i+1)
		}
	}
	return doses
}

func (o moveMethod) Apply(c *harness.Corpus, site harness.Site, dose int) (harness.Applied, error) {
	f, ok := findFn(c, site.Symbol)
	if !ok {
		return harness.Applied{}, errors.New("site vanished")
	}
	to := toFunction
	var newOwn *types.Var
	if o.collaborator {
		to = dose - 1
		newOwn = f.obj.Type().(*types.Signature).Params().At(to)
	}
	r, err := planRelocation(c, f, to)
	if err != nil {
		return harness.Applied{}, err
	}
	edit, err := r.edit()
	if err != nil {
		return harness.Applied{}, err
	}
	moved := r.newSymbol()
	return harness.Applied{
		Edit: edit,
		Expectation: harness.Expectation{
			Touched:    []string{moved},
			Renames:    map[string]string{f.symbol: moved},
			Prediction: predictEnvy(profileOf(f.pkg.TypesInfo, f.decl.Body, newOwn), limitsOf(c), "after the move"),
		},
	}, nil
}

// moveFunction is Move Function, the cure. For each baseline feature-envy case whose
// qualifying group is a bare parameter of a same-package named type, it moves the
// declaration onto that type. Dose is the parameter index plus one.
type moveFunction struct{}

func (moveFunction) ID() string           { return "move-function" }
func (moveFunction) Class() harness.Class { return harness.Cure }
func (moveFunction) Target() string       { return envySmell }

func (o moveFunction) Sites(c *harness.Corpus) []harness.Site {
	var out []harness.Site
	for _, f := range funcs(c) {
		if f.decl.Body != nil && f.baselineHas(c, envySmell, f.symbol) && len(o.Doses(c, f.site())) > 0 {
			out = append(out, f.site())
		}
	}
	return out
}

func (moveFunction) Doses(c *harness.Corpus, site harness.Site) []int {
	f, ok := findFn(c, site.Symbol)
	if !ok || f.obj == nil {
		return nil
	}
	sig := f.obj.Type().(*types.Signature)
	params := map[*types.Var]int{}
	for i := 0; i < sig.Params().Len(); i++ {
		params[sig.Params().At(i)] = i
	}
	p := profileOf(f.pkg.TypesInfo, f.decl.Body, recvVarOf(f))
	groups := p.qualifying(limitsOf(c))
	sortedGroups(groups)
	var doses []int
	for _, g := range groups {
		i, isParam := params[g.root]
		if !g.bare() || !isParam {
			continue
		}
		if _, err := planRelocation(c, f, i); err == nil {
			doses = append(doses, i+1)
		}
	}
	return doses
}

func (moveFunction) Apply(c *harness.Corpus, site harness.Site, dose int) (harness.Applied, error) {
	f, ok := findFn(c, site.Symbol)
	if !ok {
		return harness.Applied{}, errors.New("site vanished")
	}
	to := dose - 1
	r, err := planRelocation(c, f, to)
	if err != nil {
		return harness.Applied{}, err
	}
	edit, err := r.edit()
	if err != nil {
		return harness.Applied{}, err
	}
	after := profileOf(f.pkg.TypesInfo, f.decl.Body, r.params[to].v)
	l := limitsOf(c)
	clears := len(after.qualifying(l)) == 0
	moved := r.newSymbol()
	return harness.Applied{
		Edit: edit,
		Expectation: harness.Expectation{
			Touched: []string{f.symbol},
			Renames: map[string]string{f.symbol: moved},
			Prediction: harness.Prediction{
				Kill:   &clears,
				Reason: fmt.Sprintf("on %s: %s", r.params[to].name, after.describe(l)),
			},
		},
	}, nil
}
