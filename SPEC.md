# Columbo Specification

## Purpose

Columbo is a strict architectural code-smell detector for Go, designed for CI enforcement in agentic development. A smell is evidence pointing toward a deeper design issue, not merely "bad code."

Columbo MUST detect deterministic clues, derive defined smells, report each smell as a case, provide evidence-backed diagnoses and refactoring leads, fail CI for configured violations, and detect superficial metric-gaming refactors.

> **Columbo follows code smells to their architectural cause and leaves leads for fixing them.**

Core CI enforcement MUST NOT require an LLM. Identical source, Go toolchain, Git history, and configuration MUST produce identical enforcement results.

## Concepts

**Clue:** deterministic observation, e.g. function length 27, complexity 11, or a helper with one caller.

**Smell:** a defined rule over clues. v1 smells: Long Function, Long Parameter List, High Cognitive Complexity, Excessive Dependencies, Feature Envy, Data Clump, Cosmetic Extraction.

**Case:** the reporting/enforcement unit. In v1, one triggered smell produces one case; Columbo does not merge smells automatically.

**Diagnosis:** qualified, evidence-backed explanation of likely design pressure. Use language such as "appears to" or "may indicate," not claims that a principle is mechanically proven.

**Lead:** concrete direction toward better cohesion, ownership, responsibility, or dependency boundaries.

**Verdict:** `WARN` or `FAIL`. No case means pass.

## CLI and exit codes

```bash
columbo [flags] [packages...]
```

No packages means `./...`.

Required flags:
- `--config PATH`, default `.columbo.yml`
- `--format text|json`, default `text`
- `--no-history`
- `--version`

Exit codes are exactly: `0` = analysis completed with no FAIL cases; `1` = analysis completed with >=1 FAIL case; `2` = invalid invocation/configuration or analysis could not complete because of package loading, parsing, type checking, or internal error.

## Configuration

YAML schema:

```yaml
version: 1
severity:
  long-function: fail
  long-parameter-list: fail
  high-cognitive-complexity: fail
  excessive-dependencies: fail
  feature-envy: fail
  data-clump: fail
  cosmetic-extraction: fail
thresholds:
  function-lines: 10
  parameters: 4
  cognitive-complexity: 7
  dependencies: 5
  feature-envy-foreign-accesses: 5
  feature-envy-ratio: 2.0
  data-clump-size: 3
  data-clump-occurrences: 3
  cosmetic-min-helpers: 2
  cosmetic-dependency-overlap: 0.75
  cosmetic-parameter-overlap: 0.75
history:
  enabled: true
  max-commits: 500
exclude:
  - "**/*_generated.go"
  - "**/vendor/**"
```

Allowed severity values: `off`, `warn`, `fail`. Unknown keys are errors. Numeric thresholds must be positive; overlaps must be in `[0,1]`.

If the default config is absent, use defaults. If an explicitly supplied config is absent, exit 2. `--no-history` overrides config.

Exclusions are doublestar globs against module-relative slash paths. Files carrying the standard Go `// Code generated ... DO NOT EDIT.` marker are always excluded. Test files are analyzed.

## Metric definitions

**Function lines:** non-empty, non-comment source lines from first through last statement in the body. Signature and braces do not count. A physical line counts at most once.

**Parameters:** expand grouped declarations; receiver does not count.

**Cognitive complexity:** implement and pin the semantics of `gocognit` v1.2.0. Fixtures must lock expected values so dependency upgrades cannot silently alter results.

**Dependencies:** distinct external packages, interface-typed collaborators, or non-builtin named concrete types used by a function through calls, field access, parameters, results, or construction. Builtins, receiver type, and private same-file implementation-only types do not count.

## Smell rules

### Long Function

Trigger when logical lines > `function-lines` (default 10). Leads must emphasize cohesive responsibilities and ownership. Avoid guidance must explicitly reject arbitrary helper extraction solely to reduce lines.

### Long Parameter List

Trigger when parameters > `parameters` (default 4). Lead toward missing domain/parameter objects or responsibility boundaries.

### High Cognitive Complexity

Trigger when complexity > `cognitive-complexity` (default 7). Lead toward simplifying decision structure, clarifying early returns, or meaningful variant abstractions.

### Excessive Dependencies

Trigger when dependency count > `dependencies` (default 5). Do not recommend a facade whose only purpose is hiding the count.

### Feature Envy

For each method, count member accesses and calls against its receiver and each distinct foreign receiver value with known static named type. Trigger when one foreign receiver has >= `feature-envy-foreign-accesses` accesses (default 5) AND foreign accesses are >= `feature-envy-ratio` times own-receiver accesses (default 2.0). If own accesses are zero, the ratio condition is satisfied once the minimum foreign count is met.

### Data Clump

Within a package, normalize parameters by static type; names do not matter and repeated types retain multiplicity. Trigger for a set of >= `data-clump-size` types (default 3) occurring together in >= `data-clump-occurrences` distinct signatures (default 3). Report one case per maximal qualifying clump per package; a clump is maximal if no strict superset qualifies in at least the same signatures.

### Cosmetic Extraction

Detect private helper decomposition that shrinks a parent without creating a meaningful boundary.

A **candidate trivial helper** must be unexported, same-package, have exactly one static call site, be reachable from the analyzed parent through candidate helpers, not implement a known interface method, and not be referenced as a first-class function value.

For parent parameter/receiver set P and values forwarded to helper H, parameter overlap = `|H|/|P|`; if P is empty, 0.

