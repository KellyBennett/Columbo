# Calibration findings and tuning proposals

Status: proposals. Nothing here changes Columbo's rules or defaults. The companion pull
request that applies the threshold changes is separate, so each one can be accepted or
dropped on its own.

The question these proposals answer: when you run Columbo on a Go project and act on what
it reports, does the code become easier to maintain? That needs three things to be true.
Columbo has to notice the smells it claims to notice. A cosmetic edit must not be able to
silence a finding more cheaply than a real refactoring clears it. And a real refactoring
has to be able to reach green without tripping a different rule.

The [mutation calibration harness](../calibration/README.md) measures all three. It changes
Go code in known ways, runs Columbo before and after, and records what Columbo noticed.

## What was measured

- **Corpus.** Columbo itself and four pinned external modules: cobra v1.9.1, chi v5.2.1,
  BurntSushi/toml v1.5.0, google/uuid v1.6.0 ([corpus.txt](../calibration/corpus.txt)).
- **Operators.** 41 operators across all eleven smells: injects that add a smell (inverse
  refactorings), evades that try to hide one cosmetically, cures that apply the real
  refactoring, and neutral edits that should change nothing.
- **Red team.** An agent that saw only the code, the `columbo` binary and its output was
  asked to silence 14 real FAIL cases in cobra and chi without improving the design. Its
  edits are frozen under [calibration/redteam](../calibration/redteam) and replayed as
  evades, with identical outcomes on two consecutive replays.
- **Validity.** Every mutant must pass `go vet`. External code is never executed, so its
  mutants are build-validated only. On Columbo's own module, build and full-test validity
  agreed on all 16 mutants where both were measured.
- **Scale.** 2,334 mutants in the first full run (`calibrate report` over
  `calibration.sqlite`), plus threshold sweeps that rerun the relevant operators under each
  proposed configuration.

Reproduce with `calibration/calibrate.sh <db> all 25`, then `calibrate report --db <db>`.
Sweeps use `calibration/sweep.sh` and `calibration/compare.sh`.

## Verdict by rule

"Catches" is the share of injected smells Columbo reported, among those the operator's own
metric prediction said should fire. "Cheapest evade" is the smallest surviving cosmetic
edit, in changed AST nodes. "Green path" is the share of real cures that cleared the target
without setting off another rule.

| Rule | Catches | Survives a cosmetic evade? | Green path | Trust today |
|---|---|---|---|---|
| long-function | 47 of 48 | Yes: unbalanced forwarding split, block isolation, red team (12 nodes) | 45 of 52 cures | Good catch, weak against splitting |
| high-cognitive-complexity | 10 of 10 | Rarely (3 survivors) | 37 of 41 | Good |
| feature-envy | 84 of 84 | Yes: copy fields to locals, aliasing (9 nodes) | 16 of 16 Move Function | Good catch, cheap alias evade, noisy at the limit |
| excessive-dependencies | 125 of 125 | Always: a private facade struct (125 of 125) | 17 of 18 | Noisy and trivially evaded |
| data-clump | 29 of 29 | Always: one anonymous struct parameter (29 of 29) | 1 of 5 | Its own cure trips Feature Envy |
| long-parameter-list | 31 of 31 | Always: anonymous struct (31 of 31) | none measured | Trivially evaded |
| cosmetic-extraction | 20 of 109 splits | Almost always: a bare block per call (18 of 20) | n/a | Misses most mechanical splits |
| duplicate-code (WARN) | 49 of 49 | Never: renaming is still caught (48 of 48) | no mechanical cure | Good; unstable IDs under reordering |
| prose-comment | 26 of 26 | Always: string constant, `//nolint:` prefix, generated header | 78 of 78 | Trivially evaded |
| repeated-variant-decision | 11 of 11 | Always: a map lookup (13 of 13) | no mechanical cure | Trivially evaded |
| selection-use-coupling | 0 natural sites in 5 corpora | Yes: identity helper laundering | n/a | Unmeasured on real code |

Two things hold everywhere. First, Columbo's metrics are accurate: of 854 inject mutants
whose operator predicted the outcome from its own reimplementation of the metric, 1
disagreed. Second, the README's promise that "a violation should be harder
to silence than to properly refactor" does not hold for any rule today. The red team
silenced all 14 targets, and 9 of those took under 3 lines.

## Proposals

Ordered by how much each one changes whether acting on Columbo improves the code. Each
names the evidence that justifies it and the before and after it would produce. Proposals
1 to 3 are rule-scope changes that need a SPEC amendment; 4 to 6 are default threshold
changes measured by sweep; 7 to 9 are guidance and stability; 10 is a bug already fixed.

### 1. Close the three escapes that silence any rule (CAL-001)

The red team found three edits that remove a declaration from analysis entirely, so every
rule goes quiet at once.

