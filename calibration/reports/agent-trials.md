# Agent trials and lead checks

## Setup: agent trials

Each agent got a fresh copy of one killed inject mutant and Columbo's output for the cases the mutant added, and nothing about how the mutant was made. A different model is a flag change (`--model`), and its trials report separately. `agent_read` is the Columbo build whose output the agent read and ran; `scored_by` planted the mutant and scored the fix. They differ only for a trial rescored from its stored fix, which `trials rescore` does only when `scored_by` gives the mutant the same prompt the agent saw.

| model | model_id | prompt_version | trials | agent_read | scored_by | validity | agent_command |
|---|---|---|---|---|---|---|---|
| sonnet | claude-sonnet-5-5 | v1 | 50 | dev+sha256:51786a9a68582a9b | dev+sha256:76525be62d5b4e72 | test | claude -p --safe-mode --restricted --no-session-persistence --model {model} --max-turns {turns} --output-format stream-json --verbose --permission-mode dontAsk --tools Read,Edit,Write,Glob,Grep,Bash --allowedTools "Read,Edit,Write,Glob,Grep,Bash(go build:*),Bash(go vet:*),Bash(go test:*),Bash(columbo:*)" |
| sonnet | claude-sonnet-5-5 | v1 | 5 | dev+sha256:76525be62d5b4e72 | dev+sha256:76525be62d5b4e72 | test | claude -p --safe-mode --restricted --no-session-persistence --model {model} --max-turns {turns} --output-format stream-json --verbose --permission-mode dontAsk --tools Read,Edit,Write,Glob,Grep,Bash --allowedTools "Read,Edit,Write,Glob,Grep,Bash(go build:*),Bash(go vet:*),Bash(go test:*),Bash(columbo:*)" |

## Setup: lead checks

The Columbo build behind the newest lead-check batch for each corpus.

| columbo | validity | corpora | cases | leads |
|---|---|---|---|---|
| dev+sha256:76525be62d5b4e72 | build | 4 | 491 | 1140 |
| dev+sha256:76525be62d5b4e72 | test | 10 | 16 | 43 |

## Outcomes by smell

Counts read k/n over the smell's n trials. Cleared: Columbo no longer reports the planted smell anywhere beyond the unmutated baseline, and the trial is not gamed. Gamed: the fix added a suppression, or put code where Columbo does not look (behind a generated-code header, in an excluded or unselected file, or in a test file); a gamed trial never counts as cleared. Tests held: the mutant's own test files, restored over the fix, pass, or they no longer compile because the fix changed a signature they call (adapted) while the agent's updated tests pass, keep every original test function, and call no function the fix removed. Edited tests: the agent changed a test file. Changed out of scope: the fix removed or moved a case the prompt said to leave alone. Green: cleared, vet passes, tests held, the mutant's other cases are gone, and no case appeared that neither the baseline nor the mutant had. Fix distance is token edits from the fix to the original divided by token edits from the mutant to the original: 0 restored the original, 1 got no closer, above 1 moved further away. Quartiles use the nearest rank.

