# Common-Role Inference

Common-role inference is descriptive evidence for concrete players already implicated by selection/use or repeated variant decisions. It is not a smell, has no severity or threshold, and never creates a FAIL or WARN case or changes an existing verdict.

## Observed collaboration

For each eligible context, retain the concrete receiver types and the canonical messages actually invoked on each. The candidate surface is the intersection of those observed sets, with at least two implementations and one common message. Unused common methods do not contribute. A one-message candidate is strong only because the supported context proves that the players are alternatives.

Message identity includes method name (and defining package for unexported methods), parameter/result types and counts, and variadic distinction. Receivers and parameter/result names are excluded. Existing canonical type normalization resolves aliases. Go type checking and method-set lookup establish compatibility, including promoted methods and instantiated generic methods. An implicit address-taking invocation records the pointer player; it does not claim the value implements a pointer-only method.

Candidate identity is a full SHA-256 digest of the sorted canonical implementation identities and sorted canonical messages. Pointer/value identities remain distinct. Positions, local names, constructor names, parameter names, unused methods, file placement and explicit interface introduction do not change equivalent identity.

## Supported contexts

* Existing `selection-use-coupling` cases: retain each observation's reaching concrete origins, rather than assigning every observed message indiscriminately to every selected player. The candidate links to its case and adds an evidence-derived lead. Existing rule thresholds, exclusions and verdicts are unchanged.
* Direct branch-local messages: a switch or if/else chain can establish alternatives without an interface variable. Each contributing arm must have exactly one concrete receiver identity; nested decisions and closures do not contribute messages to the enclosing arm. These observations are collected while selection analysis is enabled. They are stored as role evidence without manufacturing a selection case. In particular, `Email{}.Send(...)` versus `SMS{}.Send(...)` produces evidence, not an additional verdict.
* Qualifying `repeated-variant-decision` groups: corresponding arms must map each repeated variant to one consistent concrete identity across supporting sites. Ambiguity or conflicting mappings reject inference for that group. Each site must establish common messages on at least two players before messages can be combined across sites. The resulting candidate links to the existing repeated-variant case.

This conservative first version omits unsupported or ambiguous observations. It does not scan arbitrary type pairs, infer semantics from names, follow constructor bodies, execute reflection, or use method values as proof of invocation. Direct receiver assertions do not contribute evidence. All emitted candidates have deterministic `strong` confidence; storage also accepts `supporting` for future explicitly supported weaker contexts.

## Existing interfaces

Search named interfaces in the loaded production/dependency type universe and anonymous interfaces used by a supporting selection. An exact method surface is recorded as `exact`. A strict superset is separately recorded as `compatible` only if every concrete player implements it. An interface already used at the supporting site is additionally recorded as `used`.

An exact match or already-used interface classifies the candidate as `existing role`; otherwise it remains `inferred role`. A merely available superset never replaces the minimal observed surface. All matches are retained in deterministic order; no arbitrary preferred interface is chosen. The interface-shaped sketch is canonical descriptive evidence, not generated compilable source or a required refactor.

## Persistence and reports

SQLite schema version **2** adds:

| Table | Evidence |
| --- | --- |
| `role_candidates` | Stable ID, canonical minimal interface, confidence and classification |
| `role_candidate_implementations` | Canonical concrete players |
| `role_candidate_messages` | Canonical observed common messages |
| `role_candidate_interfaces` | Exact, compatible and used interfaces |
| `role_candidate_receipts` | Typed declaration, source coordinates and observed message |
| `role_candidate_case_links` | Existing cases supported by the candidate |

Equivalent candidates merge across contexts, preserving receipts and all case links. Unlinked branch-local observations remain available in the same tables. No role table participates in summary totals or exit status. The compact report renders the minimal role and existing interfaces. Linked cases carry a `common-role` clue and the deterministic consumer-owned-role lead, including in the Checks API projection.

Readers and the repository action validate version 2; old version-1 snapshots should be read with their matching Columbo revision or regenerated. Existing snapshots are never rewritten.

See [role evidence queries](sqlite/role-evidence.sql) for standalone and case-linked inspection.

## Validation

Acceptance tests cover selected messages, minimal/multiple surfaces, exact/superset/loaded dependency interfaces, pointer/value and promoted generic method sets, renamed parameters and locals, aliases, constructor changes, moved files, added unrelated methods, interface introduction, unrelated shared methods, incompatible signatures and variadic forms, ambiguous variant mappings, if/switch contexts, persisted case links, unlinked evidence and unchanged verdicts. Existing selection and repeated-variant acceptance tests remain authoritative for those smells.
