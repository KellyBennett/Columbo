# Calibration findings and tuning proposals

Status: proposals with evidence. This document changes nothing by itself. Three proposals
also exist as separate pull requests so each can be accepted or dropped on its own: #59 (a
Feature Envy bug fix), #60 (three default thresholds, stacked on #59) and #61 (a Cosmetic
Extraction rule change that amends SPEC). The rest are proposals only.

The question these proposals answer: when you run Columbo on a Go project and act on what
it reports, does the code become easier to maintain? The harness cannot answer that
directly, because it never judges whether a finding is right. It measures three things that
have to hold for the answer to be yes:

1. Columbo notices the smells it claims to notice.
2. A cosmetic edit cannot silence a finding more cheaply than a real refactoring clears it.
3. A real refactoring can reach green without tripping a different rule.

The [mutation calibration harness](../calibration/README.md) measures all three by changing
Go code in known ways, running Columbo before and after, and recording what Columbo noticed.

## Short answer

- **(1) holds.** Columbo's metrics and triggers are exact. On the canonical run, all 412
  injected smells the operators' own metric predicted should fire did fire, and none of
  the 442 predicted not to fire did.
- **(2) fails for every rule.** Each rule has a cosmetic edit that silences it, and three
  edits silence any rule. Until those are closed, a green Columbo run means the code
  avoided Columbo's shapes, not that it is better designed.
- **(3) mostly holds where a mechanical cure exists.** When a cure's own metrics said the
  target was cleared, Columbo always agreed. The cross-rule conflicts that remain cluster in
  two places: helpers produced by Extract Function, and Introduce Parameter Object followed
  by Feature Envy.

So today, acting on a Columbo finding in good faith tends to improve the code, but Columbo
cannot tell good faith from gaming. Proposals 1 and 2 are what would make a green run mean
something on its own.

## What was measured

- **Corpus.** Columbo itself and four pinned external modules: cobra v1.9.1, chi v5.2.1,
  BurntSushi/toml v1.5.0, google/uuid v1.6.0 ([corpus.txt](../calibration/corpus.txt)).
- **Operators.** 41 operators across all eleven smells. Injects add a smell (inverse
  refactorings), evades try to hide one cosmetically, cures apply the real refactoring, and
  neutral edits should change nothing. Each has a fixture module and a test.
- **Red team.** An agent that saw only the code, the `columbo` binary and its output, including
  each case's "avoid" guidance, was asked to silence 14 real FAIL cases in cobra and chi
  without improving the design. Its edits are frozen under
  [calibration/redteam](../calibration/redteam) and replay with identical outcomes on
  consecutive runs.
- **Validity.** Every mutant must pass `go vet`. External code is never executed, so external
  mutants are build-validated only and behavior preservation rests on how the operators are
  built. On Columbo's own module, build and full-test validity agreed on all 16 mutants
  compared, all from the Inline Function operator.
- **Scale.** The canonical run is 2,319 mutants from one harness version, sampled
  deterministically at 25 sites per operator per corpus. Its full report is
  [calibration/reports/canonical.md](../calibration/reports/canonical.md).
- **Sweeps.** Each proposed threshold reran the relevant operators (20 sites per operator per
  corpus) under the old and new value. The sweeps ran on an earlier harness version than the
  canonical run, so compare their numbers only with each other, not with the canonical run.

Reproduce with `calibration/calibrate.sh <db> all 25` (set `REDTEAM=1` to replay the red-team
corpus), then `calibrate report --db <db>`. Sweeps use `calibration/sweep.sh` and
`calibration/compare.sh`.

## Verdict by rule

All counts are from the canonical run.

- **Catches** is injected smells Columbo reported, out of those the operator's metric said
  should fire.
- **Survives a cosmetic evade** gives the evade and how many of its mutants Columbo stopped
  reporting.
- **Green after a real cure** counts cures that cleared the target without setting off
  another rule, out of cures that cleared the target. Cures that did not clear the target
  are left out: in every such case the cure's own metric also said the target was still over
  the limit, so they measure the mechanical operator, not Columbo.