| smell | model | n | sites | cleared | gamed | tests_held | adapted | edited_tests | introduced_new_cases | changed_out_of_scope | green | distance_min | p25 | median | p75 | distance_max |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| cosmetic-extraction | sonnet | 5 | 3 | 5/5 | 0/5 | 5/5 | 0/5 | 0/5 | 0/5 | 0/5 | 5/5 | 0.16 | 0.40 | 0.42 | 0.45 | 0.79 |
| data-clump | sonnet | 5 | 2 | 5/5 | 0/5 | 5/5 | 5/5 | 5/5 | 0/5 | 0/5 | 5/5 | 0.29 | 0.37 | 0.65 | 0.65 | 0.84 |
| duplicate-code | sonnet | 5 | 5 | 5/5 | 0/5 | 5/5 | 0/5 | 0/5 | 1/5 | 4/5 | 4/5 | 0.00 | 0.46 | 0.49 | 0.87 | 1.29 |
| excessive-dependencies | sonnet | 5 | 5 | 5/5 | 0/5 | 5/5 | 0/5 | 0/5 | 0/5 | 0/5 | 5/5 | 0.00 | 0.00 | 0.02 | 0.42 | 0.71 |
| feature-envy | sonnet | 5 | 3 | 5/5 | 0/5 | 5/5 | 0/5 | 0/5 | 0/5 | 0/5 | 5/5 | 1.87 | 1.93 | 2.33 | 2.50 | 2.67 |
| high-cognitive-complexity | sonnet | 5 | 5 | 5/5 | 0/5 | 5/5 | 0/5 | 0/5 | 0/5 | 5/5 | 5/5 | 1.03 | 1.12 | 1.14 | 1.25 | 1.65 |
| long-function | sonnet | 5 | 1 | 5/5 | 0/5 | 5/5 | 0/5 | 0/5 | 0/5 | 0/5 | 5/5 | 0.00 | 0.00 | 0.00 | 0.00 | 0.21 |
| long-parameter-list | sonnet | 5 | 2 | 5/5 | 0/5 | 5/5 | 0/5 | 0/5 | 0/5 | 0/5 | 5/5 | 0.10 | 0.10 | 0.29 | 0.38 | 0.38 |
| prose-comment | sonnet | 5 | 5 | 5/5 | 0/5 | 5/5 | 0/5 | 0/5 | 0/5 | 0/5 | 5/5 | 0.00 | 0.00 | 0.00 | 0.00 | 0.00 |
| repeated-variant-decision | sonnet | 5 | 1 | 5/5 | 0/5 | 5/5 | 0/5 | 0/5 | 0/5 | 0/5 | 5/5 | 0.00 | 1.57 | 1.57 | 1.58 | 3.39 |
| selection-use-coupling | sonnet | 5 | 2 | 5/5 | 0/5 | 5/5 | 0/5 | 0/5 | 0/5 | 0/5 | 5/5 | 0.00 | 0.17 | 0.17 | 0.17 | 0.45 |

## Agent runs

Wall time is per agent run, summed; runs overlapped, so elapsed time was shorter. Infrastructure errors are runs where the agent never started work; a rerun retries them and they are not scored.

| model | runs | infra_errors | timeouts | turn_cap_stops | nonzero_exits | agent_minutes | cost_usd | min_turns | max_turns |
|---|---|---|---|---|---|---|---|---|---|
| sonnet | 55 | 0 | 0 | 0 | 0 | 14.5 | 1.92 | 4 | 18 |

## Trials

One row per scored trial. Tests is how the mutant's own test files fared against the fix (pass, fail, adapted, or broken); agent_tests is whether the tests as the agent left them pass. Distances are raw token edits. Flags name what the fix did to Columbo's view rather than to the code: config (restored before scoring), suppression, generated-marker, excluded, and moved-to-test hide code, and removed-file only records that a file Columbo analyzed is gone.

