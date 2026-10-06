# Selection + Use Coupling

Status: provisional dogfooding policy **SUC-001**.

A collaborator should not normally have to know both which player fills a role
and how to use that role. `selection-use-coupling` establishes the narrower
mechanical fact that one named function or method owns both implementation
selection and downstream interface collaboration. It does not prove that a
factory, dependency injection, or polymorphism is the correct design.

## Configuration

```yaml
severity:
  selection-use-coupling: fail
thresholds:
  selection-use-implementations: 2
```

The threshold is an integer greater than or equal to two. The trigger is **>=**,
not the strict greater-than comparison used by some other Columbo metrics.
`warn`, `off`, and declaration-scoped `//columbo:ignore` use existing semantics.
The repository's default-policy self-check runs this rule at FAIL.

## Trigger and local flow

A structured `if`/`else`/`else-if` chain or expression switch qualifies when
its continuing control-flow paths deliver the configured number of distinct,
statically known concrete implementations to a common non-empty interface
value, and that selected value subsequently receives an interface method call
in the same named declaration. An enum domain is unnecessary.

```go
var sender Sender = &EmailSender{}
if sms {
    sender = &SMSSender{}
}
active := sender
return active.Send(msg)
```

The default before the conditional contributes an origin on the unchanged
path. Local assignments, short declarations, interface-to-interface assignments,
neutral interface conversions, and parentheses preserve value provenance.
Aliases retain their value if the original variable is overwritten. A later
overwrite of the receiver kills the old selection evidence. Parallel assignment
evaluates all right-hand values before rebinding destinations.

Return paths do not reach a subsequent merge. Built-in panic terminates a path.
Switch breaks reach the switch merge; nested conditional exits retain their
control-flow destination. All invocation sites reached by one selection are
grouped rather than reported as separate cases. `go` and `defer` calls qualify,
as do interface method expressions with the selected receiver supplied as the
first argument. Method-value extraction alone is not a use.

The analysis is local and deterministic. It does not inspect constructor bodies
or infer identities from names such as New, Build, Create, Factory, or Provider.
A helper with a concrete static result still exposes that implementation. A
helper returning an interface contributes an unknown origin, even if its body
always constructs one known type.

## Identities

A role is the interface type already established by the program, not an inferred
common role. Named interfaces use canonical type identity; anonymous interfaces
use their complete, sorted structural method set, including embedded methods.
Empty interfaces and unfixed type parameters do not qualify. Aliases normalize
to their targets; instantiated generic types retain canonical arguments.

An implementation is a statically known non-interface named type assignable to
the role. Values, pointers, `new`, concrete-returning calls, existing concrete
locals, and statically concrete fields can supply origins. Pointer and value
forms of one named implementation share an identity. Distinct named types remain
distinct even if their structures match. Generic implementation identities retain
fixed type arguments. Nil does not count. Unknown interface origins do not count
and prevent a candidate when their flow obscures the reaching alternatives.

A message uses its method name and canonical signature; parameter names and
receiver spelling are irrelevant. Unexported method identities retain their
package identity.

One case groups every qualifying site in one named declaration for one canonical
role. Its discriminator is the qualified enclosing declaration plus role identity.
Files, byte offsets, local names, constructor names, concrete sets, and message
sets do not participate. Adding or removing alternatives while staying above the
threshold preserves the case ID. Different roles produce different cases.

## Boundaries and exclusions

* A selected value flowing to any return value exempts that selection as a
  construction boundary, including aliases and named returns. Setup calls before
  returning are exempt; no attempt is made to label messages as setup or business
  behavior. Returning an unrelated value does not exempt another selection.
* The language-defined `main.main` entrypoint and top-level `init` functions are
  exempt. Names such as run, build, setup, bootstrap, start, and execute are not.
* Function literals are analysis boundaries. Their selections and invocations
  do not combine with the enclosing function. Captured writes and address-taking
  invalidate affected locals so indirect overwrites cannot invent evidence.
* Passing, returning, printing, comparing, asserting, or discarding an interface
  is not itself an interface message. Flow is not followed through fields,
  globals, channels, maps, slices, ordinary calls, or returned closures.
* Type switches and select statements do not supply selection sites in v1.
  Switches containing fallthrough are excluded. Declarations with goto or labeled
  control transfers are conservatively excluded because their merge would require
  additional control-flow analysis.
* Loops stabilize local reaching origins before collecting evidence. A second
  finite fixed point propagates selection and alias identities across back edges.
  Breaks, continues, post statements, and exit values retain their proper flow.