| Rule | Catches | Survives a cosmetic evade? | Green after a real cure |
|---|---|---|---|
| long-function | 48 of 48 | Forwarding split: 20 of 77 | 45 of 52 Extract Function |
| high-cognitive-complexity | 10 of 10 | Forwarding split: 2 of 15 | 37 of 41 Extract Function |
| feature-envy | 84 of 84 | Copy fields to locals: 2 of 16; aliasing (red team) | 16 of 16 Move Function |
| excessive-dependencies | 125 of 125 | Private facade struct: 125 of 125 | 17 of 18 split by collaborators |
| data-clump | 29 of 29 | One anonymous struct parameter: 29 of 29 | 1 of 5 Introduce Parameter Object |
| long-parameter-list | 31 of 31 | One anonymous struct parameter: 31 of 31 | not measured |
| cosmetic-extraction | 20 of 109 forwarding splits | Bare block per call: 10 of 14 | no mechanical cure |
| duplicate-code (WARN) | 48 of 48 | Renaming and relabeling: 0 of 47 | no mechanical cure for structural clones |
| prose-comment | 26 of 26 | Prose in an unused string constant: 26 of 26 | 72 of 72 deletions |
| repeated-variant-decision | 11 of 11 | Map lookup instead of a switch: 13 of 13 | no mechanical cure on real code |
| selection-use-coupling | no natural site in 5 corpora | Identity-helper laundering, fixture only | fixture only |

The red team replays: 13 of its 14 evasions still silence their case. The one partly caught
(chi `walk`) moved its dependency case onto the new helper. By the agent's own account, 6 of
the 14 took under 3 lines and 3 more took a few lines, and several are shapes the case's own
"avoid" guidance names.

## Proposals

Ordered by how much each changes whether a green Columbo run means better code. 1 to 3 change
rule scope and need SPEC amendments. 4 to 6 are default thresholds measured by sweep. 7 to 9
are stability and coverage. 10 is a bug fix.

### 1. Close the three escapes that silence any rule (CAL-001)

The red team found three edits that remove a declaration from analysis entirely, so every
rule goes quiet at once.

- **A hand-written generated-code header.** Adding `// Code generated by hand. DO NOT EDIT.`
  to cobra's `command.go` removed all 41 of its cases with no warning. SPEC makes the
  generated-file exclusion mandatory and silent.
  *Proposal:* keep the exclusion, and emit a `generated-exclusion` warning for each excluded
  file with how many declarations it hid. In the staged gate, fail a newly generated file whose
  header names no generator.
  *Evidence:* `redteam/cobra-prose-comment`, `redteam/chi-long-parameter-list`.
- **A function literal in a package-level `var`.** Moving `middleware.RedirectSlashes`'s body
  into `var redirectSlashes = func(...) {...}` cleared its long-function, complexity 11 and 8
  dependencies, and nothing was reported on the new var. SPEC limits finding subjects to named
  function declarations.
  *Proposal:* make a package-level variable whose initializer is a function literal a finding
  subject, under the variable's qualified name.
  *Evidence:* `redteam/chi-long-function`.
  *Status:* adopted as FS-001 in SPEC's decision register. The red-team replay now kills
  `redteam/chi-long-function`. Function literals nested in other initializers, such as
  `http.HandlerFunc(func(...) {...})`, remain an escape.
- **A `//nolint:` prefix on prose.** `//nolint:godot // <the original prose>` is classified as
  a machine directive, because docs/prose-comments.md exempts nolint's trailing justification.
  *Proposal:* exempt the directive, and classify text after its second `//` as prose.
  *Evidence:* `redteam/chi-prose-comment`.

### 2. Close the cheap per-rule evades (CAL-002)

Each of these silenced most or all of the sites it was tried on.

| Evade | Survived | Proposed detection |
|---|---|---|
| Bare `{ }` block around each helper call | 10 of 14 | Treat a bare block statement as transparent when forming clusters. **Done in #61**: 0 of 14 survive on all five corpora and nothing else in the cosmetic family changes; with #60's overlap 0.5 as well, 0 of 55 survive and splits caught rise to 52 of 109 |
| One anonymous struct parameter instead of a clump | 29 data-clump, 31 long-parameter-list | Count an anonymous struct parameter's fields as parameters for both rules |
| Private facade struct holding the collaborators | 125 of 125 | Score the dependencies of a same-package unexported type's fields when the function only reaches them through it |
| Map from the variants instead of a second switch | 13 of 13 | Count a map literal keyed by a known variant set as a decision site |
| Prose moved into an unused string constant | 26 of 26 | Flag an unreferenced package-level string constant containing whitespace |
| Copying the envied value's fields into locals | 2 of 16 | Count distinct fields read through a value, including reads into locals |