| smell | corpus | site | dose | repeat | stop | turns | cleared | vet | tests | agent_tests | introduced_cases | other_remaining | out_of_scope_gone_cases | distance_fixed | distance_mutant | fix_distance | files_changed | test_files_changed | flags | wall_s | cost_usd |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| cosmetic-extraction | cosmetic | example.com/cosmetic.Total | 3 | 0 | success | 5 | 1 | 1 | pass | 1 |  | 0 |  | 41 | 92 | 0.45 | 1 | 0 |  | 10 | 0.02 |
| cosmetic-extraction | cosmetic | example.com/cosmetic.Total | 4 | 0 | success | 9 | 1 | 1 | pass | 1 |  | 0 |  | 72 | 91 | 0.79 | 1 | 0 |  | 21 | 0.04 |
| cosmetic-extraction | deps | example.com/deps.Summary | 3 | 0 | success | 6 | 1 | 1 | pass | 1 |  | 0 |  | 55 | 138 | 0.40 | 1 | 0 |  | 14 | 0.03 |
| cosmetic-extraction | extract | example.com/extract.Summary | 3 | 0 | success | 5 | 1 | 1 | pass | 1 |  | 0 |  | 41 | 98 | 0.42 | 1 | 0 |  | 10 | 0.02 |
| cosmetic-extraction | extract | example.com/extract.Summary | 4 | 0 | success | 5 | 1 | 1 | pass | 1 |  | 0 |  | 22 | 138 | 0.16 | 1 | 0 |  | 9 | 0.02 |
| data-clump | clump | example.com/clump.Address | 2 | 0 | success | 7 | 1 | 1 | adapted | 1 |  | 0 |  | 51 | 78 | 0.65 | 2 | 1 |  | 18 | 0.04 |
| data-clump | clump | example.com/clump.Address | 3 | 0 | success | 10 | 1 | 1 | adapted | 1 |  | 0 |  | 39 | 106 | 0.37 | 2 | 1 |  | 22 | 0.05 |
| data-clump | clump | example.com/clump.Address | 4 | 0 | success | 7 | 1 | 1 | adapted | 1 |  | 0 |  | 40 | 137 | 0.29 | 2 | 1 |  | 18 | 0.04 |
| data-clump | envy | example.com/envy.Customer | 2 | 0 | success | 8 | 1 | 1 | adapted | 1 |  | 0 |  | 108 | 128 | 0.84 | 3 | 1 |  | 21 | 0.05 |
| data-clump | envy | example.com/envy.Customer | 3 | 0 | success | 9 | 1 | 1 | adapted | 1 |  | 0 |  | 108 | 166 | 0.65 | 3 | 1 |  | 23 | 0.06 |
| duplicate-code | cosmetic | example.com/cosmetic.Total | 1 | 0 | success | 11 | 1 | 1 | pass | 1 |  | 0 | long-function@example.com/cosmetic.Total | 115 | 89 | 1.29 | 1 | 0 |  | 22 | 0.05 |
| duplicate-code | deps | example.com/deps.Summary | 1 | 0 | success | 10 | 1 | 1 | pass | 1 |  | 0 | excessive-dependencies@example.com/deps.Summary, long-function@example.com/deps.Summary | 72 | 146 | 0.49 | 1 | 0 |  | 19 | 0.04 |
| duplicate-code | dup | example.com/dup.Score | 1 | 0 | success | 8 | 1 | 1 | pass | 1 | prose-comment@example.com/dup.<package-clause> | 0 | long-function@example.com/dup.Score | 62 | 71 | 0.87 | 1 | 0 |  | 15 | 0.03 |
| duplicate-code | envy | example.com/envy.(*Invoice).Total | 1 | 0 | success | 5 | 1 | 1 | pass | 1 |  | 0 |  | 0 | 78 | 0.00 | 1 | 0 |  | 11 | 0.02 |
| duplicate-code | extract | example.com/extract.Summary | 1 | 0 | success | 6 | 1 | 1 | pass | 1 |  | 0 | long-function@example.com/extract.Summary | 41 | 89 | 0.46 | 1 | 0 |  | 12 | 0.02 |
| excessive-dependencies | deps | example.com/deps.Quiet | 4 | 0 | success | 5 | 1 | 1 | pass | 1 |  | 0 |  | 0 | 58 | 0.00 | 1 | 0 |  | 11 | 0.02 |
| excessive-dependencies | envy | example.com/envy.(*Invoice).Total | 7 | 0 | success | 7 | 1 | 1 | pass | 1 |  | 0 |  | 79 | 112 | 0.71 | 1 | 0 |  | 17 | 0.04 |
| excessive-dependencies | inline | example.com/inline.Total | 7 | 0 | success | 7 | 1 | 1 | pass | 1 |  | 0 |  | 0 | 116 | 0.00 | 1 | 0 |  | 12 | 0.03 |
| excessive-dependencies | selection | example.com/selection.(*Email).Send | 8 | 0 | success | 6 | 1 | 1 | pass | 1 |  | 0 |  | 3 | 131 | 0.02 | 1 | 0 |  | 13 | 0.03 |
| excessive-dependencies | variants | example.com/variants.Label | 7 | 0 | success | 6 | 1 | 1 | pass | 1 |  | 0 |  | 49 | 116 | 0.42 | 1 | 0 |  | 13 | 0.03 |
| feature-envy | envy | example.com/envy.(*Invoice).Audit | 1 | 0 | success | 5 | 1 | 1 | pass | 1 |  | 0 |  | 30 | 12 | 2.50 | 1 | 0 |  | 10 | 0.02 |
| feature-envy | envy | example.com/envy.(*Invoice).Remind | 1 | 0 | success | 5 | 1 | 1 | pass | 1 |  | 0 |  | 32 | 12 | 2.67 | 1 | 0 |  | 10 | 0.02 |
| feature-envy | envy | example.com/envy.(*Invoice).Remind | 1 | 0 | success | 4 | 1 | 1 | pass | 1 |  | 0 |  | 35 | 15 | 2.33 | 1 | 0 |  | 10 | 0.02 |
| feature-envy | envy | example.com/envy.(*Invoice).Total | 1 | 0 | success | 7 | 1 | 1 | pass | 1 |  | 0 |  | 28 | 15 | 1.87 | 1 | 0 |  | 14 | 0.03 |
| feature-envy | envy | example.com/envy.(*Invoice).Total | 1 | 1 | success | 9 | 1 | 1 | pass | 1 |  | 0 |  | 29 | 15 | 1.93 | 1 | 0 |  | 16 | 0.03 |
| high-cognitive-complexity | github.com/spf13/cobra | github.com/spf13/cobra.gen | 7 | 0 | success | 11 | 1 | 1 | pass | 1 |  | 0 | long-function@github.com/spf13/cobra.gen | 280 | 170 | 1.65 | 1 | 0 |  | 41 | 0.10 |
| high-cognitive-complexity | github.com/spf13/cobra | github.com/spf13/cobra.writeFlag | 3 | 0 | success | 10 | 1 | 1 | pass | 1 |  | 0 | high-cognitive-complexity@github.com/spf13/cobra.writeFlagHandler, long-function@github.com/spf13/cobra.writeFlagHandler | 290 | 282 | 1.03 | 1 | 0 |  | 40 | 0.10 |
| high-cognitive-complexity | github.com/spf13/cobra | github.com/spf13/cobra.writeShortFlag | 2 | 0 | success | 9 | 1 | 1 | pass | 1 |  | 0 | high-cognitive-complexity@github.com/spf13/cobra.writeFlagHandler, long-function@github.com/spf13/cobra.writeFlagHandler | 293 | 257 | 1.14 | 1 | 0 |  | 36 | 0.09 |
| high-cognitive-complexity | github.com/spf13/cobra | github.com/spf13/cobra/doc.GenMan | 1 | 0 | success | 18 | 1 | 1 | pass | 1 |  | 0 | excessive-dependencies@github.com/spf13/cobra/doc.genMan, feature-envy@github.com/spf13/cobra/doc.genMan, high-cognitive-complexity@github.com/spf13/cobra/doc.genMan, long-function@github.com/spf13/cobra/doc.genMan | 427 | 341 | 1.25 | 1 | 0 |  | 75 | 0.22 |
| high-cognitive-complexity | github.com/spf13/cobra | github.com/spf13/cobra/doc.manPrintOptions | 1 | 0 | success | 6 | 1 | 1 | pass | 1 |  | 0 | high-cognitive-complexity@github.com/spf13/cobra/doc.manPrintFlags, long-function@github.com/spf13/cobra/doc.manPrintFlags | 193 | 172 | 1.12 | 1 | 0 |  | 26 | 0.06 |
| long-function | inline | example.com/inline.Total | 3 | 0 | success | 5 | 1 | 1 | pass | 1 |  | 0 |  | 0 | 62 | 0.00 | 1 | 0 |  | 10 | 0.02 |
| long-function | inline | example.com/inline.Total | 3 | 1 | success | 5 | 1 | 1 | pass | 1 |  | 0 |  | 0 | 62 | 0.00 | 1 | 0 |  | 12 | 0.02 |
| long-function | inline | example.com/inline.Total | 3 | 2 | success | 6 | 1 | 1 | pass | 1 |  | 0 |  | 0 | 62 | 0.00 | 1 | 0 |  | 11 | 0.02 |
| long-function | inline | example.com/inline.Total | 4 | 0 | success | 6 | 1 | 1 | pass | 1 |  | 0 |  | 0 | 86 | 0.00 | 1 | 0 |  | 13 | 0.02 |
| long-function | inline | example.com/inline.Total | 4 | 1 | success | 5 | 1 | 1 | pass | 1 |  | 0 |  | 18 | 86 | 0.21 | 1 | 0 |  | 12 | 0.02 |
| long-parameter-list | clump | example.com/clump.Address | 1 | 0 | success | 6 | 1 | 1 | pass | 1 |  | 0 |  | 3 | 31 | 0.10 | 1 | 0 |  | 12 | 0.02 |
| long-parameter-list | clump | example.com/clump.Address | 1 | 1 | success | 6 | 1 | 1 | pass | 1 |  | 0 |  | 3 | 31 | 0.10 | 1 | 0 |  | 12 | 0.02 |
| long-parameter-list | clump | example.com/clump.Address | 1 | 2 | success | 6 | 1 | 1 | pass | 1 |  | 0 |  | 9 | 31 | 0.29 | 1 | 0 |  | 11 | 0.02 |
| long-parameter-list | envy | example.com/envy.Customer | 1 | 0 | success | 9 | 1 | 1 | pass | 1 |  | 0 |  | 21 | 56 | 0.38 | 2 | 0 |  | 22 | 0.05 |
| long-parameter-list | envy | example.com/envy.Customer | 1 | 1 | success | 6 | 1 | 1 | pass | 1 |  | 0 |  | 21 | 56 | 0.38 | 2 | 0 |  | 16 | 0.04 |
| prose-comment | clump | example.com/clump.<package-clause> | 1 | 0 | success | 5 | 1 | 1 | pass | 1 |  | 0 |  | 0 | 1 | 0.00 | 1 | 0 |  | 8 | 0.02 |
| prose-comment | deps | example.com/deps.<package-clause> | 1 | 0 | success | 5 | 1 | 1 | pass | 1 |  | 0 |  | 0 | 1 | 0.00 | 1 | 0 |  | 7 | 0.01 |
| prose-comment | envy | example.com/envy.<package-clause> | 1 | 0 | success | 5 | 1 | 1 | pass | 1 |  | 0 |  | 0 | 1 | 0.00 | 1 | 0 |  | 9 | 0.02 |
| prose-comment | inline | example.com/inline.<package-clause> | 1 | 0 | success | 4 | 1 | 1 | pass | 1 |  | 0 |  | 0 | 1 | 0.00 | 1 | 0 |  | 8 | 0.01 |
| prose-comment | variants | example.com/variants.<package-clause> | 1 | 0 | success | 5 | 1 | 1 | pass | 1 |  | 0 |  | 0 | 1 | 0.00 | 1 | 0 |  | 8 | 0.02 |
| repeated-variant-decision | variants | example.com/variants.Label | 2 | 0 | success | 5 | 1 | 1 | pass | 1 |  | 0 |  | 105 | 31 | 3.39 | 1 | 0 |  | 12 | 0.02 |
| repeated-variant-decision | variants | example.com/variants.Label | 2 | 1 | success | 5 | 1 | 1 | pass | 1 |  | 0 |  | 0 | 31 | 0.00 | 1 | 0 |  | 10 | 0.02 |
| repeated-variant-decision | variants | example.com/variants.Label | 2 | 2 | success | 6 | 1 | 1 | pass | 1 |  | 0 |  | 49 | 31 | 1.58 | 1 | 0 |  | 13 | 0.03 |
| repeated-variant-decision | variants | example.com/variants.Label | 3 | 0 | success | 6 | 1 | 1 | pass | 1 |  | 0 |  | 58 | 37 | 1.57 | 1 | 0 |  | 14 | 0.03 |
| repeated-variant-decision | variants | example.com/variants.Label | 3 | 1 | success | 6 | 1 | 1 | pass | 1 |  | 0 |  | 58 | 37 | 1.57 | 1 | 0 |  | 14 | 0.03 |
| selection-use-coupling | selection | example.com/selection.Alert | 1 | 0 | success | 6 | 1 | 1 | pass | 1 |  | 0 |  | 21 | 47 | 0.45 | 1 | 0 |  | 14 | 0.03 |
| selection-use-coupling | selection | example.com/selection.Alert | 1 | 1 | success | 4 | 1 | 1 | pass | 1 |  | 0 |  | 0 | 47 | 0.00 | 1 | 0 |  | 9 | 0.02 |
| selection-use-coupling | selection | example.com/selection.Notify | 1 | 0 | success | 4 | 1 | 1 | pass | 1 |  | 0 |  | 6 | 36 | 0.17 | 1 | 0 |  | 8 | 0.02 |
| selection-use-coupling | selection | example.com/selection.Notify | 1 | 1 | success | 4 | 1 | 1 | pass | 1 |  | 0 |  | 6 | 36 | 0.17 | 1 | 0 |  | 8 | 0.02 |
| selection-use-coupling | selection | example.com/selection.Notify | 1 | 2 | success | 4 | 1 | 1 | pass | 1 |  | 0 |  | 6 | 36 | 0.17 | 1 | 0 |  | 9 | 0.02 |

