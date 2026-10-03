# Columbo Specification

## Purpose

Columbo is a strict architectural code-smell detector for Go, designed for CI enforcement in agentic development. A smell is evidence pointing toward a deeper design issue, not merely "bad code."

Columbo MUST detect deterministic clues, derive defined smells, report each smell as a case, provide evidence-backed diagnoses and refactoring leads, fail CI for configured violations, and detect superficial metric-gaming refactors.

> **Columbo follows code smells to their architectural cause and leaves leads for fixing them.**

Core CI enforcement MUST NOT require an LLM. Identical source, Go toolchain, Git history, and configuration MUST produce identical enforcement results.

## Motivation

Agentic coding changes the economics of strict CI enforcement. Columbo is intended to run in CI even when developers or coding agents never run it locally. A red build is deliberate feedback: code is submitted, Columbo detects architectural pressure, CI fails, and the coding agent receives enough evidence and guidance to make another design pass.

Merely enforcing metrics is insufficient because coding agents can satisfy metrics literally. A 30-line function can become five tiny private helpers while preserving exactly the same responsibility structure. Columbo therefore treats metrics as clues and attempts to distinguish meaningful decomposition from cosmetic decomposition.

The inconvenience is intentional. In an agentic workflow, the CI failure is the trigger that sends the agent back to reconsider the architecture.

## Design Principles

### Evidence Before Interpretation

Every higher-level conclusion must be traceable back to concrete evidence:

```text
Diagnosis
    ↓
Case
    ↓
Smell
    ↓
Clues
    ↓
Source / AST / type information / call graph / repository history
```

A developer or agent must always be able to ask why Columbo reached a conclusion and receive concrete receipts.

### Strict by Design

Columbo is not a passive suggestion engine. Teams may deliberately configure aggressive thresholds so ordinary development regularly encounters its guardrails. CI failure is a feature.

### Architecture Over Metrics

Metrics are evidence, not the objective. Columbo prefers better design over smaller numbers. Whenever practical, it detects attempts to satisfy structural metrics without improving cohesion, coupling, ownership, or responsibility boundaries.

### Make the Correct Fix Easier Than the Fake Fix

A failure must explain what Columbo observed, why it matters, where the evidence occurs, what design pressure it suggests, promising refactoring directions, and superficial fixes that will not resolve the case.

> **A Columbo violation should be harder to silence than to properly refactor.**

### Deterministic Core

Clues, smell rules, policy evaluation, case identity, and CI verdicts are deterministic. Architectural language may be heuristic, but it must be generated from deterministic evidence and templates. An LLM is not required for enforcement.



# Vocabulary and Reasoning Model

Columbo uses a deliberate reasoning ladder:

> **Clue → Smell → Case → Diagnosis → Lead → Verdict**

The distinction separates what Columbo observes, what it infers, what it investigates, what it suggests, and what CI enforces.

## Clue

A **Clue** is an objective, deterministic observation about the codebase.

Examples:

```text
ProcessOrder is 27 lines.
ProcessOrder has cognitive complexity 11.
ProcessOrder accepts 7 parameters.
PaymentGateway is referenced 9 times.
sendReceipt has exactly one caller.
Three private helpers share 87% of their dependency set.
```

Clues are mechanically reproducible and do not claim that code is bad. They are evidence.

## Smell

A **Smell** is a recognized design heuristic supported by one or more clues.

Examples include Long Function, Long Parameter List, High Cognitive Complexity, Excessive Dependencies, Feature Envy, Data Clump, and Cosmetic Extraction.

The metaphor is intentional: a smell is a **scent worth following**, not a declaration that code "stinks." A smell says that something about the structure deserves investigation; it does not prove the architecture is wrong.

## Case

A **Case** is the primary investigative and reporting unit presented to developers, agents, and CI. It contains the smell, its clues, receipts, diagnosis, leads, avoid guidance, severity, and verdict.

Conceptually, multiple observations may point toward a common architectural cause. For v1, however, case formation is deliberately deterministic: **one triggered smell produces one case**. Columbo does not automatically merge multiple smells into a larger case in v1. Future versions may correlate cases without changing the meaning of the underlying vocabulary.

## Diagnosis

A **Diagnosis** is Columbo's evidence-backed hypothesis about the underlying design problem suggested by a case.

Example:

```text
ProcessOrder appears to coordinate three independently
meaningful responsibilities:

  • payment authorization
  • order persistence
  • customer notification
```

Diagnoses are interpretations, not mechanically proven facts. They use qualified language such as "appears to", "suggests", or "may indicate".

Prefer:

```text
These responsibilities appear to change independently.
```

over:

```text
This type violates SRP.
```

Every diagnosis must be navigable backward to its receipts.

## Lead

A **Lead** is a promising direction for resolving a case.

Examples:

```text
Move payment behavior toward the object that owns payment state.
Consider introducing an abstraction around notification.
The repeated customer fields may represent a missing domain object.
Consider replacing variant conditional behavior with polymorphism.
```

Leads point toward architectural improvement without prescribing a blind mechanical transformation. They may reference established refactorings where appropriate.

## Verdict

A **Verdict** is the CI policy outcome for a case: `WARN` or `FAIL`. No case means pass.

The critical distinction is:

> **Architectural uncertainty does not imply enforcement uncertainty.**

Columbo may be cautious about a diagnosis while being completely certain that a configured deterministic threshold was exceeded.

## Agent-Oriented CI Output

Columbo messages are inputs to the next coding-agent iteration. An obscure metric error encourages the most naive workaround, so every failing case must include clues, why the smell matters, a diagnosis, leads, avoid guidance, and receipts.

Bad:

```text
funlen: ProcessOrder is 23 lines (max 10)
```

Preferred:

```text
CASE C-... — ProcessOrder

VERDICT
  FAIL

SMELL
  Long Function

CLUES
  ProcessOrder is 23 lines (limit: 10)
  Cognitive complexity is 9 (limit: 7)

WHY THIS MATTERS
  Long functions can indicate multiple responsibilities
  or a missing abstraction.

DIAGNOSIS
  ProcessOrder appears to coordinate payment,
  persistence, and notification behavior.

LEADS
  → Look for responsibilities with different reasons to change.
  → Prefer moving behavior toward the object that owns its data.
  → Extract cohesive concepts rather than arbitrary blocks.

AVOID
  ✗ Extracting helpers solely to satisfy the line limit.
  ✗ Creating stepOne/stepTwo/stepThree functions.
  ✗ Moving the same procedural sequence into another file.
  ✗ Suppressing the rule without documented justification.

RECEIPTS
  internal/orders/service.go:38-64
```

## Dogfooding policies

A **dogfooding policy** is an explicitly provisional product or enforcement decision being evaluated through real use. It is first-class specification metadata, distinct from objective clues, diagnoses, source suppressions, and configuration. Multiple smells may use this mechanism. A provisional policy MUST have a fully defined deterministic enforcement rule; uncertainty about its desirability MUST NOT leave implementation behavior unresolved.

### Decision register

This specification is the authoritative register. Each entry MUST contain:

