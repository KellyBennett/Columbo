# Implementation rationale retained from source comments

These notes were moved from production source when the prose-comment policy was introduced. Existing regression tests remain the enforcement mechanism; these descriptions are supporting documentation.


## cmd/releasecheck/main.go

releasecheck is the public-release gate; ordinary dogfooding builds remain valid.


## internal/columbo/calls.go

callGraph owns the reference universe and helper eligibility. Declaration calls

and whole-file references are separate traversals because package initializers

cannot become lexical helper clusters.

Callee resolution is shared with helper clustering and forwarding receipts.


## internal/columbo/clumps.go

Closed frequent multisets are intersections of supporting signatures.

Keep each intersection once rather than enumerate every possible subset.

clumpEvidence keeps the three support roles named until their clues are complete.


## internal/columbo/complexity.go

Adapted from github.com/uudashr/gocognit v1.2.0 (BSD-3-Clause).

The only traversal extension is the CallExpr interception hook.

Each specialized handler owns its node's traversal; other nodes use ast.Walk.

Visit implements the ast.Visitor interface.


## internal/columbo/config.go

The section schema dispatches assignment without changing validation order.


## internal/columbo/correlation_evidence.go

SelectionContext preserves semantic joins while syntax and type information

are available. Receipt keys point back to evidence owned by the SUC case.


## internal/columbo/cosmetic.go

These grammar tables classify lexical scopes and sequence boundaries. They do

not select helpers or relax policy: every boundary from the original rule is kept.

clusterEvidence owns the distinct evidence roles behind aggregate overlaps.


## internal/columbo/dependency_uses.go

Origins describe evidence, not a second scoring policy. One identity may

occur in several origin lists while consuming the dependency budget once.

Tuple results were previously counted through callable signatures. Annotate

those identities without broadening the collector or changing package scoring.


## internal/columbo/duplicates.go

dupl owns structural matching. Columbo owns scope, deterministic groups and

evidence; no subprocess or independently installed detector is required.

A case must have a typed primary declaration. Package-level matches without

any function declaration are outside the current findings contract. A match

spanning functions retains its entire source range, anchored at the first.


## internal/columbo/envy.go

Feature Envy's normalization also treats address-taking and dereferencing

as access to the same stable value; the shared path resolver owns fields.

valueAccess binds a selected member to the stable value that owns it.


## internal/columbo/expansion.go

expansionContext owns the recursive call path shared by both virtual metrics.

Each child gets its own trace; the stack disqualifies recursive expansion.

A replaced call loses its own syntax while retaining nested-call token ranges.

Nested calls are expanded first, so their masks and copied evidence survive.


## internal/columbo/interfaces.go

interfaceDiscovery follows loaded semantic type graphs, independent of source

exclusions, while the seen set prevents recursive named types from looping.


## internal/columbo/metrics.go

Rebuild only structural components. Named underlying types remain opaque.

dependencyCollector owns recursive type policy; dependencyScan owns source sites.


## internal/columbo/model.go

DeclarationRef is a physical declaration identity. Display subjects are never

used to reconstruct this identity or any evidence relationship.

Case construction belongs to the report model; physical coordinates arrive

as an existing declaration receipt, preserving the same exclusive byte range.

sourceEvidenceKey uses the same private canonical encoding as receipt

normalization. Only aggregate contributions omit their pre-summed value.

clueSupport owns one clue's explicit, de-duplicated receipt references.


## internal/columbo/private.go

Private-type dispersion is deliberately separate from function dependency

fan-out. Function metrics ignore package-private named types; this index

records how broadly those types are spread through production files.


## internal/columbo/references.go

referenceVisitor interprets the two syntax forms that name an identifier or

selected member. Callers own normalization and semantic eligibility.

visitReference returns the result's zero value for unsupported expressions.


## internal/columbo/role_syntax.go

The syntax adapters own arm boundaries for both inference contexts. Each arm

carries its labels, so variant correlation need not rediscover branch syntax.


## internal/columbo/roles.go

RoleCandidate is evidence, never an independently enforceable case.

Unlike selection counting, role identity preserves pointer/value distinctions.


## internal/columbo/selection_control.go

One dispatch table owns statement classification. Each handler owns the

transfer semantics of its syntax, rather than repeating a variant switch.

