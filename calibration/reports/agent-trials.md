# Agent trials and lead checks

## Setup

Each agent got a fresh copy of one killed inject mutant and Columbo's output for the cases the mutant added, and nothing about how the mutant was made. A different model is a flag change (`--model`), and its trials report separately.

| model | model_id | prompt_version | trials | columbo | validity | agent_command |
|---|---|---|---|---|---|---|
| sonnet | claude-sonnet-5-5 | v1 | 50 | dev+sha256:51786a9a68582a9b | test | claude -p --safe-mode --restricted --no-session-persistence --model {model} --max-turns {turns} --output-format stream-json --verbose --permission-mode dontAsk --tools Read,Edit,Write,Glob,Grep,Bash --allowedTools "Read,Edit,Write,Glob,Grep,Bash(go build:*),Bash(go vet:*),Bash(go test:*),Bash(columbo:*)" |

## Outcomes by smell

Counts read k/n over the smell's n trials. Cleared: Columbo no longer reports the planted smell anywhere beyond the unmutated baseline, with no new suppression or generated-code marker. Changed out of scope: the fix removed or moved a case the prompt said to leave alone. Green: cleared, vet and tests pass, the mutant's other cases are gone, and no case appeared that neither the baseline nor the mutant had. Fix distance is token edits from the fix to the original divided by token edits from the mutant to the original: 0 restored the original, 1 got no closer, above 1 moved further away. Quartiles use the nearest rank.

| smell | model | n | sites | cleared | vet_and_tests_pass | introduced_new_cases | changed_out_of_scope | green | distance_min | p25 | median | p75 | distance_max |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| cosmetic-extraction | sonnet | 5 | 3 | 5/5 | 5/5 | 0/5 | 0/5 | 5/5 | 0.16 | 0.40 | 0.42 | 0.45 | 0.79 |
| data-clump | sonnet | 5 | 2 | 5/5 | 5/5 | 0/5 | 0/5 | 5/5 | 0.29 | 0.37 | 0.65 | 0.65 | 0.84 |
| duplicate-code | sonnet | 5 | 5 | 5/5 | 5/5 | 1/5 | 4/5 | 4/5 | 0.00 | 0.46 | 0.49 | 0.87 | 1.29 |
| excessive-dependencies | sonnet | 5 | 5 | 5/5 | 5/5 | 0/5 | 0/5 | 5/5 | 0.00 | 0.00 | 0.02 | 0.42 | 0.71 |
| feature-envy | sonnet | 5 | 3 | 5/5 | 5/5 | 0/5 | 0/5 | 5/5 | 1.87 | 1.93 | 2.33 | 2.50 | 2.67 |
| high-cognitive-complexity | sonnet | 5 | 5 | 5/5 | 5/5 | 0/5 | 5/5 | 5/5 | 1.03 | 1.12 | 1.14 | 1.25 | 1.65 |
| long-function | sonnet | 5 | 1 | 5/5 | 5/5 | 0/5 | 0/5 | 5/5 | 0.00 | 0.00 | 0.00 | 0.00 | 0.21 |
| long-parameter-list | sonnet | 5 | 2 | 5/5 | 5/5 | 0/5 | 0/5 | 5/5 | 0.10 | 0.10 | 0.29 | 0.38 | 0.38 |
| prose-comment | sonnet | 5 | 5 | 5/5 | 5/5 | 0/5 | 0/5 | 5/5 | 0.00 | 0.00 | 0.00 | 0.00 | 0.00 |
| repeated-variant-decision | sonnet | 5 | 1 | 5/5 | 5/5 | 0/5 | 0/5 | 5/5 | 0.00 | 1.57 | 1.57 | 1.58 | 3.39 |

## Agent runs

Wall time is per agent run, summed; runs overlapped, so elapsed time was shorter. Infrastructure errors are runs where the agent never started work; a rerun retries them and they are not scored.

| model | runs | infra_errors | timeouts | turn_cap_stops | nonzero_exits | agent_minutes | cost_usd | min_turns | max_turns |
|---|---|---|---|---|---|---|---|---|---|
| sonnet | 50 | 0 | 0 | 0 | 0 | 13.7 | 1.81 | 4 | 18 |

## Trials

One row per scored trial. Distances are raw token edits; flags name anything the agent did to Columbo's inputs rather than the code (config is restored before scoring).