- a unique, stable policy ID;
- status: `provisional`, `retained`, `revised`, or `removed`;
- the decision and rationale;
- an exact applicability rule identifying which cases carry its review note;
- the literal review note and human-review prompt;
- the evidence required to evaluate the decision;
- for a resolved entry, the resolution rationale and named regression fixtures covering the resulting behavior.

Policy IDs MUST NOT be reused. Resolved entries remain in the register for future agents and release audits. Policy metadata is not user-configurable in v1 and introduces no new CLI flag.

### Case output and agent handoff

Every emitted case matching a provisional entry MUST include that entry's review note and prompt, including WARN cases and cases displayed as suppressed. Disabled smells produce no cases or policy-review notes. If several entries apply, order them by policy ID.

Text output appends an optional `DOGFOODING POLICY REVIEW` section after `RECEIPTS`, with each entry's ID, note, and prompt. Every JSON case includes `policy_reviews`, an array of objects with exactly the string fields `id`, `note`, and `review_prompt`; use an empty array when none apply. The strings are the literal registered templates. Resolved entries are not included in case output.

Policy notes MUST NOT change smell triggers, thresholds, severity, verdicts, suppressions, case IDs, summary counts, or exit codes. They are not clues and MUST NOT suggest that the tool has established the author's intent. Columbo reports the handoff instruction; it does not itself contact a human, invoke an agent, modify code, or create a suppression.

### Public-release gate

Provisional entries are permitted during dogfooding. Before public v1 release, every entry MUST be explicitly resolved as retained, revised, or removed, with rationale and regression fixtures. Public-release validation MUST reject a nonempty set of provisional entries. Ordinary dogfooding builds MUST NOT be blocked merely because an entry is provisional. Fixtures MUST verify deterministic text/JSON review notes and that adding or removing review metadata does not change enforcement results or case identity. Public-release output MUST contain no provisional review notes.

### CE-001: strict Cosmetic Extraction during dogfooding

- **Status:** `provisional`.
- **Decision:** Trigger Cosmetic Extraction whenever its four defined structural conditions hold, even when the helpers might represent meaningful responsibilities. Do not require proof of cosmetic intent or architectural invalidity.
- **Rationale:** Strict CI feedback is intentional. Dogfooding will establish whether the structural rule rejects too many worthwhile decompositions.
- **Applicability:** Every emitted `cosmetic-extraction` case.
- **Review note:** "This rule deliberately rejects some decompositions that may represent meaningful responsibilities. This is a provisional dogfooding policy; its review note does not change the case verdict."
- **Human-review prompt:** "Show the human the parent and qualifying helpers, forwarded inputs, shared dependencies, overlap values and thresholds, and original versus expanded line and complexity metrics. Discuss whether the failure reflects the intended policy before changing code or requesting a suppression."
- **Required evidence:** Receipts identify the parent, helper declarations, and qualifying call sites. Clues report the forwarded-input sets, dependency sets, calculated overlaps and configured thresholds, and original and expanded metrics. Multiple qualifying clusters are individually identifiable in the evidence.
- **Evaluation:** Review examples where humans judge the flagged decomposition meaningful, alongside examples of arbitrary helper extraction. Resolve whether to retain, revise, or remove the structural policy.
- **Required regression coverage:** Include an arbitrary helper extraction that fails and a plausibly meaningful decomposition that nevertheless fails because all four conditions hold. The latter fixture locks the deliberate strictness, not a claim that the design is objectively wrong.

## CLI and exit codes

```bash
columbo [flags] [packages...]
```

No packages means `./...`. Flags must precede package arguments; `--` ends flag parsing. Unknown flags, invalid values, and unmatched package patterns exit 2. Help exits 0 and prints usage without analysis. Required flags are `--config PATH` (default `.columbo.yml`), `--format text|json` (default `text`), `--no-history`, and `--version`. Repeated flags use their last value. Boolean flags accept true/false. A valid `--version` invocation prints `columbo <build-version>` and exits 0 without loading configuration or packages; invalid flag syntax still exits 2. An unset build version is `dev`.

Resolve the current directory's enclosing go.mod as the invocation module; absence is fatal. The module root is the path root for exclusions, identities, and receipts. Configuration paths are relative to the invocation directory. Each invocation supports exactly one module: every selected source package, including external test packages, must belong to it. Workspace selection spanning modules, selected dependencies outside it, and GOPATH-only inputs exit 2. Dependency packages may be loaded for type information but do not produce cases.

Use the active Go build environment (including build tags supplied through GOFLAGS), with tests enabled. Analyze selected packages' normal source, in-package tests, and external test packages. Deduplicate the normal/test variants of a declaration by physical file and declaration position; do not analyze synthetic test-main packages. Original build-selected cgo source is the finding subject; compiler-generated wrappers provide semantic support only. Generated declarations do not become finding subjects, Data Clump occurrences, helper references/calls, or additional known interfaces. Use type information corresponding to original physical source; if the loader cannot provide the physical-source/type correspondence required for analysis, exit 2 with a diagnostic identifying that failure. Generated positions never substitute for physical receipts. Exclusions apply to original physical source. Receipts refer to physical source positions, ignoring `//line` remapping. Source outside the module or unavailable source needed for selected-package analysis is fatal.

Selected packages' original build-selected physical source files form the reference universe; compiler-generated cgo wrappers are not additional source files in that universe. Include excluded and generated source files when counting helper references/calls and discovering interface types, but not as finding subjects or Data Clump occurrences. Interface knowledge includes their named and anonymous interface types and the interfaces in transitively loaded dependency type information; do not search the repository or unrelated modules. All selected-package files must load, parse, and type-check, including excluded/generated files. Analyze no unselected package bodies. Deduplicate call sites and directives across test variants.

Exit codes: 0 means completed with no unsuppressed FAIL cases; 1 means completed with at least one unsuppressed FAIL; 2 means invalid invocation/configuration, package loading/parsing/type-checking failure, or internal error. Complete analysis and serialization before writing stdout. Any fatal error before output delivery emits no stdout analysis document or summary, emits diagnostics on stderr, and exits 2 regardless of discoveries. An output write failure is fatal: emit a diagnostic on stderr and exit 2. If the sink has already accepted bytes, partial stdout is permitted and is not a completed analysis document; stdout delivery is not transactional.

Named function/method declarations, including blank-named func _ declarations, are finding subjects. Blank-named declarations participate in every applicable rule but cannot be helper candidates because they cannot have a resolved caller. Bodyless declarations participate only in parameter-list and Data Clump rules. Function literals, interface methods, and function-type declarations are not independent subjects. Enclosing-body metrics include nested literals; parameters of nested literals do not contribute to the enclosing parameter list. Feature Envy and helper-cluster analysis do not cross function-literal boundaries.

## Configuration

The following complete defaults define the schema:

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

An absent implicit default file uses these defaults; an absent explicit --config file exits 2. Empty file/mapping uses defaults. Override supplied scalar fields individually; omitted sections/keys retain defaults. A supplied exclude sequence replaces the default sequence; [] clears it. Generated-file exclusion cannot be disabled. --no-history=true overrides history.enabled.

