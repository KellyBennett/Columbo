# Ordered refactoring stages

Run `columbo --staged --output report.sqlite ./...` to receive a bounded task for
one active stage. Without `--staged`, legacy rule verdicts and exit behavior are
unchanged. The GitHub Action exposes the same opt-in as `staged: 'true'`.

| Order | Stage | Collectors required to clear |
| --- | --- | --- |
| 1 | Untangle Behavior | `nested-field-decision`, `variant-coordination` |
| 2 | Assign Ownership | Pending definition; cannot be cleared |

Every invocation analyzes the current source and evaluates the ordered membership
list from the top. A defined stage clears only when every member has zero issues.
The first uncleared stage is active; stages below it are locked. After an edit
removes the untangling findings, the next run marks Untangle Behavior cleared/off
and Assign Ownership active/on. There is no persisted advancement switch: a new
untangling finding on a later run activates Untangle Behavior again.

Untangle Behavior asks the agent to gather each behavioral category into coherent
paths or methods, preserving behavior, ordering and boundaries. Temporary
duplication is acceptable. Do just enough to address the current findings, then
rerun for fresh evidence. Ownership, common roles, sharing and deduplication belong
to the later stage. Assign Ownership gives those gathered behaviors appropriate
owners, but its collector membership and completion criterion are deliberately
undefined in this version. Its task includes available repeated-variant-decision and
selection-use-coupling case IDs, source locations, diagnoses and refactoring leads
as review context, never as completion gates. An active pending-definition stage
is not completion.

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
- Exit 0: no active-stage issues; currently this reaches **Assign Ownership,
  pending definition**, not a claim that refactoring is complete.
- Exit 2: analysis, snapshot, or output error.

The GitHub check likewise succeeds at the pending-definition boundary while its
summary explicitly states that completion is undefined. Warnings remain visible.
Use the active task and status as the agent's stopping boundary, not a green check
as proof that all stages are done.

Schema 6 stores `refactoring_stages` in order, `stage_collectors` with membership
and issue counts, and an `active_stage_issues` view joining membership to the
advisory groups and their source receipts. Legacy snapshots have no stage rows.
See [the stage query](sqlite/stages.sql). Order and membership are centralized in
`refactoringStages`; adding a later defined stage does not require renderer or
collector-specific stage checks. Membership is intentionally code-defined in this
version; no speculative user-configurable workflow framework is introduced.
