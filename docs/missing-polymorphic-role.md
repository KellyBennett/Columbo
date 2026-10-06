# Cross-case correlation: missing polymorphic role

Correlations explain independently detected cases; they are not smells and add no
FAIL/WARN count, exit-status change, suppression, case ID, or GitHub annotation.
The current cases retain their clues, receipts, verdicts, and annotation locations.

A strong `missing-polymorphic-role` correlation requires:

1. A repeated-variant-decision case and selection-use-coupling cases joined by the
   same canonical typed variant domain, regardless of local variable names.
2. A common-role candidate containing the selected concrete implementations
   (subset participation is supported), with observed messages shared by the
   selected players at every participating site.
3. A complete, consistent mapping from each observed named variant to the concrete
   implementation selected for it. Pointer/value distinctions are preserved.

Selection contexts are grouped by canonical variant domain and observed message
surface. Without a domain, the concrete player set is also part of the grouping.
No file proximity, naming convention, or shared method declaration creates a join.
When several existing candidates cover the group, the smallest player set wins;
canonical candidate ID breaks ties deterministically. A named interface does not
clear a correlation while distributed selection remains.

## Conservative mapping boundaries

The selection scanner already proves which concrete origins reach collaboration.
Correlation associates those origins with their enclosing switch/equality-chain
arms using the same typed variant parser as RVD. Multi-label arms map every label.
Aliases and renamed locals retain the canonical domain and constant value.

Origins outside the controlling decision, default/else arms without an explicit
variant, and other unproved mappings prevent strong confidence. The analyzer does
not invent the complement of an open variant domain. Unsupported selection forms
remain subject to the existing SUC scanner's limits.

## Partial diagnoses

- RVD + SUC without a safe common role: distributed selection is established, but
  a common behavioral role is not.
- SUC + common role without RVD: collaboration may be expressible as a role, but
  repeated variant knowledge has not been established.
- Conflicting mappings: the same variant selects incompatible concrete identities.
  The diagnosis explicitly reports that a stable variant-to-player relationship
  cannot be established; all conflicting mappings remain visible in evidence.
- All three with incomplete mappings: shared behavior is established, but a complete
  variant-to-player mapping is not.

Suppressed member cases participate structurally. Compact output displays each
member's current suppression status and original verdict. Removing distributed
selection through a factory/composition boundary removes the corresponding cases
and correlation. Adding an interface alone does not.

## Identity and persistence

The ID hashes correlation kind, canonical variant domain (when available), sorted
unique member case IDs, and canonical role-candidate ID (when available). Source
coordinates, confidence, severity, suppression, history, and review metadata do not
participate. Moving source preserves identity when member identities are stable.

Schema version 3 adds `correlations`, `correlation_cases`,
`correlation_role_candidates`, `correlation_evidence`, and `correlation_guidance`.
Each database is one analysis snapshot. Foreign keys require every member and role
candidate to exist in that snapshot. Evidence links also require the receipt to
belong to the linked member case. Mapping receipts belong to the originating SUC
case; correlations reference them rather than copying source evidence. The role
surface is read through the linked role candidate.

The summary view and Checks API projection still count/project only cases.
Correlations render after individual cases and role evidence. Leads and avoid
advice are stored relationally and printed with the diagnosis.

## MPR-001 — provisional

Decision: emit deterministic correlations without changing enforcement.

Review note: “This diagnosis is derived by correlating independently enforced
evidence. The correlation itself does not add a CI failure.”

Human-review prompt: “Review whether the linked cases genuinely point to one role
and one selection boundary. Pay particular attention to the variant-to-implementation
mapping and inferred message surface.”

Strong leads identify the minimal role, consider an interface at the consumer,
centralize variant-to-player selection, pass/return the role, and rerun analysis
to verify knowledge moved. Avoid advice rejects interface-only changes, renamed
switch helpers, per-player facades preserving caller taxonomy, blanket suppression,
and treating interface creation as proof of reduced coupling.

`correlations_test.go` exercises semantic joins and non-joins, strong/partial
confidence, consistent/conflicting/incomplete mappings, subset participation,
case ordering and line shifts, suppression display, unchanged exit status,
factory removal, existing interfaces, policy metadata, and snapshot evidence.
