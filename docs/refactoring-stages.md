# Ordered refactoring stages

Run `columbo --staged --output report.sqlite ./...` to receive a bounded task for
one active stage. Without `--staged`, legacy rule verdicts and exit behavior are
unchanged. The GitHub Action exposes the same opt-in as `staged: 'true'`.

| Order | Stage | Collectors required to clear |
| --- | --- | --- |
| 1 | Untangle Behavior | `nested-field-decision`, `variant-coordination` |
| 2 | Assign Ownership | `category-selected-behavior` (initial bounded gate) |
| 3 | Consolidate Shared Behavior | `repeated-guarded-update` |

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
to later stages. Assign Ownership gives those gathered behaviors appropriate
owners. Its initial gate is now defined: zero category-selected-behavior findings.
Clearing it means only that this bounded gate found no issues, not that all
architectural ownership is correct. Its task includes available repeated-variant-decision and
selection-use-coupling case IDs, source locations, diagnoses and refactoring leads
as review context, never as completion gates.

Consolidate Shared Behavior asks the agent to consolidate repeated responsibilities
into shared implementations while preserving behavior and the guarantees of earlier
stages. Resolve current evidence, then rerun. Its completion gate is zero
`repeated-guarded-update` groups, including supported caller/helper boundary groups.
Preserve surrounding conditions, evaluation order and effects: evidence is not
proof of safe extraction or outer-guard removal. Zero groups do not establish that
all shared responsibilities have been consolidated. Earlier-stage regressions
reopen the first affected stage and lock consolidation again. Locked-stage evidence
and counts remain in SQLite; only active-stage issues are presented for action.
The stage framework supports pending future definitions; none of the three
configured stages is pending.

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
- Exit 0: all three configured stages are cleared; not a claim of architectural completeness.
- Exit 2: analysis, snapshot, or output error.

The GitHub check likewise succeeds when all three configured stages clear and states
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
`==` if/else chains over the same resolved field (constant on either side). An understood
action branch consists of one direct call expression or one return-call, optionally
preceded immediately by a single branch-local function binding (`f := action` or
`var f = action`, including an explicit function type). The binding must resolve
to a free-function declaration and the invocation must use that exact local
variable. Reassignment, alias chains, closures and bindings outside the branch
are not followed. Every contributing action must resolve to an included local declaration with an identical non-generic,
non-variadic signature and pass the same subject at the same first matching
argument position. Other arguments must be resolved variable/field paths or
constants. Zero results and basic scalar results are supported. Empty branches,
unlabeled break/continue and bare returns are retained as explicit no-op/control
transfer evidence. An omitted default stays omitted; no exhaustiveness is inferred.

At least two understood actions with distinct compatible callees are required.
Unsupported branches do not invalidate supported actions; they remain explicit
unknown context and never count as actions or inferred no-ops. Compatibility is
checked across all resolved contributing actions, not assumed for unknown branches.

Receipts identify the selector, every branch, category-to-callee mapping, call and
callee declaration. Local bindings have their own source receipts. Unsupported
branches are marked `category-unknown-branch`. The candidate callable signature and zero-based subject
argument index are recorded. Resolved parameter-field accesses and direct writes
in the callees support shared-state evidence; access is not a precise read-effect
claim. Sharing state is supporting evidence, not a gate or proof of semantics.

Unsupported shapes include general aliases, assignments or multiple statements
other than the immediate local binding and call,
computed arguments, nested decisions within branches, boolean combinations,
fallthrough, go/defer calls, closures, method dispatch, unresolved function-variable
dispatch and external callees. These branches are retained as unknown context when
other branches supply sufficient evidence. Factory selection returning functions or objects does not trigger: function
values are not direct calls, and calls returning aggregate, pointer, interface or
callable results are conservatively excluded. This also intentionally misses some
action functions returning those types. No broad flow or alias analysis is claimed.

A function-valued selection boundary followed by invoking the selected operation
can clear this gate without introducing any new type. Preserve default and no-op
behavior when considering any restructuring; static compatibility alone does not
prove the callees implement the same operation or that the existing design is wrong.

## Repeated guarded mutations (third-stage gate; ordinary-mode advisory)

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
Each candidate is a direct assignment, compound assignment or `++`/`--` within
a guarded body, which may contain additional statements.
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
if conditions. Multi-statement bodies have a complete `guarded-body` receipt: the
guard also controls other statements, so a finding does not recommend replacing
the whole `if`. Full condition order and branch context must be preserved. Scalar
reads/arithmetic can still panic, enclosing conditions can have effects, and this
is not concurrency/effect analysis or proof that extraction is safe. The collector
never asserts a universal domain invariant or safe unconditional clamping.

Before each candidate, only effect-free scalar local declarations, assignments,
increments/decrements and empty statements are traversed. Their resolved targets
must be independent of every value referenced by the full condition and mutation.
Field/indirect/global writes, subject or input reassignment, calls, aliases and
control flow conservatively block a later candidate. A prior checked-field write
invalidates the evidence even if the comparison might still hold. Later statements
do not invalidate an already-supported candidate. Nested guards are independently
examined with their own checks; no outer check is propagated through them.

Conservative exclusions: init/else, OR clauses, runtime
calls inside check/write expressions, pointer/indexed/promoted/nested target fields, closures, unclear aliases
between guard/write subjects, unsupported input expressions, floating-point,
complex, interface or aggregate mutation types, and conditions without a supported
comparison.
No sequence/algorithm recognition, alias analysis or transitive call inference is
attempted. Short-circuit context is never stripped to claim an unconditional write.

The direct-action category collector does not establish membership for a
function-returning selector. This collector does not infer behavior families;
names and historical reports are not substitute evidence. Existing ownership
eligibility remains unchanged and factories stay clear.

Consolidate Shared Behavior requires zero groups from this collector. Ordinary CLI/GitHub summaries display this advisory
without changing verdicts/exit status. Staged summaries remain gate-focused;
inspect either snapshot with [the advisory queries](sqlite/advisories.sql), filtering
`advisory_groups.kind = 'repeated-guarded-update'`, or run ordinary analysis to a new
output path. Schema 8 is unchanged; normalized-check/normalized-write values and
mutation-input receipts use the existing tables. Thresholds/suppressions are unchanged.

### Guard repeated across a helper boundary

The collector also recognizes a direct call to an included free function with
one pointer parameter and no results. The caller argument must be a resolved
variable, and the helper mutation must target that exact parameter's direct field.
The caller check and helper check must normalize to the same typed comparison
and field identity. Constants and same-subject sibling fields are supported;
additional scalar parameters, argument expressions, methods, function bindings,
return/factory calls, external functions and transitive calls are excluded.

The helper guard must be top-level, reached through only independent scalar
operations. The existing safe-prefix rules apply between the caller guard and
call and between the helper guard and mutation. Unknown effects, control flow,
subject reassignment and possibly aliased writes block inference. Init/else,
closures and nested helper guards are excluded. No recursion is followed.
The ordinary helper mutation remains one existing site; the caller contributes
a separate guarded-call site, not a second copy of the helper mutation.

Receipts retain the caller condition, body context, call, helper declaration,
helper condition and mutation. The persisted call receipt names the resolved
callee; callee receipts belong to its existing declaration-owned site, including
when the helper is in a different file. This is evidence of duplicated boundary knowledge,
not proof that the outer guard is redundant or safe to delete: it may control
other behavior. Preserve all such work and its order when refactoring. This
evidence participates in the third-stage gate and remains advisory in ordinary
mode. Thresholds, legacy severities and schema are unchanged.