#61 reverses a SPEC sentence ("an explicit nested block ends the current sequence"). A bare
block used for real scoping still ends at any control statement inside it, so only blocks
that contain nothing but straight-line calls join the surrounding sequence. Decide it together
with CE-001.

### 3. Give data-clump a one-pass green path (CAL-003)

Introduce Parameter Object, the cure Columbo's data-clump lead points to, set off another rule
in 4 of 5 measured cases. Feature Envy fired on the functions that now read the new object's
fields 5 or more times; Excessive Dependencies and Duplicate Code also fired where every
caller now repeats the same struct literal. Feature Envy is right about the second step: Move
Function onto an envied same-package type reached green in 16 of 16 cures. So the cure exists
but takes two passes, and an agent that stops after the first sees a new FAIL.

*Proposal:* change the data-clump lead to name both steps: introduce the type, then move the
behavior that reads those values onto it. Where the clump mirrors an existing struct's fields,
lead toward passing that struct whole (Preserve Whole Object) instead.

### 4. Lower cosmetic-dependency-overlap from 0.75 to 0.5 (CAL-004, in #60)

Most mechanical splits of real long functions are not reported. The probe on cobra's
`writePreamble` shows why: two helpers forwarding the same `(buf, name)` share 2 of 3
dependencies, an overlap of 0.67.

| Configuration | Forwarding splits of real long functions caught | Forwarding splits of inlined functions caught | Cures blocked by Cosmetic Extraction | Natural cosmetic cases, 5 corpora |
|---|---|---|---|---|
| 0.75 (today) | 20 of 100 | 44 of 59 | 0 | 0 |
| **0.5** | **49 of 100** | **47 of 59** | **0** | **1** |
| 0.25 | 63 of 99 | 51 of 59 | 0 | 18 |

The one natural case at 0.5 is Columbo's own `(*complexityVisitor).visitIfStmt`, which hands its
one parameter to two single-use helpers, `ifCondition(n)` and `ifBranches(n)`. That is a true
cosmetic split by the rule's own intent, and it is the only new case adjudicated by hand. 0.25
catches more but adds 17 cases nobody has judged. Lowering the overlap makes the provisional
CE-001 policy stricter, so decide it together with CE-001.

### 5. Raise dependencies from 5 to 7 (CAL-005, in #60)

| Configuration | Natural FAIL cases, 5 corpora (Columbo) | Injected collaborators caught past the line | Extract cures blocked by Excessive Dependencies | Parameter-object cures blocked by it | All conflicting cures |
|---|---|---|---|---|---|
| 5 (today) | 77 (20) | 100 of 100 | 3 of 6 conflicts | 1 of 4 | 11 |
| **7** | **26 (8)** | **100 of 100** | **0 of 3** | **0 of 4** | **8** |

Two findings point the same way. Extracting a helper copies the parent's parameter types into
the helper's signature, so splitting a function with 7 dependencies tends to yield two
functions that each still exceed 5. And 15 of Columbo's 20 cases have no extractable seam: the
split-by-collaborators operator searched every statement partition, using a dependency
estimator that matches Columbo's count on 1,511 of 1,518 functions. Those 15 are
single-purpose go/types and go/ast walks that use that many types. At 7 the dependency-split
cure has fewer places to apply (18 to 6 sampled sites) because fewer functions cross the line,
and conflicting cures drop from 11 to 8 rather than to 0, because some now stop on Feature
Envy instead.

What this does not show: whether the 51 findings that disappear were false positives. The
harness counts findings; it does not judge them. An alternative that keeps 5 is to discount
signature-only dependencies (types that only pass through a parameter or result), since those
are what extraction duplicates. That is a rule change and is not measured.

### 6. Raise feature-envy-foreign-accesses from 5 to 6 (CAL-006, in #60)

| Configuration | Natural FAIL cases, 5 corpora (Columbo) | Injected envy caught | Extract cures blocked by Feature Envy | Parameter-object cures blocked by it | Copy-to-locals evades surviving |
|---|---|---|---|---|---|
| 5 (today) | 78 (24) | 50 of 70 sampled moves | 5 | 4 | 2 |
| **6** | **49 (10)** | **45 of 61** | **2** | **3** | **1** |
| ratio 1.5 instead | 92 (34) | 49 of 70 | 4 | 5 | 2 |

