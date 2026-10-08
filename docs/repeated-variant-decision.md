# Repeated Variant Decision

Status: accepted v1 rule; enforcement policy RVD-001 is provisional.

Columbo asks whether two places know the same variants, independently of whether their branch bodies contain duplicate code. Register `repeated-variant-decision` at default FAIL. `off` skips dedicated analysis. Both integer thresholds have minimum 2:

```yaml
severity:
  repeated-variant-decision: fail
thresholds:
  repeated-variant-sites: 2
  repeated-variant-variants: 2
```

## Domains and sites

Value domains are alias-normalized named Go integer, unsigned integer, or string types. Named booleans, float/complex types and raw primitives are excluded. The key is `value:<canonical-named-type>`; package import paths and instantiated type arguments establish identity. A value variant is the domain plus the exact Go constant value, independent of constant spelling or aliases.

An expression switch qualifies only if every non-default expression is a compile-time constant representable by its named discriminant type. A runtime expression excludes the entire switch. Multiple case expressions contribute separate variants; fallthrough has no effect.

A maximal if/else-if chain qualifies only if every conditional branch consists of equality or OR-connected equalities between the same discriminant expression and domain constants. Operand order and parentheses have no effect. The discriminant must be an identifier or identifier-rooted field selector chain; resolved objects establish structural equivalence, including shadowing. A final unconditional else contributes nothing. Comparisons other than equality, AND, calls as discriminants, assertions, unrelated conditions, bit tests, and standalone guard sequences are excluded. An ineligible chain's else-if tail is never interpreted independently.

Type domains are alias-normalized named interfaces, including generic instantiations. Keys are `type:<canonical-interface-type>`. Type switches over any, interface{}, and anonymous interfaces are excluded. Eligible variants are named concrete types and pointers to named concrete types, including supported instantiations. Aliases normalize to their targets. Nil and default contribute nothing; unsupported non-default variants exclude the entire site.

A site must explicitly distinguish at least two distinct variants. Branch bodies do not participate in eligibility or correlation. A site is identified by `<module-relative-file>:<physical-start-byte-offset>`. Decisions in nested literals remain attributed to their nearest enclosing named function/method; decisions without such an owner are excluded. Only included, selected production source contributes.

## Correlation

Let V(S) be the explicitly distinguished canonical variants at site S. For site threshold N, support(v) is the number of distinct sites containing v. R(D) contains variants with support(v) >= N. Supporting sites contain at least one variant in R(D).

Emit exactly one case per domain when there are at least N supporting sites and at least M repeated variants, where M is the variant threshold. For example, {A,B} and {B,C} do not trigger at defaults. {A,B}, {A,C}, {B,C} do trigger: all three variants have support 2. Do not manufacture pairwise cases or require exhaustive decisions.

## Evidence and identity

| Clue | Subject | Value / comparison |
| --- | --- | --- |
| variant-decision-sites | Canonical domain | Supporting site count >= N |
| repeated-variant-count | Canonical domain | Repeated variant count >= M |
| repeated-variant-set | Canonical domain | Sorted canonical variant list |
| variant-set | File:start-offset | Sorted complete site variant list |
| variant-support | Canonical repeated variant | Distinct-site support >= N |

Every supporting site has a `variant-decision` receipt covering its complete statement/chain, with domain subject and named declaration ownership. Every source expression establishing a repeated variant has a `variant-arm` receipt with canonical variant subject, exact physical range, and original source spelling. Preserve all alias spellings, including multiple occurrences at one site; they do not inflate support. Fabricate no receipts for default, else, nil, absent variants, or inferred complements.

Sort variants by canonical identity and sites by module-relative file then physical byte offset. SQLite retains all clues, receipt relationships, source spellings, and declaration ownership. Compact summaries and GitHub annotations show domain, repeated sets, support counts, all supporting locations and owners, plus policy review metadata. History continues to cover every participating source file.

The logical case identity uses smell and canonical domain only: the existing identity encoder receives empty file and symbol fields and the domain as discriminator. Repeated sets and the primary declaration never enter identity. Identity survives line shifts, file relocation, local renaming, switch/if conversion, aliases, reordered arms/OR clauses, branch-body changes, helper extraction, and changes to severity/history/suppression/policy. Additional repeated variants enrich the same case.

The enclosing declaration of the lexically first supporting site is the canonical primary anchor. Existing suppression grammar applies:

```go
//columbo:ignore repeated-variant-decision -- JUSTIFICATION
```

A primary-anchor directive suppresses the entire domain. Directives attached only to secondary declarations do not suppress it. An anchor change may make a previous directive unused; existing warning behavior applies.

## Guidance and pressure

### Coordination evidence