## Leads by smell

Each lead Columbo wrote on a natural case in the corpora, and what applying the refactoring the harness maps it to did to that same case, with no model involved. Reading says how the harness got there: `says` when the lead's text names the change; `points toward` when the lead asks the reader to inspect something or answers a question, names what to look for, and the harness judged one refactoring the obvious action on finding it; `names none` when it names nothing specific or its answer acts on a test, a contract, or code outside the case. The `points toward` rows are the harness's judgment, listed here so a reader can discount them. Cleared: the cure removed the case and added nothing. Conflict: removed it but set off another rule. Survived: the case remains. Invalid: the cured code failed vet, or on a fixture its tests. No site: the cure has no application aimed at the case; cure operators take sites only where their own target smell is reported, so a cure for another smell usually has none. Refused: the cure had a site at the case and declined to rewrite it. extract-function extracts at most three segments, so a long-function case that survives it shows the limit of that mechanical cure, not that the lead's advice fails. delete-prose-comments deletes the reported comment, and the case is the comment, so its clearing the prose case is close to true by construction; it shows only that deleting the comment sets off nothing else.

| smell | lead | reading | refactoring | cure | cases | cleared | conflict | survived | invalid | no_site | refused |
|---|---|---|---|---|---|---|---|---|---|---|---|
| data-clump | Inspect the participating signatures and struct types for a shared domain concept. | points toward | Introduce Parameter Object | introduce-parameter-object | 7 | 4 | 3 | 0 | 0 | 0 | 0 |
| data-clump | Introduce a parameter object only if the values belong together. | says | Introduce Parameter Object | introduce-parameter-object | 7 | 4 | 3 | 0 | 0 | 0 | 0 |
| data-clump | Every member of this clump has a primitive type. Consider a distinct named type for each kind of value, such as one ID type per identifier; the type checker would then see the clump and reject values passed in each other's place. | says | Replace Primitive with Object | no cure operator | 3 | 0 | 0 | 0 | 0 | 0 | 0 |
| duplicate-code | Compare every duplicate-fragment receipt and identify knowledge that must change together. | points toward | Extract Function | delegate-duplicate | 18 | 2 | 0 | 0 | 0 | 0 | 16 |
| duplicate-code | Share an operation only when the copies own the same policy. | says | Extract Function | delegate-duplicate | 18 | 2 | 0 | 0 | 0 | 0 | 16 |
| excessive-dependencies | Does the caller know details its dependency could own? | points toward | Move Function | move-function | 19 | 0 | 0 | 1 | 0 | 18 | 0 |
| excessive-dependencies | Is policy repeated across boundaries? | points toward | Extract Function | delegate-duplicate | 19 | 0 | 0 | 0 | 0 | 18 | 1 |
| excessive-dependencies | Are behavioral assumptions explicit in the contract? | points toward | Introduce Assertion | no cure operator | 19 | 0 | 0 | 0 | 0 | 0 | 0 |
| excessive-dependencies | Do these dependencies support one cohesive operation? | points toward | Extract Function | split-by-collaborators | 19 | 4 | 1 | 0 | 1 | 0 | 13 |
| excessive-dependencies | Compare supplied arguments and consumed results with types appearing only in signatures or discarded results; origin lists overlap and do not alter the score. | names none |  |  | 19 | 0 | 0 | 0 | 0 | 0 | 0 |
| feature-envy | Review each qualifying foreign value and its contributing accesses. | names none |  |  | 41 | 0 | 0 | 0 | 0 | 0 | 0 |
| feature-envy | A qualifying value's type is declared in another package, so Go cannot take new methods on it. Consider a local type that wraps it and owns this behavior, or a value holding only the fields this code reads. | says | Introduce Local Extension | no cure operator | 14 | 0 | 0 | 0 | 0 | 0 | 0 |
| feature-envy | Consider moving cohesive behavior toward the owner of that data. | says | Move Function | move-function | 30 | 7 | 0 | 0 | 0 | 23 | 0 |
| high-cognitive-complexity | Clarify early exits and simplify nested decisions. | says | Replace Nested Conditional with Guard Clauses | no cure operator | 120 | 0 | 0 | 0 | 0 | 0 | 0 |
| high-cognitive-complexity | Examine whether variants deserve meaningful abstractions. | points toward | Replace Conditional with Polymorphism | replace-conditional-with-polymorphism | 120 | 0 | 0 | 0 | 0 | 120 | 0 |
| long-function | Look for cohesive responsibilities with different reasons to change. | points toward | Extract Function | extract-function | 205 | 53 | 2 | 120 | 0 | 30 | 0 |
| long-function | Move behavior toward the object that owns its data. | says | Move Function | move-function | 205 | 0 | 0 | 4 | 0 | 201 | 0 |
| long-parameter-list | Examine whether inputs form a cohesive domain object. | points toward | Introduce Parameter Object | introduce-parameter-object | 2 | 0 | 0 | 0 | 0 | 2 | 0 |
| long-parameter-list | Separate responsibilities that require different inputs. | says | Extract Function | extract-function | 2 | 0 | 0 | 1 | 0 | 1 | 0 |
| prose-comment | If a comment preserves an intentional omission or guards against a future fix, write a regression test that fails when the unwanted behavior is introduced. Comments are not a reliable enforcement mechanism. | names none |  |  | 89 | 0 | 0 | 0 | 0 | 0 | 0 |
| prose-comment | Express intent through clear names, cohesive structure, and executable contracts; preserve useful design rationale in external documentation. | names none |  |  | 89 | 0 | 0 | 0 | 0 | 0 | 0 |
| prose-comment | Remove obsolete explanations and commented-out code after preserving any required behavior in tests. | says | Delete Comment | delete-prose-comments | 89 | 83 | 0 | 6 | 0 | 0 | 0 |
| repeated-variant-decision | Examine whether callers can send the same message to interchangeable role players instead of selecting behavior themselves. | points toward | Replace Conditional with Polymorphism | replace-conditional-with-polymorphism | 5 | 1 | 0 | 0 | 0 | 4 | 0 |
| repeated-variant-decision | If the variants select implementations, use a factory or construction boundary to select once and return a common behavioral role; callers then send messages through that role. | says | Replace Constructor with Factory Function | extract-factory | 5 | 0 | 0 | 0 | 0 | 5 | 0 |
| repeated-variant-decision | If independent operations inspect the same variants, evaluate a visitor or adapter with one dispatch boundary; keep operation-specific behavior, traversal policy, and results with each operation. | says | Introduce Visitor | no cure operator | 5 | 0 | 0 | 0 | 0 | 0 | 0 |
| repeated-variant-decision | If branches merely map variants to data, consider a data-driven representation owned near the variant definition. | says | Replace Conditional with Lookup Table | no cure operator | 5 | 0 | 0 | 0 | 0 | 0 | 0 |
| repeated-variant-decision | Move behavior toward the object or role that has the knowledge required to perform it. | says | Move Function | move-function | 5 | 0 | 0 | 1 | 0 | 4 | 0 |
| selection-use-coupling | Consider moving implementation selection into a factory or composition boundary that returns the role. | says | Replace Constructor with Factory Function | extract-factory | 1 | 1 | 0 | 0 | 0 | 0 | 0 |
| selection-use-coupling | Consider accepting the role as a dependency instead of deciding which concrete player to use here. | says | Parameterize Function | no cure operator | 1 | 0 | 0 | 0 | 0 | 0 | 0 |
| selection-use-coupling | Keep collaboration expressed in terms of the role while centralizing knowledge of concrete implementations. | points toward | Replace Constructor with Factory Function | extract-factory | 1 | 1 | 0 | 0 | 0 | 0 | 0 |
| selection-use-coupling | The selected implementations already respond to the same required messages. Consider expressing that collaboration as a role owned by the consumer. | says | Extract Interface | no cure operator | 1 | 0 | 0 | 0 | 0 | 0 | 0 |

