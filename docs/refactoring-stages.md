# Ordered refactoring stages

Run `columbo --staged --output report.sqlite ./...` to receive a bounded task for
one active stage. Without `--staged`, legacy rule verdicts and exit behavior are
unchanged. The GitHub Action exposes the same opt-in as `staged: 'true'`.

| Order | Stage | Collectors required to clear |
| --- | --- | --- |
| 1 | Untangle Behavior | `nested-field-decision`, `variant-coordination` |
| 2 | Assign Ownership | `category-selected-behavior` (initial bounded gate) |

Every invocation analyzes the current source and evaluates the ordered membership
list from the top. A defined stage clears only when every member has zero issues.
The first uncleared stage is active; stages below it are locked. After an edit
removes the untangling findings, the next run marks Untangle Behavior cleared/off
and evaluates Assign Ownership against its initial gate. There is no persisted advancement switch: a new
untangling finding on a later run activates Untangle Behavior again.

Untangle Behavior asks the agent to gather each behavioral category into coherent
paths or methods, preserving behavior, ordering and boundaries. Temporary
duplication is acceptable. Do just enough to address the current findings, then
rerun for fresh evidence. Ownership, common roles, sharing and deduplication belong
to the later stage. Assign Ownership gives those gathered behaviors appropriate
owners. Its initial gate is now defined: zero category-selected-behavior findings.
Clearing it means only that this bounded gate found no issues, not that all
architectural ownership is correct. Its task includes available repeated-variant-decision and
selection-use-coupling case IDs, source locations, diagnoses and refactoring leads
as review context, never as completion gates. The stage framework still supports pending future definitions; neither current
stage is pending.

## Collector scope

`nested-field-decision` is the existing advisory collector for repeated nested
field decisions with guarded field updates. `variant-coordination` is the existing
coordination evidence attached to repeated-variant-decision cases: its
`variant-shared-write` and `variant-write-read` clues are two issue shapes from
one collector. In staged mode each such clue becomes an advisory with the original
source receipts and uncertainty details. They are not two independent collectors.

Coordination uses the same domain eligibility, `repeated-variant-sites` and
`repeated-variant-variants` thresholds, supported syntax, and lexical checks as
legacy analysis. Stage collection ignores the legacy smell's severity and
suppressions so setting it to `off`, `warn`, or suppressing a case cannot hide
coordination findings. It does not broaden detection to previously ineligible
domains. Existing source exclusions and configured thresholds still define the
scope of analysis. No new collector tuning is introduced.

A repeated-variant-decision case with no coordination clues does **not** block
Untangle Behavior. For example, a coherent Quote operation and a separate Label
operation can still repeat a discriminator while leaving no coordination evidence.
Broader smell findings remain available for later investigation in the snapshot.
Zero findings mean only that these bounded collectors raised no issues in the
analyzed scope; they do not prove behavioral correctness or good design.

## Outputs and enforcement

In staged mode, CLI and GitHub output report the ordered stage states, active task,
member issue counts, and only active-stage issues. SQLite retains all ordinary
cases, suppressions, advisories and provenance alongside the stage results.
Unrelated legacy failures, including duplication, do not drive staged annotations
or the staged exit code. Existing non-staged CI remains strict; adopting staged
mode is an explicit workflow choice, not an automatic change to existing jobs.

- Exit 1: the active stage has collector issues.
- Exit 0: both configured stages are cleared; not a claim of architectural completeness.
- Exit 2: analysis, snapshot, or output error.

The GitHub check likewise succeeds when both configured stages clear and states
the bounded meaning of clearing. Warnings remain visible.
Use the active task and status as the agent's stopping boundary, not a green check
as proof that all stages are done.

Schema 8 stores `refactoring_stages` in order, `stage_collectors` with membership
and issue counts, and an `active_stage_issues` view joining membership to the
advisory groups and their source receipts. Legacy snapshots have no stage rows.
See [the stage query](sqlite/stages.sql). Order and membership are centralized in
`refactoringStages`; adding a later defined stage does not require renderer or
collector-specific stage checks. Membership is intentionally code-defined in this
version; no speculative user-configurable workflow framework is introduced.

## Category-selected behavior contract

`category-selected-behavior` runs only in opt-in staged mode and is not a new
legacy severity rule. One finding describes one selector with at least two distinct
resolved free functions executing on the selector subject. Strings (including raw
literals and named string types) and integer category fields are supported. The
subject is the resolved value path immediately preceding the selected field.

Supported decisions are tagged switches with constant case expressions, and
`==` if/else chains over the same resolved field (constant on either side). Each
action branch consists of one direct call expression or one return-call. Every
action must resolve to an included local declaration with an identical non-generic,
non-variadic signature and pass the same subject at the same first matching
argument position. Other arguments must be resolved variable/field paths or
constants. Zero results and basic scalar results are supported. Empty branches,
unlabeled break/continue and bare returns are retained as explicit no-op/control
transfer evidence. An omitted default stays omitted; no exhaustiveness is inferred.