Dependency overlap for helpers A/B is Jaccard similarity of dependency sets; if both sets are empty, 0. Cluster overlap is mean pairwise similarity.

A sequential cluster exists when at least `cosmetic-min-helpers` candidate helpers (default 2) are called from the same parent in source order with no intervening non-candidate call except builtins, logging, or error wrapping.

Trigger Cosmetic Extraction only when:
1. a sequential cluster qualifies;
2. mean parameter overlap >= `cosmetic-parameter-overlap` (default 0.75);
3. mean dependency overlap >= `cosmetic-dependency-overlap` (default 0.75); and
4. virtual inlining makes the parent violate Long Function or High Cognitive Complexity.

Weak names such as `stepOne` may be additional clues but MUST NOT affect verdicts.

## Virtual inlining

Virtual inlining is analysis-only. Recursively replace candidate-helper call sites with their bodies for metric calculation. Stop at non-candidate callees and at the first repeated function in a recursion cycle.

Expanded lines replace each inlined call line with the helper body's executable lines. Expanded cognitive complexity is calculated over equivalent expanded control flow.

Virtual metrics are used only for Cosmetic Extraction in v1 and MUST NOT independently trigger Long Function or High Cognitive Complexity.

## Git history

When enabled, inspect at most `max-commits` commits reachable from HEAD, newest first. History is supporting evidence only in v1 and MUST NOT change verdicts.

If Git is unavailable, the target is not a worktree, or history is unreadable, continue static analysis and emit one stderr warning. Do not exit 2 for history failure.

## Case identity and ordering

Canonical identity:

`<smell-id>|<module-relative-file>|<qualified-symbol>|<secondary-key>`

Data Clump uses its sorted normalized type multiset as secondary key; otherwise secondary key is empty.

Displayed ID: `C-` plus first 10 lowercase hex characters of SHA-256 of the canonical identity.

Sort cases by file path, starting line, smell ID, then case ID.

## Suppressions

Only source suppressions exist in v1:

```go
// columbo:ignore <smell-id> -- <justification>
```

The comment must immediately precede the declaration (other comments may intervene), name exactly one known smell, and contain >=10 non-whitespace justification characters. Malformed suppressions exit 2. Suppression applies only to cases whose primary symbol is that declaration. Data Clump cannot be suppressed in v1. Applied suppressions appear in all output. Unused suppressions warn but do not fail. There is no ignore-all suppression.

## Output

Text output for every WARN/FAIL case MUST contain these sections in order:

```text
CASE <id> — <symbol>
VERDICT
SMELL
CLUES
WHY THIS MATTERS
DIAGNOSIS
LEADS
AVOID
RECEIPTS
```

Standard explanations, diagnoses, leads, and avoid guidance are deterministic templates covered by golden tests. Receipts use module-relative `file:start-end`.

Finish with:

`Columbo: <N> failed, <N> warned, <N> suppressed`

or, with no cases:

`Columbo: no cases`

Use ANSI only when stdout is a TTY.

`--format json` emits one JSON document and no prose on stdout:

```json
{"version":1,"summary":{"failed":0,"warned":0,"suppressed":0},"cases":[],"suppressions":[],"warnings":[]}
```

Each case includes: `id`, `smell`, `verdict`, `symbol`, `file`, `start_line`, `end_line`, `clues`, `why`, `diagnosis`, `leads`, `avoid`, and `receipts`. Arrays and cases use deterministic ordering. Fatal execution diagnostics go to stderr; JSON analysis warnings belong in `warnings`.

## Implementation constraints

Use the Go standard analysis/type ecosystem (`go/packages`, AST, and type information) for source analysis. Columbo may reuse mature libraries for pinned metric algorithms but MUST own its smell rules, case model, output, configuration, virtual-inlining logic, and fixtures.

v1 does not shell out to golangci-lint, Revive, or an LLM.

## Acceptance requirements

The repository MUST include fixture-based tests proving:

1. every smell triggers at its threshold boundary and not below it;
2. every default FAIL produces exit 1;
3. WARN-only runs exit 0;
4. malformed config/suppression and load/type failures exit 2;
5. case IDs and ordering are stable;
6. text and JSON outputs are deterministic;
7. legitimate delegation across non-candidate boundaries is not virtually inlined;
8. trivial single-use helper extraction that meets Cosmetic Extraction conditions remains red;
9. refactoring into helpers with sufficiently distinct parameter/dependency sets clears Cosmetic Extraction when other configured smells also clear;
10. history availability never changes a v1 verdict;
11. generated/excluded files are ignored;
12. suppressions require justification and are auditable.

Include adversarial fixtures where a long function is "fixed" using `stepOne/stepTwo/stepThree`-style helpers. Columbo MUST still fail when the defined Cosmetic Extraction conditions are met.

## Non-goals for v1

Columbo does not replace the compiler, gofmt, go vet, Staticcheck, security scanners, or correctness linters. It does not automatically rewrite code. It does not use an LLM for enforcement. It does not automatically merge multiple smells into architectural cases. Git-history evidence does not affect verdicts.

## Definition of done

v1 is done when the CLI, configuration, seven smell rules, virtual inlining, suppressions, text/JSON output, stable case identity, optional history evidence, exit semantics, and acceptance fixtures above are implemented and all tests pass.

There are no implementation questions intentionally deferred by this specification. If an implementation detail is not externally observable and does not alter these requirements, the implementer may choose it.