## Leads by corpus

| corpus_module | corpus_version | validity | cases | leads | leads_with_cure | cleared | conflict | survived | invalid | no_site | refused |
|---|---|---|---|---|---|---|---|---|---|---|---|
| clumpcure | fixture | test | 1 | 3 | 2 | 2 | 0 | 0 | 0 | 0 | 0 |
| cosmetic | fixture | test | 1 | 2 | 2 | 1 | 0 | 0 | 0 | 1 | 0 |
| deps | fixture | test | 2 | 7 | 5 | 2 | 0 | 0 | 0 | 3 | 0 |
| dup | fixture | test | 4 | 8 | 8 | 4 | 0 | 0 | 0 | 2 | 2 |
| envy | fixture | test | 2 | 4 | 2 | 2 | 0 | 0 | 0 | 0 | 0 |
| extract | fixture | test | 1 | 2 | 2 | 1 | 0 | 0 | 0 | 1 | 0 |
| github.com/BurntSushi/toml | v1.5.0 | build | 191 | 427 | 291 | 33 | 1 | 64 | 1 | 179 | 13 |
| github.com/go-chi/chi/v5 | v5.2.1 | build | 133 | 336 | 207 | 51 | 4 | 32 | 0 | 106 | 14 |
| github.com/google/uuid | v1.6.0 | build | 31 | 77 | 42 | 18 | 0 | 4 | 0 | 18 | 2 |
| github.com/spf13/cobra | v1.9.1 | build | 136 | 300 | 206 | 40 | 4 | 33 | 0 | 114 | 15 |
| neutral | fixture | test | 2 | 5 | 4 | 4 | 0 | 0 | 0 | 0 | 0 |
| prose | fixture | test | 1 | 3 | 1 | 1 | 0 | 0 | 0 | 0 | 0 |
| selection | fixture | test | 1 | 4 | 2 | 2 | 0 | 0 | 0 | 0 | 0 |
| variants | fixture | test | 1 | 5 | 3 | 1 | 0 | 0 | 0 | 2 | 0 |

## Unclassified leads

Lead text the harness's lead table does not know. Each one needs a row in trials/leads.go before its check means anything.

None.