| smell | corpus | site | dose | repeat | stop | turns | cleared | vet | tests | introduced_cases | other_remaining | out_of_scope_gone_cases | distance_fixed | distance_mutant | fix_distance | files_changed | test_files_changed | flags | wall_s | cost_usd |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| cosmetic-extraction | cosmetic | example.com/cosmetic.Total | 3 | 0 | success | 5 | 1 | 1 | 1 |  | 0 |  | 41 | 92 | 0.45 | 1 | 0 |  | 10 | 0.02 |
| cosmetic-extraction | cosmetic | example.com/cosmetic.Total | 4 | 0 | success | 9 | 1 | 1 | 1 |  | 0 |  | 72 | 91 | 0.79 | 1 | 0 |  | 21 | 0.04 |
| cosmetic-extraction | deps | example.com/deps.Summary | 3 | 0 | success | 6 | 1 | 1 | 1 |  | 0 |  | 55 | 138 | 0.40 | 1 | 0 |  | 14 | 0.03 |
| cosmetic-extraction | extract | example.com/extract.Summary | 3 | 0 | success | 5 | 1 | 1 | 1 |  | 0 |  | 41 | 98 | 0.42 | 1 | 0 |  | 10 | 0.02 |
| cosmetic-extraction | extract | example.com/extract.Summary | 4 | 0 | success | 5 | 1 | 1 | 1 |  | 0 |  | 22 | 138 | 0.16 | 1 | 0 |  | 9 | 0.02 |
| data-clump | clump | example.com/clump.Address | 2 | 0 | success | 7 | 1 | 1 | 1 |  | 0 |  | 51 | 78 | 0.65 | 2 | 1 |  | 18 | 0.04 |
| data-clump | clump | example.com/clump.Address | 3 | 0 | success | 10 | 1 | 1 | 1 |  | 0 |  | 39 | 106 | 0.37 | 2 | 1 |  | 22 | 0.05 |
| data-clump | clump | example.com/clump.Address | 4 | 0 | success | 7 | 1 | 1 | 1 |  | 0 |  | 40 | 137 | 0.29 | 2 | 1 |  | 18 | 0.04 |
| data-clump | envy | example.com/envy.Customer | 2 | 0 | success | 8 | 1 | 1 | 1 |  | 0 |  | 108 | 128 | 0.84 | 3 | 1 |  | 21 | 0.05 |
| data-clump | envy | example.com/envy.Customer | 3 | 0 | success | 9 | 1 | 1 | 1 |  | 0 |  | 108 | 166 | 0.65 | 3 | 1 |  | 23 | 0.06 |
| duplicate-code | cosmetic | example.com/cosmetic.Total | 1 | 0 | success | 11 | 1 | 1 | 1 |  | 0 | long-function@example.com/cosmetic.Total | 115 | 89 | 1.29 | 1 | 0 |  | 22 | 0.05 |
| duplicate-code | deps | example.com/deps.Summary | 1 | 0 | success | 10 | 1 | 1 | 1 |  | 0 | excessive-dependencies@example.com/deps.Summary, long-function@example.com/deps.Summary | 72 | 146 | 0.49 | 1 | 0 |  | 19 | 0.04 |
| duplicate-code | dup | example.com/dup.Score | 1 | 0 | success | 8 | 1 | 1 | 1 | prose-comment@example.com/dup.<package-clause> | 0 | long-function@example.com/dup.Score | 62 | 71 | 0.87 | 1 | 0 |  | 15 | 0.03 |
| duplicate-code | envy | example.com/envy.(*Invoice).Total | 1 | 0 | success | 5 | 1 | 1 | 1 |  | 0 |  | 0 | 78 | 0.00 | 1 | 0 |  | 11 | 0.02 |
| duplicate-code | extract | example.com/extract.Summary | 1 | 0 | success | 6 | 1 | 1 | 1 |  | 0 | long-function@example.com/extract.Summary | 41 | 89 | 0.46 | 1 | 0 |  | 12 | 0.02 |
| excessive-dependencies | deps | example.com/deps.Quiet | 4 | 0 | success | 5 | 1 | 1 | 1 |  | 0 |  | 0 | 58 | 0.00 | 1 | 0 |  | 11 | 0.02 |
| excessive-dependencies | envy | example.com/envy.(*Invoice).Total | 7 | 0 | success | 7 | 1 | 1 | 1 |  | 0 |  | 79 | 112 | 0.71 | 1 | 0 |  | 17 | 0.04 |
| excessive-dependencies | inline | example.com/inline.Total | 7 | 0 | success | 7 | 1 | 1 | 1 |  | 0 |  | 0 | 116 | 0.00 | 1 | 0 |  | 12 | 0.03 |
| excessive-dependencies | selection | example.com/selection.(*Email).Send | 8 | 0 | success | 6 | 1 | 1 | 1 |  | 0 |  | 3 | 131 | 0.02 | 1 | 0 |  | 13 | 0.03 |
| excessive-dependencies | variants | example.com/variants.Label | 7 | 0 | success | 6 | 1 | 1 | 1 |  | 0 |  | 49 | 116 | 0.42 | 1 | 0 |  | 13 | 0.03 |
| feature-envy | envy | example.com/envy.(*Invoice).Audit | 1 | 0 | success | 5 | 1 | 1 | 1 |  | 0 |  | 30 | 12 | 2.50 | 1 | 0 |  | 10 | 0.02 |
| feature-envy | envy | example.com/envy.(*Invoice).Remind | 1 | 0 | success | 5 | 1 | 1 | 1 |  | 0 |  | 32 | 12 | 2.67 | 1 | 0 |  | 10 | 0.02 |
| feature-envy | envy | example.com/envy.(*Invoice).Remind | 1 | 0 | success | 4 | 1 | 1 | 1 |  | 0 |  | 35 | 15 | 2.33 | 1 | 0 |  | 10 | 0.02 |
| feature-envy | envy | example.com/envy.(*Invoice).Total | 1 | 0 | success | 7 | 1 | 1 | 1 |  | 0 |  | 28 | 15 | 1.87 | 1 | 0 |  | 14 | 0.03 |
| feature-envy | envy | example.com/envy.(*Invoice).Total | 1 | 1 | success | 9 | 1 | 1 | 1 |  | 0 |  | 29 | 15 | 1.93 | 1 | 0 |  | 16 | 0.03 |
| high-cognitive-complexity | github.com/spf13/cobra | github.com/spf13/cobra.gen | 7 | 0 | success | 11 | 1 | 1 | 1 |  | 0 | long-function@github.com/spf13/cobra.gen | 280 | 170 | 1.65 | 1 | 0 |  | 41 | 0.10 |
| high-cognitive-complexity | github.com/spf13/cobra | github.com/spf13/cobra.writeFlag | 3 | 0 | success | 10 | 1 | 1 | 1 |  | 0 | high-cognitive-complexity@github.com/spf13/cobra.writeFlagHandler, long-function@github.com/spf13/cobra.writeFlagHandler | 290 | 282 | 1.03 | 1 | 0 |  | 40 | 0.10 |
| high-cognitive-complexity | github.com/spf13/cobra | github.com/spf13/cobra.writeShortFlag | 2 | 0 | success | 9 | 1 | 1 | 1 |  | 0 | high-cognitive-complexity@github.com/spf13/cobra.writeFlagHandler, long-function@github.com/spf13/cobra.writeFlagHandler | 293 | 257 | 1.14 | 1 | 0 |  | 36 | 0.09 |
| high-cognitive-complexity | github.com/spf13/cobra | github.com/spf13/cobra/doc.GenMan | 1 | 0 | success | 18 | 1 | 1 | 1 |  | 0 | excessive-dependencies@github.com/spf13/cobra/doc.genMan, feature-envy@github.com/spf13/cobra/doc.genMan, high-cognitive-complexity@github.com/spf13/cobra/doc.genMan, long-function@github.com/spf13/cobra/doc.genMan | 427 | 341 | 1.25 | 1 | 0 |  | 75 | 0.22 |
| high-cognitive-complexity | github.com/spf13/cobra | github.com/spf13/cobra/doc.manPrintOptions | 1 | 0 | success | 6 | 1 | 1 | 1 |  | 0 | high-cognitive-complexity@github.com/spf13/cobra/doc.manPrintFlags, long-function@github.com/spf13/cobra/doc.manPrintFlags | 193 | 172 | 1.12 | 1 | 0 |  | 26 | 0.06 |
| long-function | inline | example.com/inline.Total | 3 | 0 | success | 5 | 1 | 1 | 1 |  | 0 |  | 0 | 62 | 0.00 | 1 | 0 |  | 10 | 0.02 |
| long-function | inline | example.com/inline.Total | 3 | 1 | success | 5 | 1 | 1 | 1 |  | 0 |  | 0 | 62 | 0.00 | 1 | 0 |  | 12 | 0.02 |
| long-function | inline | example.com/inline.Total | 3 | 2 | success | 6 | 1 | 1 | 1 |  | 0 |  | 0 | 62 | 0.00 | 1 | 0 |  | 11 | 0.02 |
| long-function | inline | example.com/inline.Total | 4 | 0 | success | 6 | 1 | 1 | 1 |  | 0 |  | 0 | 86 | 0.00 | 1 | 0 |  | 13 | 0.02 |
| long-function | inline | example.com/inline.Total | 4 | 1 | success | 5 | 1 | 1 | 1 |  | 0 |  | 18 | 86 | 0.21 | 1 | 0 |  | 12 | 0.02 |
| long-parameter-list | clump | example.com/clump.Address | 1 | 0 | success | 6 | 1 | 1 | 1 |  | 0 |  | 3 | 31 | 0.10 | 1 | 0 |  | 12 | 0.02 |
| long-parameter-list | clump | example.com/clump.Address | 1 | 1 | success | 6 | 1 | 1 | 1 |  | 0 |  | 3 | 31 | 0.10 | 1 | 0 |  | 12 | 0.02 |
| long-parameter-list | clump | example.com/clump.Address | 1 | 2 | success | 6 | 1 | 1 | 1 |  | 0 |  | 9 | 31 | 0.29 | 1 | 0 |  | 11 | 0.02 |
| long-parameter-list | envy | example.com/envy.Customer | 1 | 0 | success | 9 | 1 | 1 | 1 |  | 0 |  | 21 | 56 | 0.38 | 2 | 0 |  | 22 | 0.05 |
| long-parameter-list | envy | example.com/envy.Customer | 1 | 1 | success | 6 | 1 | 1 | 1 |  | 0 |  | 21 | 56 | 0.38 | 2 | 0 |  | 16 | 0.04 |
| prose-comment | clump | example.com/clump.<package-clause> | 1 | 0 | success | 5 | 1 | 1 | 1 |  | 0 |  | 0 | 1 | 0.00 | 1 | 0 |  | 8 | 0.02 |
| prose-comment | deps | example.com/deps.<package-clause> | 1 | 0 | success | 5 | 1 | 1 | 1 |  | 0 |  | 0 | 1 | 0.00 | 1 | 0 |  | 7 | 0.01 |
| prose-comment | envy | example.com/envy.<package-clause> | 1 | 0 | success | 5 | 1 | 1 | 1 |  | 0 |  | 0 | 1 | 0.00 | 1 | 0 |  | 9 | 0.02 |
| prose-comment | inline | example.com/inline.<package-clause> | 1 | 0 | success | 4 | 1 | 1 | 1 |  | 0 |  | 0 | 1 | 0.00 | 1 | 0 |  | 8 | 0.01 |
| prose-comment | variants | example.com/variants.<package-clause> | 1 | 0 | success | 5 | 1 | 1 | 1 |  | 0 |  | 0 | 1 | 0.00 | 1 | 0 |  | 8 | 0.02 |
| repeated-variant-decision | variants | example.com/variants.Label | 2 | 0 | success | 5 | 1 | 1 | 1 |  | 0 |  | 105 | 31 | 3.39 | 1 | 0 |  | 12 | 0.02 |
| repeated-variant-decision | variants | example.com/variants.Label | 2 | 1 | success | 5 | 1 | 1 | 1 |  | 0 |  | 0 | 31 | 0.00 | 1 | 0 |  | 10 | 0.02 |
| repeated-variant-decision | variants | example.com/variants.Label | 2 | 2 | success | 6 | 1 | 1 | 1 |  | 0 |  | 49 | 31 | 1.58 | 1 | 0 |  | 13 | 0.03 |
| repeated-variant-decision | variants | example.com/variants.Label | 3 | 0 | success | 6 | 1 | 1 | 1 |  | 0 |  | 58 | 37 | 1.57 | 1 | 0 |  | 14 | 0.03 |
| repeated-variant-decision | variants | example.com/variants.Label | 3 | 1 | success | 6 | 1 | 1 | 1 |  | 0 |  | 58 | 37 | 1.57 | 1 | 0 |  | 14 | 0.03 |