Accept exactly one YAML document with a mapping root. Reject unknown keys, duplicate keys at any level, explicit nulls, YAML merge keys, aliases, and values of the wrong type. version is integer 1; counts and max-commits are positive integers, cosmetic-min-helpers is >=2, and booleans are YAML booleans. Integer fields require YAML integer scalars fitting signed 64 bits; floats such as 3.0 are not integers. Ratio is positive and finite; overlaps are finite and in [0,1], including zero. Severity is exactly off/warn/fail. Exclusions are a sequence of strings. Invalid glob syntax is fatal even if no file matches. Off emits no case but does not disable metric computation required by another rule. Thresholds, not the severity of Long Function or High Cognitive Complexity, determine Cosmetic Extraction condition 4.

Use doublestar v4 Match semantics on module-relative slash paths, including root files. Pin the chosen v4 release in go.mod. Files with the standard Go generated marker (a line matching `^// Code generated .* DO NOT EDIT\.$` before the first non-comment/non-blank source text) are always excluded. Excluded files produce no cases, clump support, or suppression validation/warnings; their semantic role is defined under CLI and exit codes.

## Metric definitions

**Function lines:** Scan Go tokens with physical positions. Count the distinct physical lines occupied by non-comment tokens inside the outer body's braces, excluding brace and semicolon tokens. Tokens belonging to the signature and the outer braces do not count. A mixed signature/body line counts if it contains a counted body token. Count every physical line spanned by a multiline token, including blank lines inside a raw string literal. Blank/comment-only and brace-only lines count zero; multiline expressions and literals count their token-bearing lines. An empty body is zero. Local declaration syntax, control-flow headers, labels, and nested function-literal bodies count. A physical line counts once.

**Parameters:** Expand grouped names; an unnamed field counts once. Variadic parameters count once; receivers, type parameters, and results do not count.

