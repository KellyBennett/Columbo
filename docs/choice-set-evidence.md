# Choice-set evidence

Every Columbo analysis collects repeated collection construction into the normal SQLite snapshot. No collector-specific flags or extra output files are needed.

```sh
columbo --output report.sqlite ./...
sqlite3 -readonly report.sqlite < docs/sqlite/advisories.sql
```

The snapshot stores group identities, constants, projection fields, declaration-linked sites, seed ordering, input expressions, source excerpts and refactoring leads. The CLI and GitHub summary read this saved evidence. Advisory groups do not independently add FAIL/WARN cases or alter the normal analysis verdict. See [SQLite evidence snapshots](sqlite-schema.md) for the relational schema and publication guarantees.

## Evidence collected

The collector looks for a top-level local short declaration initializing a map or slice with at least two distinct package-associated named constants, immediately followed by a range loop. The loop must traverse a resolved slice field and contain exactly one unconditional insertion into that same local: a map-key assignment or the builtin `append`. The inserted key/element must be a direct resolved field of the range value. Named constants are identified by declaration and value; collections and projected fields are identified by canonical declaring receiver type and field, not spelling. Imported package aliases and local names do not define identity.

Groups require at least two distinct named declarations with the same constants, collection-field schema, and projected-field schema. Map payloads do not participate in membership grouping; their source is retained. Fixed seed order is retained per site, while the group's constant set is sorted. IDs use a full SHA-256 of the normalized construction schema. Groups, sites and receipts have deterministic ordering. Input expressions and complete initializer/loop source excerpts are retained with module-relative physical coordinates. Coverage counts describe included production files and declarations with bodies examined, not proof of comprehensive semantic coverage.

## Recognition scope

Only included production Go sources contribute; generated files, configured exclusions and test files follow normal loader policy. Function literals, nested constructions, aliases between initialization and iteration, filtered or multi-statement loop bodies, early exits, helper-mediated construction, runtime/literal seeds, keyed slice literals, promoted fields, non-slice collections and unsupported insertion forms do not qualify. Initializer and loop payload expressions are limited to literals and references without calls.

The exact fact established is a repeated **construction recipe through the end of the recorded loop**. Subsequent calls can receive the collection; subsequent writes can alter it. This version does not perform lifetime alias/mutation analysis or certify the eventual returned contents. It does not equate runtime context instances: a fresh context and a frozen context can have different candidates despite sharing the same schema. Map/slice order, multiplicity, key collisions and payload differences remain relevant. Both the per-group `limits` text and source receipts preserve this distinction for reviewers.

Shared constants in outcome counters, execution dispatch, reserved-ID checks or factory registries alone do not qualify. An evidence-ID collection with different resolved field identities does not join an action-menu group. A single shared builder used by several consumers is one definition, not repeated ownership. Moving independent copies into separate builders still leaves independently constructed recipes.

Matching syntax and typed fields does not prove a shared responsibility or coordinated-change cost. Validate the proposed ownership with the surrounding behavior before changing application code. Near misses and unrecognized forms produce no evidence, not a clean bill of architectural health.