For an emitted value-domain case, Columbo also examines separate decisions over the same resolved variable or field path within each included function. It adds `variant-shared-write` clues when two decisions lexically write the same location, and `variant-state-overlap` clues when an earlier decision lexically writes a location that a later decision lexically reads. These clues retain the access types (earlier write; later read) without claiming that the assigned value reaches the later read. Readers also accept the former `variant-write-read` label in existing snapshots. Each clue links both decision roots and the two accesses through the existing SQLite clue/receipt tables; compact reports display the links and their qualifications.

This supplementary scan accepts constant equality/inequality comparisons, AND/OR combinations over one selector, and constant value switches. It can therefore connect a qualifying switch to nearby single-variant checks. Nested decisions over the same selector, including else-if refinements, belong to their outer decision rather than being counted as independent roots. Closures, decisions with initializers, indexed or indirect paths, type switches, and mixed-selector predicates are excluded. Different variables or receivers are not merged merely because they have the same type. No clues are emitted solely for repeated boolean capability or presence checks.

The links describe lexical accesses, not proven reaching definitions or runtime coordination. They retain comparison operators and constants, with exact source ranges for the full decision; a flattened comparison list is not a normalized truth table. Else/default may include undeclared values because Go's named integer and string types are open. Accesses are aggregated across branches, so mutually exclusive paths and writes of identical expressions can produce links. Calls, conversions, possible selector writes across the pair, and writes to the linked location between decisions are counted as explicit uncertainty. Aliasing, call effects, loops, helper-mediated flows, and mutation through pointers are not resolved. A shared rendering or logging call alone does not establish a link.

Use these clues to ask whether the decisions independently reconstruct one policy and should share a calculation or mapping. Inspect the source receipts and uncertainty before acting; a link does not establish that refactoring or polymorphism would help. Absence of links does not establish independence. Coordination evidence does not change case eligibility, thresholds, identity, suppression, or verdict.

Why: Repeated decisions over the same variant domain distribute knowledge of that taxonomy. Adding or changing a variant may require coordinated edits across otherwise unrelated code.

Diagnosis: Multiple decision sites repeatedly distinguish variants of the same typed domain. This may indicate distributed variant knowledge, a missing behavioral role, or a decision that should be centralized.

Leads:

- Examine whether callers can send the same message to interchangeable role players instead of selecting behavior themselves.
- If the variants select implementations, use a factory or construction boundary to select once and return a common behavioral role; callers then send messages through that role.
- If independent operations inspect the same variants, evaluate a visitor or adapter with one dispatch boundary; keep operation-specific behavior, traversal policy, and results with each operation.
- If branches merely map variants to data, consider a data-driven representation owned near the variant definition.
- Move behavior toward the object or role that has the knowledge required to perform it.

Avoid:

- Adding wrapper objects, broad visitor protocols, or no-op methods solely to clear the finding without improving cohesion or reducing coupling.
- Moving each decision into a differently named helper while retaining the repeated variant knowledge.
- Replacing switches with equivalent if/else chains.
- Introducing an interface while callers still select concrete implementations repeatedly.
- Renaming constants, aliases, variables, or files solely to evade correlation.

The rule does not prove polymorphism is appropriate or require an interface. A single factory switch is allowed. Centralizing a decision, using a common behavioral role, owning behavior near its data, using one shared mapping table, or removing obsolete variants/sites can clear the case. Changing branch implementations or merely extracting their bodies cannot.

Duplicate Code, Cognitive Complexity, Cosmetic Extraction, and Excessive Dependencies remain independent. No branch similarity, dependency overlap or cloned-token threshold enters this rule. Cloned switches can produce separate duplicate-code and repeated-variant-decision cases.

## RVD-001

Default FAIL deliberately tests whether repeated typed branching is a useful proxy for distributed architectural knowledge. Requiring proof of an eventual abstraction would make detection circular. This policy applies to every emitted case, including WARN and suppressed cases; OFF emits none. Policy metadata never changes verdict, identity or thresholds.

Review note: “This rule deliberately treats repeated interpretation of the same typed variant domain as architectural pressure even when the decisions may be intentional. This is a provisional dogfooding policy; its review note does not change the case verdict.”

Human-review prompt: “Show the human the variant domain, every supporting decision site, each site's variant set, and the repeated variants with their support counts. Evaluate a factory that selects role implementations once, or a visitor/adapter that centralizes variant dispatch while each operation owns its behavior and traversal. Also consider a shared data mapping. Compare cohesion and coupling before changing code or requesting suppression; the finding does not prove that a pattern is appropriate.”

Collect missing-role/centralization examples and clearly preferable repeated mappings. Acceptance coverage includes a missing-role example and a reasonable enum mapping that nevertheless fails, the support/grammar/exclusion/identity boundaries, independent clone findings, SQLite logical-row and compact-summary goldens for FAIL/WARN/OFF/suppression/history, GitHub annotation projection and review metadata. The production self-check runs ./... with default FAIL enabled.

Absence of a finding does not prove absence of distributed variant knowledge. Reflection, runtime taxonomies, raw values, map keys, naming conventions, comments, bitmasks, standalone guards and branch semantics remain outside v1.
