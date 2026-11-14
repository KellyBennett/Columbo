# Mutation calibration plan

Status: plan. Nothing here is built yet.

Columbo is meant to be strict and trustworthy at once. Today the two goals pull
apart. On its own source, the enforced staged gate passes, while the all-rule
gate (`--staged=false`) reports 62 FAIL cases. Feature Envy has 24 of them and
Excessive Dependencies has 20. In 19 of the 24 Feature Envy cases, no foreign
count exceeds the limit by more than 1. Nobody can yet say whether those
failures mean mistuned rules or real debt, because Columbo has no measurement
of what it catches and what it misses.

This plan builds that measurement. A harness takes Go code, changes it in known
ways, runs Columbo before and after, and records whether Columbo noticed. The
results feed threshold, severity, and rule decisions through the existing
dogfooding decision register. The harness reports. People decide.

## What the harness answers

| Question | Mutant class | Expected result |
|---|---|---|
| Does Columbo catch a smell when one is added? | inject | A new case of the target smell on the mutated declaration |
| Can a cosmetic fix silence a case? | evade | The smell is still reported, possibly as Cosmetic Extraction |
| Are verdicts and case IDs stable under edits that change nothing? | neutral | No case added or removed, and every case ID unchanged or remapped as predicted |
| Does a legitimate refactoring reach green? | cure | The target case clears and no other case appears |

The cure class matters as much as the other three. If a proper refactoring
trips a different rule, the code has no green path. An agent in that position
can only fake a fix or suppress the case.

## Score against the unmutated code, not a clean baseline

A mutant is scored by the difference between Columbo's findings on the
original module and on the mutated copy. Findings that exist in both don't
count. So the corpus doesn't need to be well factored, or even pass Columbo.
Any Go module that builds and has tests will do.

Smells don't change behavior, so a valid mutant must still build and pass the
original module's tests. A mutant that fails either check is `invalid` and gets
discarded. The tests are how the harness knows a mutant is real.

## Data shapes

The harness is built around these types. The names are provisional. The shape
is not.

```go
type Class string // "inject", "evade", "neutral", "cure"

type Operator interface {
	ID() string
	Class() Class
	Target() string // smell slug; empty for neutral operators
	Sites(pkg *packages.Package) []Site
	Apply(pkg *packages.Package, site Site, dose int) (Edit, error)
}

type Site struct {
	Package string
	Symbol  string // qualified declaration symbol, as in declarations.symbol
	Offset  int
}

type Mutant struct {
	Chain []Step // one step for inject, neutral, and cure; inject then evade for evade
	Patch []byte
	Size  int    // changed AST nodes across touched declarations
}

type Step struct {
	Operator string
	Site     Site
	Dose     int
}

type Finding struct {
	Smell  string
	Symbol string // primary declaration symbol
	CaseID string
}

type Outcome struct {
	Mutant  Mutant
	Added   []Finding
	Removed []Finding
	Status  Status
}

type Status string // "killed", "survived", "invalid", "equivalent", "conflict", "id-drift"
```

Mutants are matched to findings by smell and primary declaration symbol, not by
case ID. An inject mutant can legitimately change a case's secondary key, such
as a Data Clump's type list. Neutral mutants are the exception, and they compare
case IDs directly.

Case identity includes the qualified primary symbol. So renaming a declaration
changes its case IDs, and it does so predictably. The neutral rename operator
therefore carries an expected old-to-new symbol mapping, and `id-drift` means a
case ID changed in a way that mapping doesn't explain.

The harness reads Columbo's SQLite snapshot (`cases` joined to `declarations`)
and doesn't reach into Columbo's internals. That keeps the harness on the same
evidence contract that agents and the GitHub publisher use. It runs Columbo
with `--staged=false --no-history`, because mutated copies are not git
checkouts and every rule has to be visible.

Results go into `calibration.sqlite`. Each run also records the corpus module
and version, the Columbo build version, and a hash of the configuration, so any
number in a report can be traced back to the run that produced it.

## Operator catalog

Operators are the inverses of named refactorings from Fowler's catalog. Each
refactoring cures a smell, so its inverse introduces that smell. Each row also
names the matching evade and cure operators.

| Inject (inverse refactoring) | Target smell | Evade | Cure |
|---|---|---|---|
| Inline Function, by dose | `long-function`, `high-cognitive-complexity` | Split into private helpers that forward the same parameters | Extract Function along a real responsibility seam |
| Move Function to the envied owner's collaborator | `feature-envy` | Hide fields behind getters, or copy them into locals | Move Function |
| Replace Polymorphism with Conditional at a second site | `repeated-variant-decision` | Swap the switch for a map lookup that is still repeated at each site | Replace Conditional with Polymorphism |
| Inline Parameter Object | `data-clump`, `long-parameter-list` | Pack into an anonymous struct, or reorder parameters | Introduce Parameter Object |
| Duplicate a block | `duplicate-code` | Rename identifiers and change literals | Extract Function |
| Add collaborators to a declaration | `excessive-dependencies` | Route through a single facade type | Split the declaration by collaborator group |
| Add explanatory comments | `prose-comment` | Move the prose into string constants or struct tags | Delete the comment |

Neutral operators are `reorder-decls`, `move-decl-to-file` (same package),
`rename-local`, and `rename-decl` (with its expected ID mapping).

`golang.org/x/tools` is already a dependency, so operators are `go/ast`
rewriters over type-checked `go/packages` loads. Its full inliner lives in
`internal/refactor/inline` and can't be imported. The Inline Function operator
therefore supports a restricted case: straight-line helpers with no `defer`,
`recover`, named results, labels, or goroutines. The validity check throws away
anything that slips through.

## Survivors are triaged, not counted

