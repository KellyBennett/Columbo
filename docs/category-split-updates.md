# Category split updates

`category-split-updates` is an observational advisory in ordinary reports and
SQLite. It is retained in staged snapshots but belongs to no stage, has no
severity or thresholds, and does not alter verdicts or exit status.

Two separate decisions in the same statement block explicitly select the same
constant on the same resolved string/integer field path. Within those category
arms, each has direct numeric assignments to resolved output paths, the output
sets are disjoint, and their assignment expressions read a common resolved path
other than the selector or an output. Raw strings and named scalar types qualify.
Identity is lexical: disjoint resolved paths can alias the same runtime storage.

Constant tagged switches and equality if/else chains qualify. A single explicit
category is enough; missing defaults and complementary categories are never
inferred. Nested decisions can supply updates within an arm, but two decision
roots must be separate statements in one block. Multiassignments, indexed or
indirect output paths, function literals, runtime labels, switch fallthrough,
decision initializers, boolean predicates, and alias-following are unsupported.
Explicit writes replacing the selector or any resolved prefix across the pair
exclude it. Distinct receiver roots and promoted/explicit path spellings are not
merged. A repeated category in an if/else chain retains distinct branch identity;
reachability is not inferred.

Receipts preserve complete decisions, selected arms, complete numeric updates,
write destinations and shared input reads. Intervening statements, calls and
conversions, address exposures and indirect writes are explicit unresolved context.
Reads are tied to single-target assignment expressions; tuple RHS expressions do
not create shared input evidence. Access summaries retain representative source
occurrences while complete decisions preserve surrounding conditions and updates.

The shared path identifies a lexical input, not an unchanged runtime value.
Neither calls (including math calls), conversions, pointers, aliases, intervening
input writes nor control flow are proved harmless. Two independently governed
pricing and rewards policies can produce exactly the same finding as fragmented
category behavior. The prompt asks a human to review that distinction; it does
not prove a defect, safe code movement, a common owner or a need for polymorphism.
No category names or domain-specific exceptions participate in detection.