## Leads by smell

Each lead Columbo wrote on a natural case in the corpora, and what applying the refactoring it names did to that same case, with no model involved. Cleared: the cure removed the case and added nothing. Conflict: removed it but set off another rule. Survived: the case remains. Not applicable: the cure has no mechanical application at that case. A lead that asks a question or says what to inspect names no refactoring.

| smell | lead | refactoring | cure | cases | cleared | conflict | survived | invalid | not_applicable |
|---|---|---|---|---|---|---|---|---|---|
| data-clump | Inspect the participating signatures and struct types for a shared domain concept. | names no refactoring |  | 7 | 0 | 0 | 0 | 0 | 0 |
| data-clump | Introduce a parameter object only if the values belong together. | Introduce Parameter Object | introduce-parameter-object | 7 | 4 | 3 | 0 | 0 | 0 |
| data-clump | Every member of this clump has a primitive type. Consider a distinct named type for each kind of value, such as one ID type per identifier; the type checker would then see the clump and reject values passed in each other's place. | Replace Primitive with Object | no cure operator | 3 | 0 | 0 | 0 | 0 | 0 |
| duplicate-code | Compare every duplicate-fragment receipt and identify knowledge that must change together. | names no refactoring |  | 18 | 0 | 0 | 0 | 0 | 0 |
| duplicate-code | Share an operation only when the copies own the same policy. | Extract Function | delegate-duplicate | 18 | 2 | 0 | 0 | 0 | 16 |
| excessive-dependencies | Does the caller know details its dependency could own? | names no refactoring |  | 19 | 0 | 0 | 0 | 0 | 0 |
| excessive-dependencies | Is policy repeated across boundaries? | names no refactoring |  | 19 | 0 | 0 | 0 | 0 | 0 |
| excessive-dependencies | Are behavioral assumptions explicit in the contract? | names no refactoring |  | 19 | 0 | 0 | 0 | 0 | 0 |
| excessive-dependencies | Do these dependencies support one cohesive operation? | names no refactoring |  | 19 | 0 | 0 | 0 | 0 | 0 |
| excessive-dependencies | Compare supplied arguments and consumed results with types appearing only in signatures or discarded results; origin lists overlap and do not alter the score. | names no refactoring |  | 19 | 0 | 0 | 0 | 0 | 0 |
| feature-envy | Review each qualifying foreign value and its contributing accesses. | names no refactoring |  | 41 | 0 | 0 | 0 | 0 | 0 |
| feature-envy | A qualifying value's type is declared in another package, so Go cannot take new methods on it. Consider a local type that wraps it and owns this behavior, or a value holding only the fields this code reads. | Introduce Local Extension | no cure operator | 14 | 0 | 0 | 0 | 0 | 0 |
| feature-envy | Consider moving cohesive behavior toward the owner of that data. | Move Function | move-function | 30 | 7 | 0 | 0 | 0 | 23 |
| high-cognitive-complexity | Clarify early exits and simplify nested decisions. | Replace Nested Conditional with Guard Clauses | no cure operator | 120 | 0 | 0 | 0 | 0 | 0 |
| high-cognitive-complexity | Examine whether variants deserve meaningful abstractions. | Replace Conditional with Polymorphism | replace-conditional-with-polymorphism | 120 | 0 | 0 | 0 | 0 | 120 |
| long-function | Look for cohesive responsibilities with different reasons to change. | Extract Function | extract-function | 205 | 53 | 2 | 120 | 0 | 30 |
| long-function | Move behavior toward the object that owns its data. | Move Function | move-function | 205 | 0 | 0 | 4 | 0 | 201 |
| long-parameter-list | Examine whether inputs form a cohesive domain object. | Introduce Parameter Object | introduce-parameter-object | 2 | 0 | 0 | 0 | 0 | 2 |
| long-parameter-list | Separate responsibilities that require different inputs. | Extract Function | extract-function | 2 | 0 | 0 | 1 | 0 | 1 |
| prose-comment | If a comment preserves an intentional omission or guards against a future fix, write a regression test that fails when the unwanted behavior is introduced. Comments are not a reliable enforcement mechanism. | names no refactoring |  | 89 | 0 | 0 | 0 | 0 | 0 |
| prose-comment | Express intent through clear names, cohesive structure, and executable contracts; preserve useful design rationale in external documentation. | names no refactoring |  | 89 | 0 | 0 | 0 | 0 | 0 |
| prose-comment | Remove obsolete explanations and commented-out code after preserving any required behavior in tests. | Remove Dead Code | delete-prose-comments | 89 | 83 | 0 | 6 | 0 | 0 |
| repeated-variant-decision | Examine whether callers can send the same message to interchangeable role players instead of selecting behavior themselves. | Replace Conditional with Polymorphism | replace-conditional-with-polymorphism | 5 | 1 | 0 | 0 | 0 | 4 |
| repeated-variant-decision | If the variants select implementations, use a factory or construction boundary to select once and return a common behavioral role; callers then send messages through that role. | Replace Constructor with Factory Function | extract-factory | 5 | 0 | 0 | 0 | 0 | 5 |
| repeated-variant-decision | If independent operations inspect the same variants, evaluate a visitor or adapter with one dispatch boundary; keep operation-specific behavior, traversal policy, and results with each operation. | Introduce Visitor | no cure operator | 5 | 0 | 0 | 0 | 0 | 0 |
| repeated-variant-decision | If branches merely map variants to data, consider a data-driven representation owned near the variant definition. | Replace Conditional with Lookup Table | no cure operator | 5 | 0 | 0 | 0 | 0 | 0 |
| repeated-variant-decision | Move behavior toward the object or role that has the knowledge required to perform it. | Move Function | move-function | 5 | 0 | 0 | 1 | 0 | 4 |
| selection-use-coupling | Consider moving implementation selection into a factory or composition boundary that returns the role. | Replace Constructor with Factory Function | extract-factory | 1 | 1 | 0 | 0 | 0 | 0 |
| selection-use-coupling | Consider accepting the role as a dependency instead of deciding which concrete player to use here. | Parameterize Function | no cure operator | 1 | 0 | 0 | 0 | 0 | 0 |
| selection-use-coupling | Keep collaboration expressed in terms of the role while centralizing knowledge of concrete implementations. | names no refactoring |  | 1 | 0 | 0 | 0 | 0 | 0 |
| selection-use-coupling | The selected implementations already respond to the same required messages. Consider expressing that collaboration as a role owned by the consumer. | Extract Interface | no cure operator | 1 | 0 | 0 | 0 | 0 | 0 |