A surviving mutant is not automatically a Columbo blind spot. Inlining a
two-line helper into a six-line function correctly stays below `long-function`.
Mutation testing calls these equivalent mutants.

Each operator states its expected outcome from the metrics where it can. For
example, the Inline Function operator knows the parent's line count after
inlining. Survivors that the expectation can't explain are triaged once in
`calibration/triage.yaml`, keyed by operator, site symbol, and dose. Each entry
has a status of `equivalent` or `blind-spot` and a reason. A blind spot either
gets a Columbo fix or becomes a named, accepted gap in the decision register.

## Build phases

Every phase ends in a check that runs before the next phase starts. Each
operator lands in its own PR with a fixture that shows its kill.

### Phase 0. Build the harness spine

Build the loop: copy the module to a temporary directory, apply an edit, check
validity, run the `columbo` binary, read both snapshots, compare findings, and
write `calibration.sqlite`. The only operator is `noop`.

Done when `noop` on Columbo's root module is valid, adds and removes nothing,
keeps every case ID, and produces identical result rows on two consecutive
runs.

### Phase 1. Add the first inject and evade pair

Add Inline Function (inject, `long-function`) and forwarding-helper split
(evade). The evade operator runs on top of a killed inject mutant. Report a
dose curve: the inline count at which `long-function` first fires.

This pair goes first because Cosmetic Extraction runs under CE-001, a
provisional policy. The public-release gate requires regression fixtures before
CE-001 can be resolved, and this pair produces them.

Done when, on a fixture with known metrics, inline kills at the predicted dose
and the split is killed by `cosmetic-extraction`.

### Phase 2. Add the neutral checks

Add the four neutral operators and run them across Columbo's root module.

Done when there is zero `id-drift`. Any drift found counts as a Columbo bug and
gets a minimized fixture.

### Phase 3. Add cures, green-path checks, and mutant size

Add Extract Function, Introduce Parameter Object, and Move Function as cure
operators. Report every `conflict`, meaning a cure that cleared its target but
set off another rule. Compute `Size` for every mutant. Report cost asymmetry per
smell: the smallest surviving evade against the smallest successful cure.

The README promises that a violation is "harder to silence than to properly
refactor". This table measures that promise.

Done when the conflict list and the asymmetry table are generated for
`long-function` and `data-clump`.

### Phase 4. Fill out the catalog and widen the corpus

Add the remaining operators from the catalog, one per PR. Add external corpus
modules, pinned by module version, with their licenses recorded.

Done when every smell in the default severity map has at least one inject, one
evade, and one cure operator with a fixture.

### Phase 5. Report the kill matrix and gate on untriaged survivors

Generate the operator-by-smell kill matrix. Flag two patterns: a rule whose
kills are all also kills of another rule (redundancy), and an operator family
that no rule kills (a missing rule). Add a CI job that warns on survivors not
covered by `triage.yaml`.

Done when the report is generated from `calibration.sqlite` by query alone and
the CI job runs on Columbo's own module.

### Phase 6. Chain operators with fuzzing and shrink the survivors

Add a `go test -fuzz` target whose input bytes select an operator sequence.
Check two invariants. A neutral-only chain causes no drift. A chain that
contains an inject is eventually killed. When a chain survives, shrink it
automatically to its minimal form and save it as a fixture under `testdata`.

Done when one seeded survivor shrinks to a fixture of no more than two steps.

### Phase 7. Freeze a red-team evasion corpus

Prompt an agent to "make Columbo pass on this code without improving the
design". Freeze its outputs under `calibration/redteam/` as patches, and replay
them as evade mutants. No LLM runs during calibration itself. Only the frozen
patches do.

Done when the replayed corpus gives identical outcomes on two consecutive runs.

### Phase 8. Run agents against mutants, and run the leads

Outside CI, hand an agent a mutant and Columbo's output. Score how close its fix
comes to the original code, and whether the tests still pass. Separately, apply
each lead that names a refactoring and check whether the lead clears its own
case. Report trial counts and spread with every number.

Done when each default smell has a fix-distance distribution from at least five
trials.

### Phase 9. Calibrate from work, privately (separate track)

Build a local case ledger (`~/.columbo/ledger.sqlite`) and a
`columbo calibrate --export` command. The export holds only aggregates per rule
and per operator. It contains no names, paths, hashes, case IDs, or
justification text, and it drops any bucket with fewer than 5 cases or 3 repos.
Use real-world shape frequencies from the ledger to weight the kill matrix.

This track starts after phase 5. It needs its own design pass before it starts.

## How results change Columbo

A threshold, severity, or rule change cites the calibration report rows that
justify it, as an entry in the dogfooding decision register. Each change shows
the before and after for three things: the kill rate, the conflict list, and
the self-check case count. The harness never edits configuration or rules.

## Non-goals

- Changing any rule as part of building the harness.
- Running an LLM in any CI path.
- Measuring false positives on natural code. The work ledger (phase 9) and the
  self-check adjudication ledger cover that.

## Resolved decisions

The build settled the three questions this plan left open.

- **Where the harness lives.** It is a nested module at `calibration/` with its
  own `go.mod`, as recommended. Product dependencies and the release build are
  unchanged, and `columbo ./...` on the root module does not analyze the harness.
- **Cost of the validity check.** Both options were built. Columbo's own module
  runs `go vet` plus the tests of the mutated packages only. External corpora
  use `--validity build`, which runs `go vet` alone, so no downloaded code is
  executed. On Columbo's own module, build and full-test validity agreed on all
  16 mutants compared (see `docs/calibration-tuning-proposals.md`).
- **When the survivor gate blocks CI.** It warns. The calibration job on pushes
  to main publishes untriaged survivors as a workflow warning, and
  `calibration/triage.yaml` covers the existing survivors. Pull requests run
  only the operator fixture tests, which fail on a broken kill.
