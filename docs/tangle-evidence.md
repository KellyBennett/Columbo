# Nested field-decision evidence

Every Columbo analysis collects nested field decisions and writes a version-1 JSON file beside the SQLite snapshot. For `--output report.sqlite`, its default path is `report.sqlite.tangles.json`. `--tangles-output PATH` overrides the destination; it does not enable collection. The GitHub Action retains the file automatically.

The evidence supplies source receipts and a refactoring lead. It does not add independent FAIL/WARN cases, correlations or annotations, and does not change normal smell verdicts. The JSON is separate from the SQLite snapshot.

## Evidence contract

Within each included production function/method, the collector groups comparisons by resolved direct field and receiver expression. String and integer fields may use built-in or named types. Constant operands may be literals or named constants. Equality, inequality, AND, OR and NOT are traversed; the complete condition is retained so negation and boolean context are not discarded or presented as positive case arms.

A group requires all of:

- At least three distinct if conditions over that field/receiver.
- At least two distinct constant values, each mentioned in at least two conditions. Repetition within one condition counts once.
- A same-group decision nested inside a then block or explicit else block of another; an ordinary else-if ladder alone is insufficient.
- At least two conditions with writes under their branches, collectively targeting at least two other direct fields on that receiver. Assignments, compound assignments and increment/decrement count. Nested writes in either branch count lexically and may therefore appear in multiple receipts.

These fixed eligibility criteria define the recognized evidence shape, not configurable severity thresholds. Groups carry declaration/field identities, sorted repeated values and written-field identities, full condition receipts, comparison receipts, and guarded assignment receipts with physical source ranges. Output is deterministic for an unchanged source snapshot. IDs include the first condition's file/offset and may change after source movement; there is no cross-revision identity promise.

Receiver paths resolve variable objects, direct named-struct fields, pointer dereferences and simple indexes. Shadowed variables, distinct roots and distinct index expressions remain separate. Promoted fields, calls as receiver paths and computed indexes are unsupported. Function literals are excluded. Existing generated/test/config exclusions apply.

## Interpretation and limits

The lead suggests reviewing whether variants should own their update behavior. It does not prove that polymorphism is appropriate, that conditions are feasible, that the same runtime object is accessed, or that a write executes. No alias, mutation-between-checks, control-flow, interval, or call-effect analysis is performed. A constant under NOT is only a mentioned comparison value, not an asserted allowed variant.

Return-only validation, checks on unrelated receivers, single-output updates, switches and helper-mediated behavior are intentionally outside v1. A validator that mutates several fields can still qualify: this heuristic cannot distinguish every reasonable validation routine from entanglement. There is no claim of general precision or recall.

## Files and failure behavior

The output path must be fresh; stdout (`-`) and overwrites are rejected. Sidecars use exclusive creation with mode 0600. Ordinary SQLite publication occurs first, then choice-set output, then tangle output. A subsequent write failure returns exit 2 and can leave earlier completed outputs in place; publication is not transactional across files. Normal smell verdicts are unchanged when evidence publication succeeds.