## Leads by corpus

| corpus_module | corpus_version | validity | cases | leads | leads_with_cure | cleared | conflict | survived | invalid | not_applicable |
|---|---|---|---|---|---|---|---|---|---|---|
| clumpcure | fixture | test | 1 | 3 | 1 | 1 | 0 | 0 | 0 | 0 |
| cosmetic | fixture | test | 1 | 2 | 2 | 1 | 0 | 0 | 0 | 1 |
| deps | fixture | test | 2 | 7 | 2 | 1 | 0 | 0 | 0 | 1 |
| dup | fixture | test | 4 | 8 | 6 | 3 | 0 | 0 | 0 | 3 |
| envy | fixture | test | 2 | 4 | 2 | 2 | 0 | 0 | 0 | 0 |
| extract | fixture | test | 1 | 2 | 2 | 1 | 0 | 0 | 0 | 1 |
| github.com/BurntSushi/toml | v1.5.0 | build | 191 | 427 | 268 | 31 | 1 | 64 | 0 | 172 |
| github.com/go-chi/chi/v5 | v5.2.1 | build | 133 | 336 | 172 | 49 | 2 | 31 | 0 | 90 |
| github.com/google/uuid | v1.6.0 | build | 31 | 77 | 41 | 18 | 0 | 4 | 0 | 19 |
| github.com/spf13/cobra | v1.9.1 | build | 136 | 300 | 191 | 39 | 2 | 33 | 0 | 117 |
| neutral | fixture | test | 2 | 5 | 2 | 2 | 0 | 0 | 0 | 0 |
| prose | fixture | test | 1 | 3 | 1 | 1 | 0 | 0 | 0 | 0 |
| selection | fixture | test | 1 | 4 | 1 | 1 | 0 | 0 | 0 | 0 |
| variants | fixture | test | 1 | 5 | 3 | 1 | 0 | 0 | 0 | 2 |

## Unclassified leads

Lead text the harness's lead table does not know. Each one needs a row in trials/leads.go before its check means anything.

None.