* Registries, reflection, unsafe recovery, and interface-returning dispatch tables
  do not supply speculative concrete identities. v1 prefers missed findings to
  invented identities.

## Evidence contract

| Clue | Value and subject | Comparison |
| --- | --- | --- |
| selection-use-sites | Number of qualifying decisions; declaration + role | None |
| selected-role | One-element canonical role list; declaration | None |
| selected-implementation-count | Distinct reaching implementations; file:decision byte offset | >= configured threshold |
| selected-implementation-set | Sorted canonical implementations; same decision subject | None |
| selected-message-set | Sorted distinct canonical messages; same decision subject | None |
| selected-use-count | Number of source invocation sites; same decision subject | None |

`selection-decision` covers the complete if/else chain or switch and names its
role. `selection-origin` covers each contributing concrete-producing expression
and names its implementation. `selection-flow` covers only alias assignments
needed between the merge and a use. `selected-message` covers each method
invocation and names its canonical message. Unused aliases have no flow receipts.
Only origins actually reaching a qualifying use contribute to emitted evidence.
Clue-receipt links represent support rather than arbitrary case membership.

SQLite persists the clues, ordered lists, source ranges, relational support,
suppression, history, severity, and policy metadata. Compact summaries and GitHub
annotations expose the role, implementations, messages, decision/origin/flow/use
locations, threshold, and review prompt.

## Guidance

**Why:** Selecting a concrete implementation and collaborating with the resulting
role in the same function couples construction policy to behavioral use. Changes
to implementation selection can therefore affect code whose primary job is
collaboration.

**Diagnosis:** {symbol} selects among multiple concrete implementations of the
same role and subsequently sends messages to the selected collaborator. This may
indicate that implementation selection belongs at a separate construction or
factory boundary.

**Leads:**

* Consider moving implementation selection into a factory or composition boundary
  that returns the role.
* Consider accepting the role as a dependency instead of deciding which concrete
  player to use here.
* Keep collaboration expressed in terms of the role while centralizing knowledge
  of concrete implementations.

**Avoid:**

* Moving only the switch into a helper that still exposes concrete implementation
  details to the caller.
* Wrapping the concrete implementations in another facade solely to reduce the finding.
* Renaming constructors, variables, or implementations.
* Extracting branch bodies while leaving selection and collaboration in the same function.

A factory returning only the interface legitimately clears the consumer finding:
the caller no longer knows which implementation was selected. Injecting the role
or removing downstream collaboration also clears it. Cosmetic extraction of
concrete-returning branch helpers does not.

## Independent rules

Repeated Variant Decision identifies distributed knowledge of a typed variant
domain. This rule identifies co-location of implementation selection and role
collaboration. Neither depends on the other or merges its case with the other.
Centralizing a single interface-returning factory can clear both. Duplicate Code
and Excessive Dependencies also remain independent; similar branch bodies or a
large dependency count are not prerequisites.

## SUC-001

**Decision:** Default to FAIL for proven local concrete-player selection followed
by role collaboration, without requiring repetition elsewhere or proof that a
factory improves the design.

**Rationale:** Test whether separating selection knowledge from collaborator-use
knowledge reliably produces more loosely coupled designs. Construction and
runtime exemptions reduce obvious noise while preserving pressure elsewhere.

**Applicability:** Every emitted Selection + Use Coupling case.

**Review note:** This rule deliberately treats local concrete-player selection
followed by role collaboration as architectural pressure. This is a provisional
dogfooding policy; its review note does not change the case verdict.

**Human-review prompt:** Show the human the selected role, concrete origins,
controlling decision, value-flow evidence, and downstream messages. Discuss whether
this function should own both implementation selection and collaboration before
changing code or requesting suppression.

**Required evidence:** Every qualifying decision, concrete origin, relevant alias
flow, implementation identity, downstream message, and configured threshold.

**Evaluation:** Compare cases where factory extraction or injection improves
boundaries with cases where selection and use legitimately belong together. Both
a strong missing-factory example and intentional short-lived transport selection
fail in the regression fixtures under this provisional policy.

## Validation

`selection_test.go` covers the acceptance fixtures, boundaries, generic/alias
identity, stable IDs, control-flow and reassignment regressions, configuration,
independent/composed rules, and exact evidence. `selection-{fail,warn,off,suppressed}`
goldens cover logical SQLite rows and compact output, including history and
SUC-001 metadata. The default-policy fixture includes selection origins, an alias
chain, and multiple messages; the Checks API regression suite checks its annotation.