Write traversal is shared by unsupported-region invalidation and capture

analysis; each caller supplies the meaning of an indirect assignment.


## internal/columbo/selection_dynamic.go

Dynamic construction is a local taint, not an inferred implementation. This

intentionally never follows a called constructor's body.


## internal/columbo/selection_loops.go

Loop solving first stabilizes reaching origins without recording evidence.

A second fixed point propagates finite site/alias sets over those stable

values, so neither iteration order nor an early partial state invents a use.


## internal/columbo/selection_scan.go

Each scan is confined to one named declaration. Unsupported indirect writes

invalidate affected locals; no callee body or closure supplies origins.

A use is an observation of one receiver value, with its physical invocation

and alias route. The site retains only the origins supporting that observation.


## internal/columbo/selection_values.go

Values are immutable snapshots. Copying a local preserves the selected value,

even when the original variable is subsequently overwritten.


## internal/columbo/snapshot_summary.go

RenderSnapshot reads an already completed snapshot. It never evaluates rules

or accepts an analyzer report; the stored outcomes also determine its exit code.


## internal/columbo/source.go

Loading a universe establishes physical declarations before classifying their

type references and calls; no analysis runs on a partially loaded engine.

Test variants contain distinct type-checker objects for the same physical declaration.

Physical source owns token and parameter ranges used by metric receipts.

Physical identity, rather than pointer equality, joins normal/test type objects.

Keep declarations only when analysis emitted evidence involving them. The

evidence is intentionally not a complete program graph.


## internal/columbo/sqlite.go

WriteSnapshot publishes a complete sibling database at a new output path.

Atomic no-clobber creation preserves every existing destination.

The publication seam permits bounded storage-failure tests without changing

the production transaction or weakening destination validation.

Linking atomically requires an absent destination at the actual publication.

The deferred temporary cleanup removes the other name of this same inode.

OpenSnapshot validates and opens an immutable, read-only Columbo snapshot.

Immutable readers do not create journals, WAL files, or shared-memory files.


## internal/columbo/sqlite_schema.go

Snapshot identity aliases preserve the public reporting contract.


## internal/columbo/sqlite_writer.go

A snapshotWriter owns the transaction and the first error. Once an operation

fails, subsequent inserts become no-ops and the transaction is rolled back.

snapshotNesting has already validated a supported signed integer kind.


## internal/columbo/type_visitor.go

typeVisitor chooses which edges of a semantic type graph to follow.

Named types and interfaces need analysis-specific policy; other types expose

their structure through typeComponents.

visitType owns classification, while the visitor owns recursion and results.


## internal/columbo/typeparts.go

Structural components are shared by dependency collection and interface discovery.

Named types and interfaces remain policy decisions for those investigations.

callResultComponents expands multi-value expressions. Single results are

already recorded directly by the dependency collector.


## internal/columbo/value_paths.go

resolvedValuePath owns the identity of a variable-rooted field chain. Resolved

objects distinguish shadowed roots and fields with the same source spelling.

The caller owns normalization policy. Both callers share root/field resolution

while only Feature Envy normalizes address-taking and dereferencing.


## internal/columbo/variant_summary.go

The cursor owns grouping the ordered relational values into one displayed set.


## internal/columbo/variant_types.go

variantTypeFacts owns eligibility and canonical identities derived from Go's

semantic information; syntax scanners never choose a representation of types.


## internal/columbo/variants.go

A site owns its physical evidence. Correlation uses only typed domain and

variant identities; branch bodies never participate.

The index owns domain correlation across all included declarations.

A scanner owns declaration attribution, chain membership and its local sites.

Successful Go type checking establishes representability at every supported

switch/equality. Typed constants must match; untyped constants are permitted.


## internal/snapshotdb/lifecycle.go

SchemaVersion and SQLiteApplicationID identify the only supported snapshot.

The embedded file is also sqlc's schema input. Do not maintain a second DDL.



Snapshot owns a validated immutable read-only database. Only generated

operations escape the boundary; no database handle or SQL callback does.

Writer owns one unpublished database and transaction. A failed write is

rolled back on Close; Complete validates the contents before committing.

Validation cursors own their individual PRAGMA contracts and always close

rows before another operation can reuse the single database connection.