- **A hand-written generated-code header.** Adding `// Code generated by hand. DO NOT EDIT.`
  to cobra's `command.go` removed all 41 of its cases with no warning. SPEC makes the
  generated-file exclusion mandatory and silent.
  *Proposal:* keep the exclusion, and emit a `generated-exclusion` warning for each excluded
  file, listing how many declarations it hid. In the staged gate, treat a newly generated
  file whose header names no generator (`by hand`, empty) as a FAIL.
  *Evidence:* `redteam/cobra-prose-comment`, `redteam/chi-long-parameter-list`.
- **A function literal in a package-level `var`.** Moving `middleware.RedirectSlashes`'s
  body into `var redirectSlashes = func(...) {...}` cleared its long-function, complexity 11
  and 8 dependencies, and nothing was reported on the new var. SPEC limits finding subjects
  to named function declarations.
  *Proposal:* make a package-level variable whose initializer is a function literal a
  finding subject, under the variable's qualified name.
  *Evidence:* `redteam/chi-long-function`.
- **A `//nolint:` prefix on prose.** `//nolint:godot // <the original prose>` is classified
  as a machine directive because docs/prose-comments.md exempts nolint's trailing
  justification.
  *Proposal:* exempt the directive, and classify text after its second `//` as prose.
  *Evidence:* `redteam/chi-prose-comment`.

### 2. Close the cheap per-rule evades (CAL-002)

Each of these survived every site it was tried on, and each is a shape the case's own
"avoid" guidance already names.

| Evade | Survived | Proposed detection |
|---|---|---|
| Bare `{ }` block around each helper call | 18 of 20 cosmetic splits | Treat a bare block statement as transparent when forming clusters. Implemented on `fix/cosmetic-bare-blocks`: 0 of 9 survive, no new cure conflicts, no new natural cases on Columbo or cobra |
| One anonymous struct parameter instead of a clump | 29 data-clump, 31 long-parameter-list | Count an anonymous struct parameter's fields as parameters for both rules |
| Private facade struct holding the collaborators | 125 of 125 | Score the dependencies of a same-package unexported type's fields when the function only reaches them through it |
| Map from the variants instead of a second switch | 13 of 13 | Count a map literal keyed by a known variant set as a decision site |
| Prose moved into an unused string constant | 26 of 26 | Flag an unreferenced package-level string constant containing whitespace |
| Copying the envied value's fields into locals | 2 of 16 | Count distinct fields read through a value, including reads into locals |

The cosmetic-block and anonymous-struct detections are small and local. The facade
detection is the largest and the one most worth having, because Excessive Dependencies is
otherwise the cheapest rule to fake (a 41-node facade against a 30-node real split).

### 3. Give data-clump a one-pass green path (CAL-003)

Introduce Parameter Object, the cure Columbo's data-clump lead points to, set off another
rule in 4 of 5 measured cases: Feature Envy on the functions that now read the new object's
fields 5 or more times, Excessive Dependencies on the new type, and Duplicate Code where
every caller repeats the same struct literal. Feature Envy is right about the second step:
Move Function onto the new type reached green in 16 of 16 cures. So the cure exists but
takes two passes, and an agent that stops after the first sees a new FAIL.

*Proposal:* change the data-clump lead to name both steps: introduce the type, then move the
behavior that reads those values onto it. Where the clump mirrors an existing struct's
fields, lead toward passing that struct whole (Preserve Whole Object) instead.
*Evidence:* `Cure conflicts`, introduce-parameter-object rows.

### 4. Lower cosmetic-dependency-overlap from 0.75 to 0.5 (CAL-004)

Most mechanical splits of real long functions are not reported. The probe on cobra's
`writePreamble` shows why: two helpers forwarding the same `(buf, name)` share 2 of 3
dependencies, an overlap of 0.67.

| Configuration | Splits of real long functions caught | Forwarding splits of inlined functions caught | New cure conflicts | Natural cosmetic cases, 5 corpora |
|---|---|---|---|---|
| 0.75 (today) | 20 of 100 | 44 of 59 | 0 | 0 |
| **0.5** | **49 of 100** | **47 of 59** | **0** | **1** |
| 0.25 | 63 of 99 | 51 of 59 | 0 | 18 |

The one natural case at 0.5 is Columbo's own `(*complexityVisitor).visitIfStmt`, which hands
its one parameter to two single-use helpers, `ifCondition(n)` and `ifBranches(n)`: a true
cosmetic split. 0.25 catches more but adds 17 natural cases nobody has adjudicated. This
interacts with CE-001: lowering the overlap makes that provisional policy stricter, so it
should be decided together with CE-001's resolution.

### 5. Raise dependencies from 5 to 7 (CAL-005)

| Configuration | Natural FAIL cases, 5 corpora (Columbo) | Injected collaborators caught at the line | Excessive Dependencies conflicts on Extract Function cures | on Introduce Parameter Object cures |
|---|---|---|---|---|
| 5 (today) | 77 (20) | 100 of 100 | 6 | 3 |
| **7** | **26 (8)** | **100 of 100** | **0** | **0** |

Two findings point the same way. First, at 5 the rule blocks real refactorings: extracting a
helper copies the parent's parameter types into the helper's signature, so splitting a
function with 7 dependencies yields two functions with 6. Second, 15 of Columbo's 20 cases
have no extractable seam at all (the split-by-collaborators operator searched every
statement partition with a dependency estimator that matches Columbo on 1,511 of 1,518
functions); they are single-purpose go/types and go/ast walks that need that many types.
The rule still fires on every injected collaborator past the new line.