Receipts identify the selector, every branch, category-to-callee mapping, call and
callee declaration. The candidate callable signature and zero-based subject
argument index are recorded. Resolved parameter-field accesses and direct writes
in the callees support shared-state evidence; access is not a precise read-effect
claim. Sharing state is supporting evidence, not a gate or proof of semantics.

Unsupported shapes include aliases, assignments or multiple statements in branches,
computed arguments, nested decisions within branches, boolean combinations,
fallthrough, go/defer calls, closures, method/function-variable dispatch and external
callees. Factory selection returning functions or objects does not trigger: function
values are not direct calls, and calls returning aggregate, pointer, interface or
callable results are conservatively excluded. This also intentionally misses some
action functions returning those types. No broad flow or alias analysis is claimed.

A function-valued selection boundary followed by invoking the selected operation
can clear this gate without introducing any new type. Preserve default and no-op
behavior when considering any restructuring; static compatibility alone does not
prove the callees implement the same operation or that the existing design is wrong.

## Repeated guarded mutations (advisory, no new stage)

The existing `repeated-guarded-update` collector identifier now describes a broader
operation schema: read a resolved field in a comparison, then write that same
field on the same resolved subject. Two or more occurrences group only when the
field identity, normalized comparison and normalized mutation match. Examples:

```go
if account.Balance >= amount { account.Balance -= amount }
if job.Status == Pending { job.Status = Running }
```

Supported subjects are direct declared integer, string or boolean fields, including
named types. Checks use `==`, `!=`, `<`, `<=`, `>`, `>=`, subject to Go type checking.
A selected comparison can read the field within scalar arithmetic expressions.
Each candidate body contains one assignment, compound assignment or `++`/`--`.
Integer arithmetic/bitwise operations, string concatenation and unary `+`, `-`,
`^`, `!` retain their structure, operand order and types. No general algebraic
simplification or commutative reordering is attempted.

Normalization removes parentheses and reverses direct-field comparisons (`amount
<= account.Balance` matches `account.Balance >= amount`). It expands compound
assignments to their corresponding binary write expression. Integer `field++`,
`field += 1` and `field = field + 1` match; subtraction keeps operand order.
Literal constants retain checked value/type. Named constants retain their resolved
declaration identity as well as value/type, so different enum labels are not
conflated merely because they share a value. Constant arithmetic retains its
syntax; it is not folded across named constants. Literal constant conversions can
normalize to their checked constant; other constant conversions retain the operand
schema. Runtime calls/conversions are excluded.

Scalar parameters become typed input roles in first-use order within the selected
comparison, then the write. That same mapping is used throughout each occurrence:
checking `amount` and subtracting `amount` differs from checking `amount` and
subtracting `fee`. Corresponding parameters in different declarations may have
different names or positions. Local aliases, shadowed local inputs and global
variable inputs do not receive parameter roles. Direct sibling fields on the same
subject retain their resolved field identity. Inputs through other subject objects
are excluded. Distinct parameters/receivers can share a schema without denoting the
same runtime object; identifier spelling never establishes identity.

Pure-syntax conjunction context is supported, including `item.SellIn < 0 &&
item.Quality < 50`. The first comparison reading the mutated field is the selected
check; other conjuncts remain context, not part of its group identity. Receipts
retain the full condition, selected check, write, input-role bindings and enclosing
if conditions. Full condition order and branch context must be preserved. Scalar
reads/arithmetic can still panic, enclosing conditions can have effects, and this
is not concurrency/effect analysis or proof that extraction is safe. The collector
never asserts a universal domain invariant or safe unconditional clamping.

Conservative exclusions: init/else, multiple body statements, OR clauses, runtime
calls, pointer/indexed/promoted/nested target fields, closures, unclear aliases
between guard/write subjects, unsupported input expressions, floating-point,
complex, interface or aggregate mutation types, and conditions without a supported
comparison. A multi-statement guard beginning with an increment remains excluded.
No sequence/algorithm recognition, alias analysis or interprocedural inference is
attempted. Short-circuit context is never stripped to claim an unconditional write.

The direct-action category collector does not establish membership for a
function-returning selector. This collector does not infer behavior families;
names and historical reports are not substitute evidence. Existing ownership
eligibility remains unchanged and factories stay clear.

No third gate is defined. Ordinary CLI/GitHub summaries display this advisory
without changing verdicts/exit status. Staged summaries remain gate-focused;
inspect either snapshot with [the advisory queries](sqlite/advisories.sql), filtering
`advisory_groups.kind = 'repeated-guarded-update'`, or run ordinary analysis to a new
output path. Schema 8 is unchanged; normalized-check/normalized-write values and
mutation-input receipts use the existing tables. Thresholds/suppressions are unchanged.
