# Experimental choice-set evidence

`--choice-sets-output choices.json` opts into a typed evidence collector for repeated collection construction. It adds no smell, threshold, severity, suppression, FAIL/WARN case, correlation, or check annotation. Existing SQLite output and enforcement remain unchanged. The separate JSON file has its own version (`1`); it is not another rendering of the SQLite report, and saved SQLite snapshots do not contain this experimental evidence.

```sh
columbo --config investigation.yml --no-history \
  --output investigation.sqlite --choice-sets-output choices.json ./...
```

Both destinations must be fresh files. Paths are relative to the invocation directory unless absolute. `-` is not accepted as either output destination. Evidence write failures exit 2; an already completed SQLite snapshot can remain available if writing the separate JSON file fails. A successful evidence write does not change the normal 0/1 analysis verdict. The action does not enable or retain this opt-in file automatically; upload it explicitly when using the CLI in CI.

## Evidence collected

The collector looks for a top-level local short declaration initializing a map or slice with at least two distinct package-associated named constants, immediately followed by a range loop. The loop must traverse a resolved slice field and contain exactly one unconditional insertion into that same local: a map-key assignment or the builtin `append`. The inserted key/element must be a direct resolved field of the range value. Named constants are identified by declaration and value; collections and projected fields are identified by canonical declaring receiver type and field, not spelling. Imported package aliases and local names do not define identity.

Groups require at least two distinct named declarations with the same constants, collection-field schema, and projected-field schema. Map payloads do not participate in membership grouping; their source is retained. Fixed seed order is retained per site, while the group's constant set is sorted. IDs use a full SHA-256 of the normalized construction schema. Groups, sites and receipts have deterministic ordering. Input expressions and complete initializer/loop source excerpts are retained with module-relative physical coordinates. Coverage counts describe included production files and declarations with bodies examined, not proof of comprehensive semantic coverage.

An example is four definitions of `NO_ACTION`, `REQUEST_RESEARCH`, plus each `Context.Candidates` element's `ID`: one criteria map, two membership maps and one ordered slice. The output supplies one group with all supporting sites and a lead to review domain ownership of enumeration and membership. It does not prescribe an interface, Command hierarchy, shared provider parser, or automatic refactoring.

## Conservative scope

Only included production Go sources contribute; generated files, configured exclusions and test files follow normal loader policy. Function literals, nested constructions, aliases between initialization and iteration, filtered or multi-statement loop bodies, early exits, helper-mediated construction, runtime/literal seeds, keyed slice literals, promoted fields, non-slice collections and unsupported insertion forms do not qualify. Initializer and loop payload expressions are limited to literals and references without calls.

The exact fact established is a repeated **construction recipe through the end of the recorded loop**. Subsequent calls can receive the collection; subsequent writes can alter it. This version does not perform lifetime alias/mutation analysis or certify the eventual returned contents. It does not equate runtime context instances: a fresh context and a frozen context can have different candidates despite sharing the same schema. Map/slice order, multiplicity, key collisions and payload differences remain relevant. Both the per-group `limits` text and source receipts preserve this distinction for reviewers.

Shared constants in outcome counters, execution dispatch, reserved-ID checks or factory registries alone do not qualify. An evidence-ID collection with different resolved field identities does not join an action-menu group. A single shared builder used by several consumers is one definition, not repeated ownership. Moving independent copies into separate builders still leaves independently constructed recipes.

This is provisional evidence discovery. Matching syntax and typed fields does not prove a shared responsibility or coordinated-change cost. Validate the proposed ownership with the surrounding behavior before changing application code. Near misses and unrecognized forms produce no evidence, not a clean bill of architectural health.

## Validation

`TestChoiceSets*` covers map/slice representations and wrapped inputs, negative constructions and nearby counterexamples, identity/determinism, single-declaration ownership, production exclusions, fresh-file preservation, CLI integration and unchanged verdicts. The first dogfood target is Pickaxe main `82992d0ba0c8234bbf85f7db5ad33e1628b88616`, compared with its independently captured Squint review. Precision on other codebases and downstream agent-token savings remain unmeasured.