An alternative that would keep 5: score signature-only dependencies (types that only pass
through a parameter or result) at a discount, since those are what extraction duplicates.
That is a rule change; 7 is a default change and is measured.

### 6. Raise feature-envy-foreign-accesses from 5 to 6 (CAL-006)

| Configuration | Natural FAIL cases, 5 corpora (Columbo) | Injected envy caught | Envy conflicts on Extract Function cures | Copy-to-locals evades surviving |
|---|---|---|---|---|
| 5 (today) | 78 (24) | 50 of 70 move-method mutants | 6 | 2 |
| **6** | **49 (10)** | **45 of 61** | **2** | **1** |
| ratio 1.5 instead | 92 (34) | 49 of 70 | 5 | 2 |

Feature Envy fires at the margin: 19 of Columbo's 24 cases and 22 of the 54 external cases
are within one access of the limit, and 65 of 78 are free functions with no receiver. Its
green path is excellent where a move is possible (16 of 16), but extraction cures produce
small free-function helpers that read a parameter five times, which is where most of its
conflicts come from. Raising the minimum to 6 roughly halves the natural cases and
two-thirds of the conflicts, at the cost of a few injected cases sitting exactly at 5.
Lowering the ratio to 1.5, which one operator family suggested, adds 14 natural cases for
one more injected kill; it is not recommended.

### 7. Stabilize duplicate-code identity under reordering (CAL-007)

Reordering declarations within one file, which changes nothing, changed duplicate-code case
IDs in toml (`lexLongUnicodeEscape`) and cobra (`MarkFlagsMutuallyExclusive`) in a way SPEC's
earliest-supporting-declaration rule does not predict. Every other rule kept or predictably
remapped its IDs across 431 neutral mutants. Duplicate code is a WARN, so the cost is
suppression and history churn, not failed builds.
*Proposal:* derive the group anchor from a canonical order of fragments rather than the
suffix tree's discovery order, and add a reorder fixture.

### 8. Measure selection-use-coupling on code that has it (CAL-008)

None of the five corpora has a natural selection-use-coupling case, and only two functions
return an interface with two or more concrete results. The rule is unmeasured on real code;
its fixtures pass. *Proposal:* keep SUC-001 provisional until a corpus with factories and
strategies is added.

### 9. Repeated-variant-decision has no mechanical green path (CAL-009)

Every natural RVD case is either an enum domain (`reflect.Kind`, `nodeTyp`, `itemType`) or a
type switch over another package's interfaces (`go/ast`), where Go cannot add methods. The
mechanical Replace Conditional with Polymorphism cure only applies to same-package pointer
types, and found no natural site. Together with the map-lookup evade, this rule today asks
for a redesign it cannot show is possible and accepts a cosmetic rewrite. *Proposal:* add a
visitor or table-driven lead for foreign and enum domains, and decide RVD-001 with that
evidence.

### 10. Fixed: Feature Envy merged type-switch clause variables

Columbo keyed envy groups by `<symbol>:<name>@<offset>`, and every clause variable of a type
switch shares one offset, so reads spread over clauses merged into one group. On Columbo,
`typeComponents` read 2 + 1 + 0 + 2 fields of four different variables and `supportedNode`
read 2 + 3, and both were reported as 5. Fixed on branch `fix/envy-type-switch-groups` with a
failing test first; the self-check drops from 62 to 60 FAIL, and exactly those two clear.

## What applying proposals 4 to 6 does to the self-check

Columbo's own all-rule gate today reports 62 FAIL cases. Measured on the envy-fix branch with
`dependencies: 7`, `feature-envy-foreign-accesses: 6` and `cosmetic-dependency-overlap: 0.5`, it
reports 37:

| Smell | Today | With the fix and proposals 4 to 6 |
|---|---|---|
| long-function | 12 | 12 |
| feature-envy | 24 | 10 |
| excessive-dependencies | 20 | 8 |
| high-cognitive-complexity | 4 | 4 |
| repeated-variant-decision | 2 | 2 |
| cosmetic-extraction | 0 | 1 |
| **Total FAIL** | **62** | **37** |

## Limits of this evidence

- External corpora are build-validated only, because downloaded code is never executed. A
  mutant that changes behavior without breaking vet would be scored, not discarded. Operators
  preserve behavior by construction, and on Columbo, where tests could run, build and test
  validity agreed on every mutant compared.
- The harness measures what Columbo catches and what refactorings conflict. It does not
  measure whether a natural-code finding is a false positive; the natural case counts above
  show how many findings a change adds or removes, not whether those findings were right.
  The one new cosmetic case was inspected by hand.
- Sites are sampled deterministically (25 per operator per corpus), so the counts are rates
  on a sample, not totals.
- The red-team corpus is one agent's 14 evasions. It shows that cheap evasions exist for every
  rule, not how often an agent will find them.