**Cognitive complexity:** Pin the function-level visitor semantics of [gocognit v1.2.0](https://github.com/uudashr/gocognit/blob/v1.2.0/gocognit.go), including its direct-recursion behavior and nesting within literals. Do not honor gocognit suppression comments or add an interprocedural recursion-cycle algorithm. Fixture outputs lock semantics independently of dependency upgrades.

**Type identity:** Resolve aliases before comparison; preserve distinct named types even if underlying types match. For canonical strings recursively resolve aliases throughout composite components and generic arguments without unfolding named underlying types, then use go/types.TypeString on the resulting type with full package import paths as qualifiers, with Go's canonical predeclared aliases (byte is uint8; rune is int32; any is interface{}). Preserve pointer/composite structure and generic type arguments. Signature type parameters are alpha-renamed by declaration order: receiver parameters R0, R1, ... and function parameters T0, T1, ...; their constraints are not part of a clump key. Before canonical serialization, erase parameter and result variable names in structural function signatures, including unnamed interface method signatures; preserve interface method names, parameter order, variadic distinctions, and types. Do not unfold named types to erase names in their underlying types. A variadic parameter normalizes as its slice type. The fixed Go toolchain is part of reproducibility.

**Dependencies:** A dependency is a canonical type or package identity, never a collaborator instance. Build a complete evidence inventory per declaration, then derive the scored collaborator set:
- Add `package:<import-path>` for every source reference to a package-scoped object from another package, including dot imports, type references, constants, variables, functions, and method/field selections whose selected object belongs to another package.
- Add `type:<canonical-type>` for every named concrete or named interface type encountered in parameter/result types, explicit body type syntax, construction, conversions, assertions, call callee/signature and argument/result types, or member receiver/selected-member types. For an unnamed interface encountered there, add `interface:<canonical-type>`. Alias resolution precedes this classification: every named interface, including a generic instantiation or an alias resolving to one, uses type:, and only genuinely unnamed interfaces use interface:.
- To collect types, strip pointers and recurse through arrays, slices, maps, channels, anonymous structs, and function signatures. At a named type, record its identity (including type arguments), recurse into its type arguments, and stop: do not traverse its underlying fields/methods. Type parameters contribute no dependency and their constraints are not expanded. Primitive builtin types and builtin functions contribute no type identity. The predeclared named interface error contributes type:error to the inventory; an unnamed empty interface contributes interface:interface{} consistently whether spelled any or interface{}. Both are universal plumbing and do not contribute to the scored collaborator set. Named application interfaces and nonempty anonymous interfaces remain scored.
- Exclude the method's receiver named type (all its instantiations). Exclude an unexported type declared in the same file whose object has no references outside that file in the reference universe; this is the exact meaning of implementation-only. Declaration alone is not a reference. Exported or cross-file-used types remain counted.
- The inventory records both package:pkg and type:pkg.Client. The scored set excludes package:pkg when at least one included named type is owned by that package, using the resolved type object’s package identity (including aliases and generic instantiations), rather than parsing canonical strings. Package-only operations remain scored. Types inherited from called signatures remain included, except for the universal plumbing above. Deduplicate each identity across all sites. Two values of one interface type contribute one identity. Inferred local types contribute only when used in the listed operations; an unused local declaration contributes its explicit type syntax, if any. Include nested literal bodies, but never follow a callee's body for ordinary dependency metrics.

Dependency source sites are defined as follows: foreign package object references use the object's identifier range (for a member selection, the selected identifier); explicit type syntax uses the complete type-expression range; callee-signature and call-result types use the complete CallExpr range; argument types use each argument-expression range; construction, conversion, assertion, and member receiver/selected-member types use the complete corresponding expression range. Traverse each of these operations, including nested operations, once. At each designated range emit one receipt per inventoried dependency identity, regardless of how many roles or recursive type-collection paths yield that identity there. Distinct ranges, including overlapping ranges, remain separate sites. Receipts retain every site contributing an included identity; exclusions of types do not exclude independently referenced external package identities. Receipts for scored identities use kind dependency; retained receipts for unscored identities use kind dependency-inventory. Excessive Dependencies cases expose the scored dependency-set as a clue, so the failing count can be reconciled independently of the larger inventory. Helper overlap uses exactly the scored dependency sets; Cosmetic Extraction retains both kinds of receipts.

## Smell rules

### Long Function

Trigger when counted source lines > `function-lines` (default 10). Leads must emphasize cohesive responsibilities and ownership. Avoid guidance must explicitly reject arbitrary helper extraction solely to reduce lines.

### Long Parameter List

Trigger when parameters > `parameters` (default 4). Lead toward missing domain/parameter objects or responsibility boundaries.

### High Cognitive Complexity

Trigger when complexity > `cognitive-complexity` (default 7). Lead toward simplifying decision structure, clarifying early returns, or meaningful variant abstractions.

### Excessive Dependencies

Trigger when dependency count > `dependencies` (default 5). Do not recommend a facade whose only purpose is hiding the count.

### Feature Envy

Count each field/method selector once; a method invocation contributes its selector, not an additional call increment. Method values count one selector; method expressions on a type do not access a value and count zero. Promoted selections count once, not once per implicit embedding step. Package-qualified identifiers are not receiver accesses.

A receiver value is identified lexically by a variable object and an optional field path. Its output key is "<qualified-symbol>:<variable-name>@<physical-declaration-byte-offset>" followed by each selected field name as ".<field>"; distinguish shadowed variables by declaration offset. Strip parentheses, address-taking, and dereference syntax. Do not propagate aliases or assignments: x and y remain distinct even after y := x; reassignment does not split x. For a selector x.Child.Name, attribute Child to x, and Name to x.Child. Repeated identical field paths share a group. Values rooted in indexing, calls, conversions, assertions, or other expressions have no stable value key and do not contribute. Only groups whose immediate static type, after pointer stripping and alias resolution, is named qualify; named interfaces qualify, unnamed interfaces/composites do not.

Own accesses are selections directly attributed to the declared receiver variable. Its field-path values are foreign groups if their immediate type qualifies. Skip nested function-literal bodies. A foreign group triggers at >= feature-envy-foreign-accesses AND >= feature-envy-ratio times own accesses. Zero own accesses satisfies the ratio once the foreign minimum is met. Produce one method case containing every qualifying group, sorted by first access position then canonical expression string; do not choose only the largest group. Provide every contributing selector range and separate counts for own and each qualifying foreign group.

### Data Clump

Within each source package import path, consider named function/method declarations' parameter multisets. Names and receivers do not contribute. Each declaration location is one distinct occurrence even if signatures are identical; deduplicate test variants, exclude excluded/generated declarations, and include bodyless declarations. External test packages are separate packages.

Use the canonical type normalization in Metric definitions. For multiset M, support S(M) consists of declarations containing at least M's multiplicity of every type. It qualifies when |M| >= data-clump-size and |S(M)| >= data-clump-occurrences. Report M only if no strict multiset superset N qualifies with S(N) = S(M). Thus {A,B,C} in four declarations and {A,B,C,D} in three produces both cases when the minimum support is three. This is closed frequent multiset reporting, not suppression of every smaller frequent set.

A case is anchored to its earliest supporting declaration by module-relative file path, physical line, then column. Use that declaration's qualified symbol and full range. Secondary key is a compact JSON array of sorted canonical type strings, retaining duplicates. Receipts include every supporting declaration and its matching parameter-field ranges; select the first matching parameters in declaration order when a signature contains excess multiplicity. Types, occurrences, and evidence are deterministically ordered.

### Cosmetic Extraction

Detect private helper decomposition by its structural conditions, not inferred author intent. CE-001 applies during dogfooding.

A candidate is a named unexported function or method with a body in an included selected-package file. It must have exactly one statically resolved call site in the reference universe and no first-class references. Resolve direct calls to function objects, concrete method selections/expressions, and generic instantiations to their origin declarations. Calls through variables and interface dispatch are not static calls. A reference to the function/method object outside the callee expression of such a direct call is a first-class reference and disqualifies it. Count go/defer calls and calls within literals as call sites, but those sites are not cluster calls. Excluded-file callers therefore can disqualify an otherwise single-use helper.

For methods, use the receiver instantiation at the candidate's unique statically resolved call site, including a method-expression call. Apply its type arguments to the receiver and method signature; type arguments that remain caller type parameters retain their actual constraints. Exclude the candidate if that receiver type or its pointer implements any interface type in the known interface universe and that interface method set contains the method's name/signature after this substitution. Use interfaces as represented in that universe, including observed instantiations; do not enumerate hypothetical receiver or interface instantiations. Merely sharing a method name is insufficient. Same-package and known-interface scope are defined under CLI. For each analyzed parent, discover clusters in its own body and in every declaration reachable through candidate edges, visiting each declaration once and stopping at already visited declarations. Every member helper must be reachable from that parent through candidate edges. Cluster ownership is the declaration physically containing its statement list. Forwarding uses that owner's inputs, without ancestor substitution. A parent may itself be a candidate and receive its own case; the same physical cluster may consequently appear in several parents' cases, each evaluated against that parent's own expanded metrics.

For each direct cluster call, P is the set of the immediate caller's parameter-variable objects plus its receiver variable, omitting blank identifiers. H is the subset of P used unchanged as call arguments or as the method receiver. Ignore parentheses; do not treat dereference, address-taking, conversions, fields, arithmetic, or other derived expressions as unchanged forwarding. Repeated arguments count once. A method-expression receiver argument is counted like any other argument. Overlap is |H|/|P|, or 0 if P is empty. No interprocedural substitution is used for this ratio. Record P and H explicitly using the same variable keys as Feature Envy, with no field suffix.

Form clusters separately in each lexical statement list (function body, block, case, or communication clause). Scan each immediate statement's calls in source-position order, skipping nested statement lists, which are analyzed separately, and function-literal bodies, which are never analyzed for clusters. Do not discover clusters in child statement lists inside a literal. Literal calls still participate in helper eligibility and ordinary/expanded metrics as specified. A control statement (if/for/range/switch/type-switch/select), explicit nested block, labeled statement, go/defer, return, branch, or empty statement ends the current sequence; analyze its child statement lists separately. Consequently opposite branches and loop bodies never join a surrounding sequence. Return/go/defer calls cannot be cluster members. Assignments, local declarations, sends, increments, and expression statements do not by themselves break a sequence.

Every synchronous direct candidate call encountered in the remaining statements is a member. Every other call breaks the sequence, except calls to Go builtins, the package functions in log (Print/Printf/Println/Fatal/Fatalf/Fatalln/Panic/Panicf/Panicln), log/slog (Debug/DebugContext/Info/InfoContext/Warn/WarnContext/Error/ErrorContext/Log/LogAttrs), and fmt.Errorf. Recognize these by resolved package path/object, not spelling; errors.New, logger methods, custom loggers, and third-party wrappers are not exceptions. Type conversions are not calls for sequence formation. Exception calls do not contribute helper count or overlap. Nested calls are ordered by their call-expression starting position; this is lexical analysis, not runtime evaluation order.

A cluster is a maximal uninterrupted sequence of candidate calls in one statement list. Do not search subsequences or arbitrary subsets. Require at least cosmetic-min-helpers distinct helpers. Compute the arithmetic mean of members' forwarding overlaps and of all unordered distinct helper pairs' dependency Jaccard overlaps; two empty dependency sets have overlap 0. Use unrounded values for comparison; text displays six fractional digits and JSON emits numerically rounded values without requiring trailing zeroes.

Trigger when at least one cluster meets both overlap thresholds and expansion of the parent exceeds function-lines or cognitive-complexity. Threshold violation is independent of those smells' severity. Expand all reachable candidate call sites in the parent, not only cluster members. Produce one case per parent with every qualifying cluster and its member order, P/H sets, dependency sets, pairwise/mean overlaps, and original/expanded metrics. Virtual metrics do not independently trigger other smells. Weak names are not an additional clue in v1; they never influence enforcement.

## Virtual inlining

Virtual inlining is a structural analysis model, not a semantics-preserving source transformation. Do not compile synthetic expanded code, rewrite returns, substitute parameter expressions, or model execution counts. This definition replaces the conceptual phrase "equivalent expanded control flow."

Expand every statically resolved candidate call, including expression calls and go/defer sites, recursively. Maintain a declaration stack beginning with the parent. At a non-candidate or a callee already on the stack, retain the call unexpanded. Each syntactic site contributes once regardless of loops or dynamic execution. Calls inside nested literals contribute at that literal's existing metric depth.

**Expanded lines:** Represent a body's counted tokens as line buckets keyed by physical source line. Process original child argument/callee expressions first. For each expandable call, remove its own callee/argument syntax tokens from its original buckets, retaining any tokens belonging to nested calls handled separately. Insert a fresh copy of the recursively expanded callee body's line buckets. Each expansion copy has a distinct identity so lines from different helpers or calls cannot coalesce. Retain original caller buckets that still have counted tokens; discard empty buckets. An empty helper contributes zero buckets. Non-call surrounding syntax such as assignment targets/operators, return, or a second nonexpanded call retains its original bucket. At a cutoff keep all remaining call tokens. Count the final buckets. No signature, parameter binding, or synthetic result assignment adds lines.

Examples: a standalone helper() with a three-line expanded body contributes 3 lines; x := helper() contributes 1 surrounding line plus 3; a(); b() with expanded bodies of 2 and 3 lines contributes 5 if no surrounding counted token remains. Inline helper argument expressions are counted only as original nested call contributions or retained call syntax at a cutoff; the callee parameter uses are already in its own body. Source receipts identify the original call and copied-body contributors, not invented physical line numbers.

**Expanded complexity:** Use the pinned visitor with one interception rule. At an expandable CallExpr, first visit its callee and arguments using the pinned traversal at the current nesting depth, then visit the callee body's statements at that same depth with a fresh logical-operator/else-node bookkeeping context and that callee's declaration name as the direct-recursion context. Do not add a function-literal nesting level or a call increment. Restore the caller context after the visit. Enclosing control statements, expression traversal, and literal nesting follow the pinned visitor unchanged. Each expanded helper's returns/results/go/defer receive only their ordinary visitor treatment; they never terminate the caller's traversal. At a cutoff use the unmodified visitor's treatment of the retained call under the current declaration's recursion context. No mutual-recursion increment is added.

For example, an if in a helper called at nesting depth 2 contributes 3. Calling that helper from an if condition uses the visitor's condition depth; calling it from the if body uses the body's incremented depth. Retain the full expansion trace and complexity contributions as receipts, using the copy identities and increment aggregation defined under Output.
 
## Git history

History is optional provenance evidence, never evidence of architectural intent and never a verdict input. For the invocation module's enclosing worktree, enumerate commits reachable from HEAD and order by descending committer timestamp, then ascending full commit hash. Metadata enumeration may cover all reachable commits; inspect changed-path/content evidence for only the first max-commits entries. Shallow history is the locally reachable set; do not fetch.

For each inspected commit, list changed paths against its first parent (or the empty tree for a root commit), with rename detection disabled. Associate a commit with a case if a changed path exactly matches any of the case's source-receipt files, translated from module-relative to worktree-relative paths. This is file provenance, not symbol tracking. Record full commit hash, integer Unix committer timestamp, and matching module-relative files sorted lexically. Include every matching inspected commit, in the specified commit order. No prior metric calculation, extraction attribution, rename tracking, author data, or commit message is required. Dirty/untracked source is analyzed as present; provenance still refers to HEAD history and does not claim to explain those changes.

When disabled, do not access Git and emit no history warning or receipts. If Git is unavailable, no enclosing worktree/HEAD exists, or any required history operation fails, discard all collected history receipts and continue static analysis with exactly one history-unavailable warning. It appears on stderr in either format and also in JSON warnings. No history error exits 2. Record the generic deterministic message "Git history unavailable; static analysis completed without history evidence." Do not include volatile OS error text in the analysis document.

## Case identity and ordering

Canonical identity is UTF-8:
`<smell-id>|<module-relative-file>|<qualified-symbol>|<secondary-key>`.

Smell IDs are the seven severity keys. File paths are cleaned module-relative slash paths. Qualified symbols are full package import path plus "." plus function name; methods use "<package>.(<receiver>).<method>" with receiver as local declared type name, prefixed "*" for pointer receiver, omitting receiver type arguments. Multiple init declarations use init#N, with N starting at 1 in declaration order within the anchor file. Blank-named functions use _#N, with a separate N starting at 1 in physical declaration order within each file. Data Clump uses its anchor declaration's symbol.

Data Clump's secondary key is the compact JSON array specified in its rule; all others use the empty string. No source line, thresholds, severity, history, suppression, or review metadata enters identity. Displayed ID is C- plus the first 10 lowercase SHA-256 hexadecimal characters. Before joining identity fields with "|", percent-escape literal %, |, CR and LF in each field as %25, %7C, %0D and %0A respectively. A collision between unequal canonical identities exits 2 rather than silently merging cases.

Use the declaration's physical start line and final token's physical line for the primary range (full declaration including signature, excluding preceding comments). Sort cases, including suppressed cases, by file path, start line, smell ID, then case ID. Source evidence sorts by file, start line, end line, start_offset, end_offset, kind, compact JSON encoding of detail. Aggregate complexity events as defined under Output before sorting and removing exact duplicates; expansion_sites participates in this encoding so distinct expansion copies cannot be deduplicated together. When internal disambiguation needs a declaration position use physical byte offset, not line remapping.

## Suppressions

Only source suppressions exist:
```go
// columbo:ignore <smell-id> -- <justification>
```

Recognize line comments whose text after "//" and optional horizontal whitespace begins "columbo:ignore". Parse exactly one known smell ID, whitespace, "--", whitespace, and a nonempty justification containing at least 10 non-whitespace Unicode characters. Trim outer whitespace in stored justification. Any other recognized directive syntax is fatal, including Data Clump directives. Block-comment text is not directive syntax.

A directive must be in the ast.FuncDecl.Doc comment group of an included named function/method declaration, including bodyless declarations. Other comments in the same group may intervene; a blank line breaking the doc group prevents attachment and makes the recognized directive fatal. Directives attached to types, variables, grouped declarations, interface members, or no declaration are fatal. A declaration may have directives for different smells; duplicate declaration/smell directives are fatal. Validate included-file directives before emitting any analysis output, even if the smell is off. Ignore excluded-file directives entirely.

A valid directive applies to the case with that primary declaration and smell. Keep that case and its original WARN/FAIL verdict, set suppressed=true, and exclude it from failed/warned totals and CI failure. Count it once in suppressed. Keep its evidence, history and applicable dogfooding notes. A valid directive with no matching case (including disabled smells) is unused and produces one warning; it does not fail. A directive cannot suppress a case merely because its declaration appears as a secondary receipt. Directive syntax uses spaces/tabs as separators; a directive may not span comments or lines. Suppression warnings and validation are unaffected by whether history is enabled.

## Output

Text renders every case, including suppressed cases, with sections in this order:
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
An unsuppressed verdict is WARN or FAIL; a suppressed verdict is "SUPPRESSED (original: <verdict>)" followed by the directive location and justification. Append DOGFOODING POLICY REVIEW when applicable. Within CLUES, show every qualifying cluster's key, owner, and members in their recorded order, including call offsets. Show all clue fields and source/history receipts; do not truncate evidence. Finish with "Columbo: <failed> failed, <warned> warned, <suppressed> suppressed", or "Columbo: no cases" when cases is empty. Counts include only unsuppressed FAIL/WARN and suppressed cases respectively. ANSI is optional only when stdout is a TTY; never emit ANSI otherwise. Plain-text byte formatting and templates must have checked-in golden coverage.

JSON stdout is one UTF-8 document followed by a newline. The top-level fields are exactly version (integer 1), summary, cases, suppressions, warnings. Summary has exactly failed, warned, suppressed (nonnegative integers). Empty arrays are [], never null.

Every case has exactly:
- id, smell, verdict, symbol, file (strings; verdict is WARN/FAIL even when suppressed);
- start_line, end_line (positive inclusive integers);
- suppressed (boolean);
- clues (array of clue records);
- clusters (array of cluster records, empty for all smells except Cosmetic Extraction);
- why, diagnosis (strings);
- leads, avoid (arrays of strings);
- receipts (array of source or history records);
- policy_reviews (the array defined under Dogfooding policies).

A cluster record has exactly key, owner, file, members. key, owner, and file are strings: key is the cluster key defined below, owner is the containing declaration's qualified symbol, and file is its module-relative path. members is an array in lexical call-expression starting-offset order, each with exactly helper (qualified symbol string) and call_offset (zero-based physical byte offset integer). Sort clusters lexically by file, then numerically by their first member's call_offset. Include every qualifying cluster exactly once per parent case. This representation preserves member order independently of clue and receipt sorting.

A clue has exactly kind, subject, value, limit, operator. kind/subject are strings. value is a finite number or sorted array of strings. limit is a finite number or null; operator is ">", ">=", or null and is null exactly when limit is null. Metric kinds are function-lines, parameters, cognitive-complexity, dependencies, own-accesses, foreign-accesses, expanded-lines, expanded-complexity, helper-count, parameter-overlap, dependency-overlap. Set kinds are dependency-set, parent-input-set, forwarded-input-set, clump-types. Clump support uses clump-occurrences. Parameter/clump sizes use parameters and clump-size respectively. subject is the qualified symbol, foreign variable/field path, cluster key, or cluster-qualified member/pair/mean subject defined below. Cluster key is "<file>:<first-call-byte-offset>"; pair subjects are the cluster key followed by ":" and the compact JSON array of the two sorted qualified helper symbols. Set clues have null limit/operator. For Feature Envy include the own count, each foreign count/minimum and its ratio as a parameter-free numeric clue kind foreign-own-ratio (value foreign/own; when own=0, use value 0, limit null, operator null, and the diagnosis explains the zero-own rule). For every violated threshold retain the value and comparison; complementary nonthreshold evidence uses null comparison. Each ordinary metric case includes its triggering metric. Feature Envy includes own-accesses, every qualifying foreign-accesses and foreign-own-ratio. Data Clump includes clump-types, clump-size and clump-occurrences. Cosmetic Extraction includes original and expanded line/complexity metrics, helper-count per qualifying cluster, every member forwarding overlap and P/H sets, every helper dependency set, every unordered pair overlap and both cluster means. For member parameter-overlap and forwarded-input-set clues, use the cluster key followed by ":member:" and the helper's qualified symbol as subject. Each cluster's parent-input-set uses the cluster key as subject. Helper dependency-set clues use the qualified helper symbol and are emitted once per helper per case. Append ":mean" to cluster keys for mean subjects. Individual member parameter-overlap and unordered pair dependency-overlap clues have null limit/operator; only their respective cluster means use the configured overlap limit and >=. Helper-count uses cosmetic-min-helpers and >=. Whenever own accesses are nonzero, the foreign-own-ratio clue uses the configured ratio limit and >= operator. Sort clues lexically by kind, subject, then compact JSON encodings of value, limit, and operator. JSON uses UTF-8 without HTML escaping, no insignificant whitespace in canonical encodings, and shortest round-trippable decimal numeric encoding; object keys in canonical encodings sort lexically. Round reported overlap/ratio values to six decimal places; compute Jaccard/forwarding ratios and their means as exact rationals and compare against the exact rational value represented by the parsed finite IEEE-754 binary64 configuration threshold, never rounded display values. Feature Envy compares foreign count against that ratio threshold times own count using the same rational comparison. Report numeric rounding uses round-to-nearest, ties-to-even.

A source receipt has exactly kind, file, start_line, end_line, start_offset, end_offset, detail. Offsets are zero-based physical UTF-8 byte offsets, start inclusive and end exclusive. Ordinary ranges use the corresponding syntax range. A line-bucket receipt uses the full physical line range, including its line terminator if present; its start_line=end_line remains the counted bucket's line. Complexity receipts use the token range at the pinned visitor's increment position, not the contributing node's complete range. kind is declaration, parameter, dependency, own-access, foreign-access, helper-call, or metric-contribution. detail is an object with exactly subject (string), value (finite number or null), nesting (nonnegative integer or null), expansion (array of qualified symbols), and expansion_sites (array of call-site records). subject is the qualified declaration, canonical dependency identity, receiver-value key, parameter type prefixed by its zero-based expanded parameter index, or metric kind. value is the line-bucket contribution (1), complexity increment, or expanded parameter count where relevant, otherwise null. nesting is set only for complexity increments and is the pinned visitor's diagnostic nesting value, including zero for its non-nesting increments; it is not an independently reconstructed control-flow depth. Expanded increment amounts still use the visitor's actual current depth under the interception rule. expansion is the root-to-helper declaration chain for copied-body evidence, otherwise []. expansion_sites is the ordered root-to-copy call-site path, with each record containing exactly file (module-relative string) and call_offset (zero-based physical byte offset integer). It is [] for original evidence and has one entry per helper expansion, one fewer than the nonempty expansion declaration chain. Append a site's identity on every expansion, including calls processed in callee/argument child expressions; preserve all entries on recursively copied evidence. This path distinguishes separate copies even when their declaration chains and source ranges coincide. For a helper-call receipt, subject is the callee qualified symbol. No source excerpt is required. A history receipt has exactly kind="history", commit (full hash), committed_at (integer Unix timestamp), files (sorted strings). Source receipts precede history receipts; source order follows Case identity and ordering, history order follows Git history.

Required source evidence:
- all cases: primary declaration receipt;
- Long Function: each counted physical line;
- Long Parameter List: every parameter field, with expanded count;
- High Cognitive Complexity: every pinned-visitor increment accounted for, with amount/nesting;
- Excessive Dependencies: every contributing site labeled with dependency identity;
- Feature Envy: all own selectors and each qualifying foreign group's selectors;
- Data Clump: every supporting declaration and matched parameter field;
- Cosmetic Extraction: cluster calls/helper declarations, forwarding sites, all helper dependency sites, original/expanded contributing line buckets and complexity increments, with expansion call-chain annotations.
Forwarding sites use parameter receipts at the forwarded identifier range; their detail.subject is the forwarded variable key and value/nesting are null. Parameter-field detail.subject is "<zero-based-parameter-index>:<canonical-type>"; grouped fields have one receipt per expanded parameter index. Before duplicate removal, aggregate complexity events with identical kind, file, line/offset range, subject, diagnostic nesting, expansion, and expansion_sites into one receipt whose value is the sum of their increments. This receipt may represent several visitor events. Aggregation never crosses copy paths or different nesting values. The sums of function-lines, expanded-lines, cognitive-complexity, and expanded-complexity contribution receipts must equal their respective reported metrics; emit original and expanded metric kinds separately even if their values coincide. Line receipts use start_line=end_line for the counted physical bucket; copied buckets preserve source file/line and both expansion annotations. Do not fabricate source locations for synthetic expansion. Additional source receipts beyond these contributing sites are not emitted.

Each suppression record has exactly smell, symbol, file, line, justification, applied, case_id. applied is boolean; case_id is the matching ID or null when unused. List all valid directives, including unused, ordered by file, line, smell. Warnings have exactly code, file, line, message: code is unused-suppression or history-unavailable; history uses file="" and line=0. An unused warning references its directive and has message "Unused suppression for <smell> on <symbol>." Sort warnings by code, file, line, message. JSON warnings belong in warnings; unused-suppression warnings also appear on stderr only in text mode. History warning additionally appears on stderr in JSON mode as explicitly required. No prose is emitted on JSON stdout.

### Deterministic guidance templates

Use these literal templates; replace {symbol} with the primary qualified symbol. Leads/avoid cells separated by ";" are separate array items. Why/diagnosis are single strings. Numeric/set details belong in clues and receipts. Do not infer domain responsibilities from names or history. Examples earlier in this document illustrate the vocabulary, not additional analysis or required domain-specific diagnoses.

| Smell | Why | Diagnosis | Leads | Avoid |
|---|---|---|---|---|
| long-function | Long functions can obscure responsibility boundaries and ownership. | {symbol} exceeds the configured source-line limit; this may indicate responsibilities that deserve separate boundaries. | Look for cohesive responsibilities with different reasons to change.; Move behavior toward the object that owns its data. | Extracting arbitrary helpers solely to reduce the line count.; Moving the same procedure into another file. |
| long-parameter-list | Many parameters can obscure which inputs belong together. | {symbol} exceeds the configured parameter limit; its inputs may represent a missing domain concept or mixed responsibilities. | Examine whether inputs form a cohesive domain object.; Separate responsibilities that require different inputs. | Bundling unrelated inputs into an opaque bag solely to reduce the count. |
| high-cognitive-complexity | Deep or repeated decision structures make behavior harder to follow. | {symbol} exceeds the configured cognitive-complexity limit; its decision structure may need simplification. | Clarify early exits and simplify nested decisions.; Examine whether variants deserve meaningful abstractions. | Moving the same decision tree into trivial helpers solely to hide its score. |
| excessive-dependencies | Broad dependency sets can indicate excessive coordination or coupling. | {symbol} exceeds the configured dependency limit; it may coordinate responsibilities with separate ownership. | Inspect the contributing dependencies for cohesive ownership boundaries.; Move behavior toward the data and collaborators it primarily uses. | Introducing a facade whose only purpose is hiding the dependency count. |
| feature-envy | Concentrated access to another value can indicate misplaced behavior. | {symbol} meets the configured foreign-access and ratio conditions; behavior may belong nearer a qualifying foreign value. Zero own accesses satisfies the ratio condition once the foreign minimum is met. | Review each qualifying foreign value and its contributing accesses.; Consider moving cohesive behavior toward the owner of that data. | Renaming or aliasing a collaborator solely to split the access count. |
| data-clump | Repeated groups of parameter types can indicate a missing shared concept. | The parameter multiset anchored at {symbol} recurs at the configured support and size; it may represent a missing domain object. | Inspect the participating signatures for a shared domain concept.; Introduce a parameter object only if the values belong together. | Bundling unrelated values solely to silence the repeated-type rule. |
| cosmetic-extraction | Single-use helper sequences can retain the original procedure's coupling while hiding its metrics. | {symbol} has helper clusters meeting the configured overlap conditions, and candidate expansion exceeds a configured metric limit; this may be cosmetic decomposition even if the helper names sound meaningful. | Review the qualifying helpers with their shared inputs and dependencies.; Establish cohesive boundaries with distinct ownership and collaboration. | Creating stepOne/stepTwo/stepThree helpers solely to satisfy metrics.; Treating a meaningful name as proof of a meaningful boundary.; Suppressing the case without documented justification. |

All cases retain the same template for their smell regardless of severity, suppression, or history; provisional review notes are separate.

## Implementation constraints

Use the Go standard analysis/type ecosystem (`go/packages`, AST, and type information) for source analysis. Columbo may reuse mature libraries for pinned metric algorithms but MUST own its smell rules, case model, output, configuration, virtual-inlining logic, and fixtures.

v1 does not shell out to golangci-lint, Revive, or an LLM.

## Acceptance requirements

The repository MUST contain fixture-based tests for every observable rule above, plus deterministic text/JSON goldens for every smell, suppressed cases, history warnings, and provisional review notes. Tests use an explicitly fixed Go toolchain and environment. The following named acceptance scenarios are mandatory; fixture implementation and private test organization are implementer choices.

| Fixture | Required result |
|---|---|
| literal-cluster-exclusion | Candidate calls inside literals never form clusters, including inside their child blocks; they still count for helper eligibility and enclosing ordinary/expanded metrics. |
| reachable-cluster-ownership | An ancestor without a lexical cluster reports a qualifying cluster in a reachable candidate helper when its own expansion exceeds a limit. Both ancestor and helper may report cases; forwarding uses the physical owner's P. |
| cluster-json-order | Two qualifying clusters and several calls on one line preserve owner, membership, and lexical order through clusters and byte offsets; member clues map to their cluster. |
| expansion-copy-evidence | Separate copies with identical source ranges and declaration chains retain distinct expansion_sites, including a call inside a copied literal also processed as an original child expression. Contribution sums reconcile with expanded metrics. |
| complexity-receipts | Multiple visitor increments at one token with equal diagnostic nesting aggregate by sum; different copy paths/nesting remain separate. Token ranges and pinned diagnostic nesting are locked by goldens. |
| structural-signature-names | Function-type parameter/result names and unnamed-interface method parameter/result names do not change clump keys; method names, type identities, order and variadic distinctions remain significant. |
| named-interface-dependencies | Named interfaces, aliases resolving to them, and generic named-interface instances use type:; genuinely unnamed interfaces use interface:. |
| generic-helper-interface | Candidate-method exclusion uses its unique call-site receiver instantiation, with a matching and a nonmatching instantiation and a caller-type-parameter case; hypothetical instantiations do not disqualify it. |
| dependency-site-ranges | Foreign identifiers/selections, explicit type syntax, callee/results, arguments and nested expression operations use designated ranges; same-site identity roles deduplicate while distinct overlapping sites remain. |
| cgo-physical-source | Original cgo declarations are subjects with physical receipts; generated wrappers add no subjects/references/interfaces. Unavailable original-source/type correspondence exits 2 before stdout delivery. |
| generated-marker | The standard literal-period marker excludes a file; a backslash followed by another character does not satisfy the marker. Existing placement requirements still apply. |
| blank-function-identities | Multiple func _ declarations receive separate file-local _#N symbols and cases, participate in applicable signature/body rules, and cannot be candidates. |
| overlap-comparison-evidence | A passing mean with an individual overlap below its threshold reports null comparison fields on individual overlaps, configured >= on means, and the helper minimum on helper-count. |
| output-write-failure | A pre-delivery fatal error leaves stdout empty. An injected sink accepting a prefix then failing produces exit 2 and a stderr diagnostic; its partial stdout is permitted. |
| threshold-boundaries | Long Function, parameters, complexity, and dependencies: at T no case, at T+1 case. Feature Envy, clump size/support and helper-count/overlaps: at their inclusive minima qualify, below any required minimum do not. Configure other metrics out of the way. |
| severity-and-exits | Each smell alone with fail exits 1; warn-only exits 0; off emits no case. Discovered FAIL plus a fatal error exits 2 with empty stdout. |
| config-merge | Partial severity/threshold overrides retain other defaults; exclude [] clears defaults; omitted exclude retains them; generated exclusion remains mandatory. |
| config-invalid | Duplicate/unknown/null fields, unsupported version, fractional counts, infinity/NaN, zero counts/ratio, helper minimum 1, invalid severity/glob/multidocument YAML all exit 2. Zero and one overlap values are valid. |
| module-and-tests | One-module selected packages and external tests analyze once; multi-module/outside-module/no-module/unmatched patterns fail; in-package declarations shared by test variants do not multiply findings or helper callers. |
| exclusions-and-errors | Excluded/generated declarations create no cases/clump occurrences/directive errors; their second helper call or function-value reference still disqualifies the helper. A type error in an excluded build-selected file exits 2. |
| callable-scope | No independent literal/interface/function-type cases; bodyless declarations participate only in signature rules; ordinary enclosing metrics retain literal contributions. |
| physical-lines | Empty body=0; func f(){ a(); b() }=1; comment/brace-only lines=0; multiline raw-string token includes every spanned line; multiline syntax counts only token-bearing non-brace/non-semicolon lines. |
| dependency-identities | pkg.Client contributes package:pkg plus type:pkg.Client; repeated references do not increase count; two instances of one interface share one dependency; aliases/pointers collapse to their named target, distinct named types remain distinct. |
| dependency-exemptions | Receiver type and truly private same-file type excluded; a same-file private type used in another reference-universe file is included; a callee's body never contributes ordinary dependencies. |
| feature-envy-values | Alias variables remain separate; reassignment preserves variable key; x.Child.Name attributes Child to x and Name to x.Child; call-rooted/index-rooted values do not qualify. |
| feature-envy-selectors | Method selector+invocation counts 1; promoted method counts 1; method expression counts 0; multiple qualifying foreign values produce one case with all receipts. |
| clump-normalization | Names irrelevant; repeated types retain multiplicity; aliases match; distinct named types differ; variadic T matches []T; generic parameters alpha-normalize. |
| clump-closed-support | With support threshold 3, ABC in four signatures and ABCD in three yields both cases; ABC and ABCD with identical support yields only ABCD. Repeated equal signatures at different declarations each contribute support. |
| helper-references | Direct functions/concrete methods/generic instances resolve; first-class/multiple references, excluded callers, or implemented known interface methods disqualify as specified. |
| helper-forwarding | With P={a,b}, helper(a,a) forwards {a}, overlap=0.5; helper(a,b) is 1; helper(a.Field,b) is 0.5; empty P is 0. Use immediate caller P at every level. |
| cluster-boundaries | Opposite if branches, loop/surrounding statements, return/go/defer/literal sites never form one cluster; non-call assignments do not break a sequence; fmt.Errorf/log exceptions do not break it; custom logging calls do. |
| maximal-clusters | A maximal cluster failing mean overlap does not trigger because a smaller subsequence would pass; two passing clusters produce one parent case with both clusters. |
| expanded-lines | Standalone 3-line helper=3; assignment to that call=4; two standalone calls on one line with expanded lengths 2 and 3 total 5; nonexpanded surrounding tokens retain one original bucket. |
| expanded-complexity | Helper with one ordinary if called at caller nesting depth 2 contributes 3; helper return does not terminate traversal; expression calls, go/defer, literals, logical operators, and cycle cutoffs follow the interception model. |
| expansion-boundaries | Stop at noncandidates; expand all reachable candidates, not only qualifying cluster members; retain the first repeated call at a cycle cutoff. Virtual metrics alone never create Long Function/High Cognitive Complexity cases. |
| cosmetic-severity | Turning Long Function and High Cognitive Complexity off does not disable Cosmetic Extraction's threshold comparison. Setting Cosmetic Extraction off does. |
| cosmetic-arbitrary-extraction | stepOne/stepTwo/stepThree-style single-use extraction meeting all conditions remains FAIL; names never change results. |
| cosmetic-meaningful-but-strict | A plausibly meaningful payment/inventory decomposition meeting all conditions remains FAIL and includes CE-001. This fixture documents intentional noise rather than objective architectural invalidity. |
| cosmetic-distinct-boundaries | Distinct forwarding/dependency sets below a required overlap clear Cosmetic Extraction when all other configured smells clear. |
| suppression-validation | Attached single-smell justified directives apply; short/unknown/Data Clump/duplicate/misattached directives are fatal; unrelated comments in the doc group are allowed. |
| suppression-accounting | Suppressed FAIL stays in cases with original verdict, suppressed=true, review/evidence retained, contributes only suppressed count, and does not cause exit 1. Unused/off-smell directives warn. |
| identities-and-schema | IDs survive line shifts of a normal named declaration, severity/history/review/suppression changes; Data Clump anchor/key and init numbering follow the contract; arrays/order/schema and empty arrays match goldens. |
| history-provenance | Commits tie-break by hash; first-parent changed files match receipts; renames are not followed; history availability and max-commits never alter cases' identity/verdict or exit code. Disabled history performs no Git access. |
| history-failure | Discard partial receipts and emit one generic warning; JSON also contains that warning, with no stdout prose. |
| dogfooding-release | Provisional entries render only on applicable emitted cases, including suppressed/WARN; no metric/ID/verdict changes; public release rejects provisional entries and resolved entries generate no notes. |

All seven default FAIL smells must be individually demonstrated. Fixtures must also pin gocognit v1.2.0 values for its supported constructs, including direct recursion and nested literals; README prose about broader recursion does not supersede the pinned visitor. Public-release validation is a checked-in validation command or test selected by the release workflow; ordinary test runs support provisional policies during dogfooding.

## Non-goals for v1

Columbo does not replace the compiler, gofmt, go vet, Staticcheck, security scanners, or correctness linters. It does not automatically rewrite code. It does not use an LLM for enforcement. It does not automatically merge multiple smells into architectural cases. Git-history evidence does not affect verdicts.

## Definition of done

v1 is done when the CLI, configuration, seven smell rules, virtual inlining, suppressions, text/JSON output, stable case identity, optional history evidence, exit semantics, and acceptance fixtures above are implemented and all tests pass.

Dogfooding status defers policy evaluation, not implementation behavior. Public v1 additionally requires the Dogfooding policies release gate to pass. There are no implementation questions intentionally deferred by this specification. If an implementation detail is not externally observable and does not alter these requirements, the implementer may choose it.