Feature Envy fires at the margin: 19 of Columbo's 24 cases and 22 of the 54 external cases are
within one access of the limit, and 65 of 78 are free functions with no receiver. Its green path
is excellent where a move is possible (16 of 16), but Extract Function produces small free
helpers that read a parameter five times, which is where most of its conflicts come from. At 6
it reports about two-thirds as many natural cases and blocks fewer cures, at the cost of
injected cases sitting exactly at 5. Lowering the ratio to 1.5, which one operator family
suggested, adds 14 natural cases for one more injected catch, so it is not proposed.

One likely false-positive pattern was seen but not measured: a free function calling methods on
a local `strings.Builder` six times is reported as envy of the builder.

### 7. Stabilize duplicate-code identity under reordering (CAL-007)

Reordering declarations within one file changed duplicate-code case IDs in toml
(`lexLongUnicodeEscape`) and cobra (`MarkFlagsMutuallyExclusive`) in a way SPEC's
earliest-supporting-declaration rule does not predict. Every other rule kept or predictably
remapped its IDs across 430 neutral mutants. The sample is two cases in a WARN rule, so the
cost is suppression and history churn, not failed builds.
*Proposal:* derive the group anchor from a canonical order of fragments rather than the
detector's discovery order, and add a reorder fixture.

### 8. Measure selection-use-coupling on code that has it (CAL-008)

None of the five corpora has a natural selection-use-coupling case, and only two functions
return an interface with two or more concrete results. The rule is measured on fixtures only.
*Proposal:* keep SUC-001 provisional until a corpus with factories and strategies is added.

### 9. Repeated-variant-decision has no mechanical green path (CAL-009)

Every natural RVD case is either an enum domain (`reflect.Kind`, `nodeTyp`, `itemType`) or a type
switch over another package's interfaces (`go/ast`), where Go cannot add methods. The mechanical
Replace Conditional with Polymorphism cure only applies to same-package pointer types and found no
natural site. Together with the map-lookup evade, this rule today asks for a redesign it cannot
show is possible and accepts a cosmetic rewrite.
*Proposal:* add a visitor or table-driven lead for foreign and enum domains, and decide RVD-001
with that evidence.

### 10. Fixed: Feature Envy merged type-switch clause variables (#59)

Columbo keyed envy groups by `<symbol>:<name>@<offset>`, and every clause variable of a type
switch shares one offset, so reads spread over clauses merged into one group. On Columbo,
`typeComponents` read 2 + 1 + 0 + 2 fields of four different variables and `supportedNode` read
2 + 3, and both were reported as 5. Fixed test-first; the self-check drops from 62 to 60 FAIL,
and exactly those two clear.

## What #59 and #60 do to Columbo's own self-check

Columbo's all-rule gate reports 62 FAIL cases today. Measured with #59 and #60 applied:

| Smell | Today | With #59 and #60 |
|---|---|---|
| long-function | 12 | 12 |
| feature-envy | 24 | 10 |
| excessive-dependencies | 20 | 8 |
| high-cognitive-complexity | 4 | 4 |
| repeated-variant-decision | 2 | 2 |
| cosmetic-extraction | 0 | 1 |
| **Total FAIL** | **62** | **37** |

Fewer findings is not the same as better code. The drop shows how much each default moves the
gate; whether each removed case was noise is a judgment the harness does not make.

## Limits of this evidence

- The harness measures what Columbo catches, what evades it, and what blocks a cure. It does not
  measure whether a finding on natural code is right, or whether following it makes code better.
- External corpora are build-validated only. A mutant that changed behavior without breaking vet
  would be scored rather than discarded. The validity comparison on Columbo covers 16 mutants
  from one operator.
- Five modules, sampled at 25 sites per operator per corpus. Counts are rates on a sample.
- The red-team corpus is one agent's 14 evasions in one pass. It shows cheap evasions exist for
  every rule, not how often an agent will find them.
- The harness's prose cure keeps any comment written `//word` as machine input, which is broader
  than Columbo's allowlist, so 6 of 78 prose cures left a comment Columbo still reports.
