# Calibration report

## Runs

The latest run per corpus module, and how many of its mutants are current. Each operator's numbers come from the newest run that measured it on that corpus.

| id | corpus_module | corpus_version | columbo_version | config_hash | validity | baseline_findings | mutants |
|---|---|---|---|---|---|---|---|
| 4 | github.com/BurntSushi/toml | v1.5.0 | dev+sha256:daee0d56ec98a506 | default | build | 208 | 436 |
| 1 | github.com/KellyBennett/Columbo | 153fa46 | dev+sha256:daee0d56ec98a506 | default | build | 73 | 627 |
| 3 | github.com/go-chi/chi/v5 | v5.2.1 | dev+sha256:daee0d56ec98a506 | default | build | 154 | 424 |
| 5 | github.com/google/uuid | v1.6.0 | dev+sha256:daee0d56ec98a506 | default | build | 31 | 241 |
| 2 | github.com/spf13/cobra | v1.9.1 | dev+sha256:daee0d56ec98a506 | default | build | 152 | 591 |

## Outcomes by operator

Invalid mutants failed vet or tests and are excluded from every rate.

| class | operator | target | killed | survived | equivalent | conflict | id_drift | invalid | kill_rate |
|---|---|---|---|---|---|---|---|---|---|
| cure | delete-prose-comments | prose-comment | 72 | 6 | 0 | 0 | 0 | 0 | 92% |
| cure | extract-function | long-function | 45 | 132 | 0 | 7 | 0 | 0 | 24% |
| cure | extract-function-complexity | high-cognitive-complexity | 37 | 124 | 0 | 4 | 0 | 0 | 22% |
| cure | introduce-parameter-object | data-clump | 1 | 0 | 0 | 4 | 0 | 0 | 20% |
| cure | move-function | feature-envy | 16 | 0 | 0 | 0 | 0 | 0 | 100% |
| cure | split-by-collaborators | excessive-dependencies | 17 | 0 | 0 | 1 | 0 | 0 | 94% |
| evade | copy-fields-to-locals | feature-envy | 14 | 2 | 0 | 0 | 0 | 0 | 88% |
| evade | decision-to-map-lookup | repeated-variant-decision | 0 | 13 | 0 | 0 | 0 | 0 | 0% |
| evade | hide-behind-getters | feature-envy | 16 | 0 | 0 | 0 | 0 | 0 | 100% |
| evade | isolate-helper-calls | cosmetic-extraction | 4 | 10 | 0 | 0 | 0 | 0 | 29% |
| evade | pack-anonymous-struct | data-clump | 0 | 29 | 0 | 0 | 0 | 0 | 0% |
| evade | pack-anonymous-struct-lpl | long-parameter-list | 0 | 31 | 0 | 0 | 0 | 0 | 0% |
| evade | prose-to-string-constant | prose-comment | 0 | 26 | 0 | 0 | 0 | 0 | 0% |
| evade | redteam-replay | data-clump | 0 | 2 | 0 | 0 | 0 | 0 | 0% |
| evade | redteam-replay | excessive-dependencies | 0 | 2 | 0 | 0 | 0 | 0 | 0% |
| evade | redteam-replay | feature-envy | 0 | 2 | 0 | 0 | 0 | 0 | 0% |
| evade | redteam-replay | high-cognitive-complexity | 1 | 1 | 0 | 0 | 0 | 0 | 50% |
| evade | redteam-replay | long-function | 0 | 2 | 0 | 0 | 0 | 0 | 0% |
| evade | redteam-replay | long-parameter-list | 0 | 1 | 0 | 0 | 0 | 0 | 0% |
| evade | redteam-replay | prose-comment | 0 | 2 | 0 | 0 | 0 | 0 | 0% |
| evade | redteam-replay | repeated-variant-decision | 0 | 1 | 0 | 0 | 0 | 0 | 0% |
| evade | rename-and-relabel | duplicate-code | 47 | 0 | 0 | 0 | 0 | 1 | 100% |
| evade | reorder-params | data-clump | 28 | 0 | 0 | 0 | 0 | 0 | 100% |
| evade | route-through-facade | excessive-dependencies | 0 | 125 | 0 | 0 | 0 | 0 | 0% |
| evade | split-forwarding-helpers | long-function | 57 | 20 | 0 | 0 | 0 | 0 | 74% |
| evade | split-forwarding-helpers-complexity | high-cognitive-complexity | 13 | 2 | 0 | 0 | 0 | 0 | 87% |
| inject | add-collaborators | excessive-dependencies | 125 | 0 | 119 | 0 | 0 | 0 | 100% |
| inject | add-prose-comment | prose-comment | 26 | 0 | 0 | 0 | 0 | 0 | 100% |
| inject | duplicate-block | duplicate-code | 48 | 0 | 77 | 0 | 0 | 0 | 100% |
| inject | inline-function | long-function | 48 | 0 | 53 | 0 | 0 | 1 | 100% |
| inject | inline-function-complexity | high-cognitive-complexity | 10 | 0 | 107 | 0 | 0 | 1 | 100% |
| inject | inline-parameter-object | data-clump | 29 | 0 | 31 | 0 | 0 | 0 | 100% |
| inject | inline-parameter-object-lpl | long-parameter-list | 31 | 0 | 0 | 0 | 0 | 0 | 100% |
| inject | move-method-to-collaborator | feature-envy | 27 | 0 | 19 | 0 | 0 | 1 | 100% |
| inject | move-method-to-function | feature-envy | 57 | 0 | 28 | 0 | 0 | 0 | 100% |
| inject | repeat-variant-decision | repeated-variant-decision | 11 | 0 | 8 | 0 | 0 | 0 | 100% |
| inject | split-long-function | cosmetic-extraction | 20 | 89 | 0 | 0 | 0 | 0 | 18% |
| neutral | move-decl-to-file |  | 0 | 0 | 106 | 0 | 0 | 0 |  |
| neutral | noop |  | 0 | 0 | 5 | 0 | 0 | 0 |  |
| neutral | rename-decl |  | 0 | 0 | 110 | 0 | 0 | 0 |  |
| neutral | rename-local |  | 0 | 0 | 124 | 0 | 0 | 0 |  |
| neutral | reorder-decls |  | 0 | 0 | 88 | 0 | 2 | 0 |  |

## Outcomes by corpus

| corpus_module | class | killed | survived | equivalent | conflict | id_drift | invalid |
|---|---|---|---|---|---|---|---|
| github.com/BurntSushi/toml | cure | 34 | 81 | 0 | 4 | 0 | 0 |
| github.com/BurntSushi/toml | evade | 25 | 34 | 0 | 0 | 0 | 0 |
| github.com/BurntSushi/toml | inject | 88 | 18 | 59 | 0 | 0 | 0 |
| github.com/BurntSushi/toml | neutral | 0 | 0 | 92 | 0 | 1 | 0 |
| github.com/KellyBennett/Columbo | cure | 23 | 14 | 0 | 1 | 0 | 0 |
| github.com/KellyBennett/Columbo | evade | 60 | 116 | 0 | 0 | 0 | 0 |
| github.com/KellyBennett/Columbo | inject | 158 | 9 | 143 | 0 | 0 | 2 |
| github.com/KellyBennett/Columbo | neutral | 0 | 0 | 101 | 0 | 0 | 0 |
| github.com/go-chi/chi/v5 | cure | 56 | 77 | 0 | 8 | 0 | 0 |
| github.com/go-chi/chi/v5 | evade | 19 | 38 | 0 | 0 | 0 | 0 |
| github.com/go-chi/chi/v5 | inject | 54 | 20 | 56 | 0 | 0 | 1 |
| github.com/go-chi/chi/v5 | neutral | 0 | 0 | 95 | 0 | 0 | 0 |
| github.com/google/uuid | cure | 20 | 10 | 0 | 0 | 0 | 0 |
| github.com/google/uuid | evade | 11 | 31 | 0 | 0 | 0 | 1 |
| github.com/google/uuid | inject | 44 | 1 | 70 | 0 | 0 | 0 |
| github.com/google/uuid | neutral | 0 | 0 | 53 | 0 | 0 | 0 |
| github.com/spf13/cobra | cure | 55 | 80 | 0 | 3 | 0 | 0 |
| github.com/spf13/cobra | evade | 65 | 52 | 0 | 0 | 0 | 0 |
| github.com/spf13/cobra | inject | 88 | 41 | 114 | 0 | 0 | 0 |
| github.com/spf13/cobra | neutral | 0 | 0 | 92 | 0 | 1 | 0 |

## Kill matrix

Valid inject and evade mutants, by operator and by every smell they added anywhere. Read across a row to see which rules notice an operator; down a column to see what a rule notices.

| operator | smell | mutants |
|---|---|---|
| add-collaborators | long-function | 132 |
| add-collaborators | excessive-dependencies | 125 |
| add-prose-comment | prose-comment | 26 |
| copy-fields-to-locals | feature-envy | 14 |
| copy-fields-to-locals | long-function | 7 |
| copy-fields-to-locals | high-cognitive-complexity | 5 |
| copy-fields-to-locals | excessive-dependencies | 1 |
| duplicate-block | duplicate-code | 48 |
| duplicate-block | long-function | 19 |
| duplicate-block | high-cognitive-complexity | 10 |
| duplicate-block | feature-envy | 10 |
| duplicate-block | excessive-dependencies | 6 |
| duplicate-block | data-clump | 5 |
| duplicate-block | repeated-variant-decision | 1 |
| duplicate-block | long-parameter-list | 1 |
| hide-behind-getters | feature-envy | 16 |
| hide-behind-getters | long-function | 5 |
| hide-behind-getters | high-cognitive-complexity | 5 |
| hide-behind-getters | excessive-dependencies | 1 |
| inline-function | long-function | 48 |
| inline-function | excessive-dependencies | 20 |
| inline-function | duplicate-code | 9 |
| inline-function | high-cognitive-complexity | 7 |
| inline-function | feature-envy | 2 |
| inline-function-complexity | long-function | 39 |
| inline-function-complexity | excessive-dependencies | 20 |
| inline-function-complexity | high-cognitive-complexity | 10 |
| inline-function-complexity | duplicate-code | 10 |
| inline-function-complexity | feature-envy | 7 |
| inline-parameter-object | long-parameter-list | 50 |
| inline-parameter-object | feature-envy | 29 |
| inline-parameter-object | data-clump | 29 |
| inline-parameter-object | excessive-dependencies | 23 |
| inline-parameter-object | duplicate-code | 6 |
| inline-parameter-object | long-function | 4 |
| inline-parameter-object-lpl | long-parameter-list | 31 |
| inline-parameter-object-lpl | feature-envy | 12 |
| inline-parameter-object-lpl | data-clump | 11 |
| inline-parameter-object-lpl | excessive-dependencies | 8 |
| isolate-helper-calls | data-clump | 10 |
| isolate-helper-calls | feature-envy | 7 |
| isolate-helper-calls | long-function | 4 |
| isolate-helper-calls | high-cognitive-complexity | 4 |
| isolate-helper-calls | long-parameter-list | 3 |
| isolate-helper-calls | excessive-dependencies | 3 |
| isolate-helper-calls | cosmetic-extraction | 2 |
| move-method-to-collaborator | feature-envy | 28 |
| move-method-to-collaborator | data-clump | 1 |
| move-method-to-function | feature-envy | 58 |
| move-method-to-function | excessive-dependencies | 5 |
| move-method-to-function | duplicate-code | 2 |
| move-method-to-function | data-clump | 1 |
| pack-anonymous-struct | long-function | 24 |
| pack-anonymous-struct | feature-envy | 19 |
| pack-anonymous-struct | duplicate-code | 15 |
| pack-anonymous-struct | excessive-dependencies | 13 |
| pack-anonymous-struct-lpl | long-function | 19 |
| pack-anonymous-struct-lpl | feature-envy | 12 |
| pack-anonymous-struct-lpl | excessive-dependencies | 8 |
| pack-anonymous-struct-lpl | duplicate-code | 7 |
| redteam-replay | long-function | 1 |
| redteam-replay | excessive-dependencies | 1 |
| rename-and-relabel | duplicate-code | 47 |
| rename-and-relabel | long-function | 19 |
| rename-and-relabel | high-cognitive-complexity | 10 |
| rename-and-relabel | feature-envy | 9 |
| rename-and-relabel | excessive-dependencies | 6 |
| rename-and-relabel | data-clump | 4 |
| rename-and-relabel | long-parameter-list | 1 |
| reorder-params | data-clump | 28 |
| reorder-params | long-parameter-list | 26 |
| reorder-params | feature-envy | 19 |
| reorder-params | excessive-dependencies | 13 |
| reorder-params | duplicate-code | 5 |
| reorder-params | long-function | 3 |
| repeat-variant-decision | repeated-variant-decision | 11 |
| route-through-facade | feature-envy | 53 |
| route-through-facade | long-function | 25 |
| split-forwarding-helpers | long-function | 39 |
| split-forwarding-helpers | duplicate-code | 28 |
| split-forwarding-helpers | cosmetic-extraction | 24 |
| split-forwarding-helpers | excessive-dependencies | 21 |
| split-forwarding-helpers | long-parameter-list | 20 |
| split-forwarding-helpers | data-clump | 17 |
| split-forwarding-helpers | high-cognitive-complexity | 12 |
| split-forwarding-helpers-complexity | long-function | 15 |
| split-forwarding-helpers-complexity | high-cognitive-complexity | 12 |
| split-forwarding-helpers-complexity | duplicate-code | 12 |
| split-forwarding-helpers-complexity | excessive-dependencies | 6 |
| split-forwarding-helpers-complexity | data-clump | 5 |
| split-forwarding-helpers-complexity | cosmetic-extraction | 5 |
| split-forwarding-helpers-complexity | long-parameter-list | 3 |
| split-forwarding-helpers-complexity | feature-envy | 2 |
| split-long-function | long-function | 70 |
| split-long-function | high-cognitive-complexity | 39 |
| split-long-function | long-parameter-list | 31 |
| split-long-function | data-clump | 28 |
| split-long-function | excessive-dependencies | 21 |
| split-long-function | cosmetic-extraction | 20 |
| split-long-function | feature-envy | 17 |
| split-long-function | repeated-variant-decision | 2 |
| split-long-function | duplicate-code | 2 |

## Operator families no rule notices

Valid inject or evade operators whose mutants never added any finding. Each is a candidate missing rule.

| operator | valid_mutants |
|---|---|
| decision-to-map-lookup | 13 |
| prose-to-string-constant | 26 |

## Redundant rules

A smell whose every killing mutant also added another given smell. The second rule already notices everything the first does on this corpus.

None.

## Dose curves

For each inject operator and site, the first dose that killed. A site with no killing dose is counted under never.

| operator | first_killing_dose | sites |
|---|---|---|
| add-collaborators | 1 | 6 |
| add-collaborators | 2 | 11 |
| add-collaborators | 3 | 16 |
| add-collaborators | 4 | 24 |
| add-collaborators | 5 | 40 |
| add-collaborators | 6 | 28 |
| add-prose-comment | 1 | 4 |
| duplicate-block | never | 77 |
| duplicate-block | 1 | 48 |
| inline-function | never | 38 |
| inline-function | 1 | 25 |
| inline-function | 2 | 4 |
| inline-function | 3 | 3 |
| inline-function | 4 | 1 |
| inline-function-complexity | never | 71 |
| inline-function-complexity | 1 | 6 |
| inline-function-complexity | 3 | 1 |
| inline-function-complexity | 7 | 1 |
| inline-parameter-object | never | 14 |
| inline-parameter-object | 1 | 11 |
| inline-parameter-object | 2 | 1 |
| inline-parameter-object | 3 | 6 |
| inline-parameter-object-lpl | 1 | 31 |
| move-method-to-collaborator | never | 16 |
| move-method-to-collaborator | 1 | 23 |
| move-method-to-collaborator | 2 | 3 |
| move-method-to-collaborator | 3 | 1 |
| move-method-to-function | never | 28 |
| move-method-to-function | 1 | 57 |
| repeat-variant-decision | 2 | 8 |
| split-long-function | never | 49 |
| split-long-function | 2 | 3 |
| split-long-function | 3 | 7 |
| split-long-function | 4 | 1 |

## Prediction agreement

Inject and cure mutants whose operator predicted the outcome. Disagreements are either harness metric bugs or Columbo behavior worth a look.

| operator | predicted | predicted_kill_but_survived | predicted_survive_but_killed |
|---|---|---|---|
| add-collaborators | 244 | 0 | 0 |
| add-prose-comment | 26 | 0 | 0 |
| delete-prose-comments | 0 | 0 | 0 |
| duplicate-block | 125 | 0 | 0 |
| extract-function | 184 | 0 | 1 |
| extract-function-complexity | 165 | 0 | 0 |
| inline-function | 101 | 0 | 0 |
| inline-function-complexity | 117 | 0 | 0 |
| inline-parameter-object | 60 | 0 | 0 |
| inline-parameter-object-lpl | 31 | 0 | 0 |
| introduce-parameter-object | 0 | 0 | 0 |
| move-function | 16 | 0 | 0 |
| move-method-to-collaborator | 46 | 0 | 0 |
| move-method-to-function | 85 | 0 | 0 |
| repeat-variant-decision | 19 | 0 | 0 |
| split-by-collaborators | 18 | 0 | 0 |
| split-long-function | 0 | 0 | 0 |

## Cure conflicts

A cure that cleared its target but set off another rule. If a proper refactoring trips a different rule, that code has no green path.

| operator | site_symbol | dose | size | reason |
|---|---|---|---|---|
| extract-function | github.com/BurntSushi/toml.(*parser).next | 1 | 111 | cleared target but added feature-envy@toml.(*parser).nextPart1 |
| extract-function | internal/columbo.(*file).inspectionType | 2 | 376 | cleared target but added excessive-dependencies@columbo.(*file).inspectionTypePart2; moved excessive-dependencies@columbo.(*file).inspectionTypePart1 |
| extract-function | github.com/go-chi/chi/v5.(*node).FindRoute | 1 | 76 | cleared target but added feature-envy@v5.findRoutePart1 |
| extract-function | github.com/go-chi/chi/v5.(*node).FindRoute | 2 | 264 | cleared target but added feature-envy@v5.findRoutePart2; moved feature-envy@v5.findRoutePart1 |
| extract-function | github.com/go-chi/chi/v5.(nodes).findEdge | 3 | 188 | cleared target but added long-parameter-list@v5.(nodes).findEdgePart2 |
| extract-function | github.com/go-chi/chi/v5/middleware.Profiler | 2 | 372 | cleared target but added excessive-dependencies@middleware.profilerPart2; moved excessive-dependencies@middleware.profilerPart1, feature-envy@middleware.profilerPart2 |
| extract-function | github.com/go-chi/chi/v5/middleware.Profiler | 3 | 436 | cleared target but added excessive-dependencies@middleware.profilerPart2; moved excessive-dependencies@middleware.profilerPart1, feature-envy@middleware.profilerPart2 |
| extract-function-complexity | github.com/BurntSushi/toml.(*MetaData).unify | 3 | 482 | cleared target but added long-function@toml.(*MetaData).unifyPart3; moved repeated-variant-decision@toml.(*MetaData).unifyPart3 |
| extract-function-complexity | github.com/BurntSushi/toml.(*parser).setValue | 3 | 235 | cleared target but added long-parameter-list@toml.(*parser).setValuePart2 |
| extract-function-complexity | github.com/go-chi/chi/v5.(*node).findEdge | 2 | 141 | cleared target but added long-parameter-list@v5.findEdgePart2 |
| extract-function-complexity | github.com/spf13/cobra.(*Command).SuggestionsFor | 2 | 189 | cleared target but added data-clump@cobra.(*Command).suggestionsForPart1 |
| introduce-parameter-object | github.com/BurntSushi/toml.(*Encoder).eMapOrStruct | 1 | 196 | cleared target but added feature-envy@toml.(*Encoder).eMap |
| introduce-parameter-object | github.com/go-chi/chi/v5/middleware.(prettyStack).decorateLine | 1 | 108 | cleared target but added feature-envy@middleware.(prettyStack).decorateFuncCallLine, feature-envy@middleware.(prettyStack).decorateLine, feature-envy@middleware.(prettyStack).decorateSourceLine |
| introduce-parameter-object | github.com/spf13/cobra.genBashComp | 1 | 132 | cleared target but added duplicate-code@cobra.(*Command).genBashCompletion, feature-envy@cobra.genFishComp, feature-envy@cobra.genPowerShellComp |
| introduce-parameter-object | github.com/spf13/cobra/doc.GenMarkdownTreeCustom | 1 | 196 | cleared target but added duplicate-code@doc.GenMarkdownTreeCustom, excessive-dependencies@doc.GenMarkdownTreeCustom, excessive-dependencies@doc.GenReSTTreeCustom, excessive-dependencies@doc.GenYamlTreeCustom, feature-envy@doc.GenMarkdownTreeCustom, feature-envy@doc.GenReSTTreeCustom, feature-envy@doc.GenYamlTreeCustom |
| split-by-collaborators | github.com/go-chi/chi/v5/middleware.Profiler | 1 | 252 | cleared target but added feature-envy@middleware.profilerPart, long-function@middleware.profilerPart |

## Conflicting rules

Which rules the conflicts set off, per cure operator.

| operator | smell | added |
|---|---|---|
| extract-function | feature-envy | 6 |
| extract-function | excessive-dependencies | 6 |
| extract-function | long-parameter-list | 1 |
| extract-function-complexity | long-parameter-list | 2 |
| extract-function-complexity | repeated-variant-decision | 1 |
| extract-function-complexity | long-function | 1 |
| extract-function-complexity | data-clump | 1 |
| introduce-parameter-object | feature-envy | 9 |
| introduce-parameter-object | excessive-dependencies | 3 |
| introduce-parameter-object | duplicate-code | 2 |
| split-by-collaborators | long-function | 1 |
| split-by-collaborators | feature-envy | 1 |

## Cost asymmetry

Per smell: the smallest surviving evade against the smallest successful cure, in changed AST nodes. The README promises a violation is harder to silence than to properly refactor; that holds where the evade is larger.

| smell | smallest_surviving_evade | surviving_evades | smallest_green_cure | green_cures | promise |
|---|---|---|---|---|---|
| cosmetic-extraction | 213 | 10 | no cure reached green | 0 | fails (no green cure) |
| data-clump | 14 | 31 | 864 | 1 | fails |
| excessive-dependencies | 41 | 127 | 30 | 17 | holds |
| feature-envy | 9 | 4 | 117 | 16 | fails |
| high-cognitive-complexity | 49 | 3 | 26 | 37 | holds |
| long-function | 12 | 22 | 41 | 45 | fails |
| long-parameter-list | 53 | 32 | no cure reached green | 0 | fails (no green cure) |
| prose-comment | 1 | 28 | 1 | 72 | fails |
| repeated-variant-decision | 38 | 14 | no cure reached green | 0 | fails (no green cure) |

## Neutral drift

| operator | site_symbol | reason |
|---|---|---|
| reorder-decls | flag_groups.go | unexplained duplicate-code C-c912a2e518 on cobra.(*Command).MarkFlagsMutuallyExclusive |
| reorder-decls | lex.go | unexplained duplicate-code C-5e9a8436cb on toml.lexLongUnicodeEscape |

## Triaged survivors

| operator | site_symbol | dose | status | reason |
|---|---|---|---|---|
| copy-fields-to-locals | internal/columbo.metricCheckExplain | 1 | blind-spot | Feature Envy counts reads, not distinct fields, so hoisting reads into locals silences a case whenever the envied value has fewer distinct fields than feature-envy-foreign-accesses. Fixture: fixtures/envy. |
| copy-fields-to-locals | github.com/go-chi/chi/v5.nodeWalk | 1 | blind-spot | Feature Envy counts reads, not distinct fields, so hoisting reads into locals silences a case whenever the envied value has fewer distinct fields than feature-envy-foreign-accesses. Fixture: fixtures/envy. |
| decision-to-map-lookup | internal/columbo.choiceRepresentationVariant | 1 | blind-spot | A map from the same variants to values repeats the variant knowledge at a second site, but Columbo counts only switches and if-chains as decision sites. Fixture: fixtures/variants. |
| decision-to-map-lookup | internal/columbo.genericOriginVariant | 1 | blind-spot | A map from the same variants to values repeats the variant knowledge at a second site, but Columbo counts only switches and if-chains as decision sites. Fixture: fixtures/variants. |
| decision-to-map-lookup | internal/columbo.overwriteBits | 1 | blind-spot | A map from the same variants to values repeats the variant knowledge at a second site, but Columbo counts only switches and if-chains as decision sites. Fixture: fixtures/variants. |
| decision-to-map-lookup | internal/columbo.overwriteBitsVariant | 1 | blind-spot | A map from the same variants to values repeats the variant knowledge at a second site, but Columbo counts only switches and if-chains as decision sites. Fixture: fixtures/variants. |
| decision-to-map-lookup | internal/columbo.overwriteBits | 1 | blind-spot | A map from the same variants to values repeats the variant knowledge at a second site, but Columbo counts only switches and if-chains as decision sites. Fixture: fixtures/variants. |
| decision-to-map-lookup | internal/columbo.overwriteBitsVariant | 1 | blind-spot | A map from the same variants to values repeats the variant knowledge at a second site, but Columbo counts only switches and if-chains as decision sites. Fixture: fixtures/variants. |
| decision-to-map-lookup | internal/columbo.binaryVariant | 1 | blind-spot | A map from the same variants to values repeats the variant knowledge at a second site, but Columbo counts only switches and if-chains as decision sites. Fixture: fixtures/variants. |
| decision-to-map-lookup | internal/columbo.visitTypeVariant | 1 | blind-spot | A map from the same variants to values repeats the variant knowledge at a second site, but Columbo counts only switches and if-chains as decision sites. Fixture: fixtures/variants. |
| decision-to-map-lookup | github.com/google/uuid.stringVariant | 1 | blind-spot | A map from the same variants to values repeats the variant knowledge at a second site, but Columbo counts only switches and if-chains as decision sites. Fixture: fixtures/variants. |
| decision-to-map-lookup | github.com/google/uuid.stringVariant | 1 | blind-spot | A map from the same variants to values repeats the variant knowledge at a second site, but Columbo counts only switches and if-chains as decision sites. Fixture: fixtures/variants. |
| decision-to-map-lookup | github.com/google/uuid.timeVariant | 1 | blind-spot | A map from the same variants to values repeats the variant knowledge at a second site, but Columbo counts only switches and if-chains as decision sites. Fixture: fixtures/variants. |
| decision-to-map-lookup | github.com/google/uuid.stringVariant | 1 | blind-spot | A map from the same variants to values repeats the variant knowledge at a second site, but Columbo counts only switches and if-chains as decision sites. Fixture: fixtures/variants. |
| decision-to-map-lookup | github.com/google/uuid.stringVariant | 1 | blind-spot | A map from the same variants to values repeats the variant knowledge at a second site, but Columbo counts only switches and if-chains as decision sites. Fixture: fixtures/variants. |
| isolate-helper-calls | github.com/spf13/cobra.defaultHelpFunc | 1 | blind-spot | Wrapping each helper call of a cosmetic parent in a bare block ends the cluster sequence, because SPEC cluster formation treats an explicit nested block as a sequence break. The design is unchanged and no rule reports it. Fixture: fixtures/cosmetic. |
| isolate-helper-calls | github.com/spf13/cobra/doc.manPreamble | 1 | blind-spot | Wrapping each helper call of a cosmetic parent in a bare block ends the cluster sequence, because SPEC cluster formation treats an explicit nested block as a sequence break. The design is unchanged and no rule reports it. Fixture: fixtures/cosmetic. |
| isolate-helper-calls | github.com/spf13/cobra/doc.manPreamble | 1 | blind-spot | Wrapping each helper call of a cosmetic parent in a bare block ends the cluster sequence, because SPEC cluster formation treats an explicit nested block as a sequence break. The design is unchanged and no rule reports it. Fixture: fixtures/cosmetic. |
| isolate-helper-calls | github.com/spf13/cobra/doc.manPreamble | 1 | blind-spot | Wrapping each helper call of a cosmetic parent in a bare block ends the cluster sequence, because SPEC cluster formation treats an explicit nested block as a sequence break. The design is unchanged and no rule reports it. Fixture: fixtures/cosmetic. |
| isolate-helper-calls | github.com/spf13/cobra/doc.printOptions | 1 | blind-spot | Wrapping each helper call of a cosmetic parent in a bare block ends the cluster sequence, because SPEC cluster formation treats an explicit nested block as a sequence break. The design is unchanged and no rule reports it. Fixture: fixtures/cosmetic. |
| isolate-helper-calls | github.com/spf13/cobra/doc.printOptions | 1 | blind-spot | Wrapping each helper call of a cosmetic parent in a bare block ends the cluster sequence, because SPEC cluster formation treats an explicit nested block as a sequence break. The design is unchanged and no rule reports it. Fixture: fixtures/cosmetic. |
| isolate-helper-calls | github.com/spf13/cobra/doc.printOptionsReST | 1 | blind-spot | Wrapping each helper call of a cosmetic parent in a bare block ends the cluster sequence, because SPEC cluster formation treats an explicit nested block as a sequence break. The design is unchanged and no rule reports it. Fixture: fixtures/cosmetic. |
| isolate-helper-calls | github.com/spf13/cobra/doc.printOptionsReST | 1 | blind-spot | Wrapping each helper call of a cosmetic parent in a bare block ends the cluster sequence, because SPEC cluster formation treats an explicit nested block as a sequence break. The design is unchanged and no rule reports it. Fixture: fixtures/cosmetic. |
| isolate-helper-calls | github.com/BurntSushi/toml.expandTab | 1 | blind-spot | Wrapping each helper call of a cosmetic parent in a bare block ends the cluster sequence, because SPEC cluster formation treats an explicit nested block as a sequence break. The design is unchanged and no rule reports it. Fixture: fixtures/cosmetic. |
| isolate-helper-calls | github.com/BurntSushi/toml.expandTab | 1 | blind-spot | Wrapping each helper call of a cosmetic parent in a bare block ends the cluster sequence, because SPEC cluster formation treats an explicit nested block as a sequence break. The design is unchanged and no rule reports it. Fixture: fixtures/cosmetic. |
| pack-anonymous-struct | internal/columbo.coordinationDecision | 1 | blind-spot | One anonymous struct parameter is one canonical type, so both data-clump and long-parameter-list go quiet while every caller now spells the same struct type out. Fixture: fixtures/clump. |
| pack-anonymous-struct | internal/columbo.coordinationDecision | 1 | blind-spot | One anonymous struct parameter is one canonical type, so both data-clump and long-parameter-list go quiet while every caller now spells the same struct type out. Fixture: fixtures/clump. |
| pack-anonymous-struct | internal/columbo.RoleCandidate | 1 | blind-spot | One anonymous struct parameter is one canonical type, so both data-clump and long-parameter-list go quiet while every caller now spells the same struct type out. Fixture: fixtures/clump. |
| pack-anonymous-struct | internal/columbo.stageDefinition | 1 | blind-spot | One anonymous struct parameter is one canonical type, so both data-clump and long-parameter-list go quiet while every caller now spells the same struct type out. Fixture: fixtures/clump. |
| pack-anonymous-struct | internal/columbo.tangleComparison | 1 | blind-spot | One anonymous struct parameter is one canonical type, so both data-clump and long-parameter-list go quiet while every caller now spells the same struct type out. Fixture: fixtures/clump. |
| pack-anonymous-struct | internal/columbo.file | 1 | blind-spot | One anonymous struct parameter is one canonical type, so both data-clump and long-parameter-list go quiet while every caller now spells the same struct type out. Fixture: fixtures/clump. |
| pack-anonymous-struct | internal/columbo.file | 1 | blind-spot | One anonymous struct parameter is one canonical type, so both data-clump and long-parameter-list go quiet while every caller now spells the same struct type out. Fixture: fixtures/clump. |
| pack-anonymous-struct | internal/columbo.file | 1 | blind-spot | One anonymous struct parameter is one canonical type, so both data-clump and long-parameter-list go quiet while every caller now spells the same struct type out. Fixture: fixtures/clump. |
| pack-anonymous-struct | internal/columbo.inspectionSpan | 1 | blind-spot | One anonymous struct parameter is one canonical type, so both data-clump and long-parameter-list go quiet while every caller now spells the same struct type out. Fixture: fixtures/clump. |
| pack-anonymous-struct | internal/columbo.inspectionSpan | 1 | blind-spot | One anonymous struct parameter is one canonical type, so both data-clump and long-parameter-list go quiet while every caller now spells the same struct type out. Fixture: fixtures/clump. |
| pack-anonymous-struct | internal/columbo.selectedValue | 1 | blind-spot | One anonymous struct parameter is one canonical type, so both data-clump and long-parameter-list go quiet while every caller now spells the same struct type out. Fixture: fixtures/clump. |
| pack-anonymous-struct | internal/columbo.coordinationLink | 1 | blind-spot | One anonymous struct parameter is one canonical type, so both data-clump and long-parameter-list go quiet while every caller now spells the same struct type out. Fixture: fixtures/clump. |
| pack-anonymous-struct | internal/columbo.advisorySite | 1 | blind-spot | One anonymous struct parameter is one canonical type, so both data-clump and long-parameter-list go quiet while every caller now spells the same struct type out. Fixture: fixtures/clump. |
| pack-anonymous-struct | internal/columbo.Suppression | 1 | blind-spot | One anonymous struct parameter is one canonical type, so both data-clump and long-parameter-list go quiet while every caller now spells the same struct type out. Fixture: fixtures/clump. |
| pack-anonymous-struct | internal/columbo.categoryBranch | 1 | blind-spot | One anonymous struct parameter is one canonical type, so both data-clump and long-parameter-list go quiet while every caller now spells the same struct type out. Fixture: fixtures/clump. |
| pack-anonymous-struct | internal/columbo.categoryBranch | 1 | blind-spot | One anonymous struct parameter is one canonical type, so both data-clump and long-parameter-list go quiet while every caller now spells the same struct type out. Fixture: fixtures/clump. |
| pack-anonymous-struct | internal/columbo.Report | 1 | blind-spot | One anonymous struct parameter is one canonical type, so both data-clump and long-parameter-list go quiet while every caller now spells the same struct type out. Fixture: fixtures/clump. |
| pack-anonymous-struct | internal/columbo.Report | 1 | blind-spot | One anonymous struct parameter is one canonical type, so both data-clump and long-parameter-list go quiet while every caller now spells the same struct type out. Fixture: fixtures/clump. |
| pack-anonymous-struct | internal/columbo.Report | 1 | blind-spot | One anonymous struct parameter is one canonical type, so both data-clump and long-parameter-list go quiet while every caller now spells the same struct type out. Fixture: fixtures/clump. |
| pack-anonymous-struct | internal/columbo.roleGroup | 1 | blind-spot | One anonymous struct parameter is one canonical type, so both data-clump and long-parameter-list go quiet while every caller now spells the same struct type out. Fixture: fixtures/clump. |
| pack-anonymous-struct | internal/columbo.Member | 1 | blind-spot | One anonymous struct parameter is one canonical type, so both data-clump and long-parameter-list go quiet while every caller now spells the same struct type out. Fixture: fixtures/clump. |
| pack-anonymous-struct | github.com/spf13/cobra.Command | 1 | blind-spot | One anonymous struct parameter is one canonical type, so both data-clump and long-parameter-list go quiet while every caller now spells the same struct type out. Fixture: fixtures/clump. |
| pack-anonymous-struct | github.com/spf13/cobra.Command | 1 | blind-spot | One anonymous struct parameter is one canonical type, so both data-clump and long-parameter-list go quiet while every caller now spells the same struct type out. Fixture: fixtures/clump. |
| pack-anonymous-struct | github.com/spf13/cobra/doc.GenManTreeOptions | 1 | blind-spot | One anonymous struct parameter is one canonical type, so both data-clump and long-parameter-list go quiet while every caller now spells the same struct type out. Fixture: fixtures/clump. |
| pack-anonymous-struct | github.com/go-chi/chi/v5/middleware.ThrottleOpts | 1 | blind-spot | One anonymous struct parameter is one canonical type, so both data-clump and long-parameter-list go quiet while every caller now spells the same struct type out. Fixture: fixtures/clump. |
| pack-anonymous-struct | github.com/BurntSushi/toml.item | 1 | blind-spot | One anonymous struct parameter is one canonical type, so both data-clump and long-parameter-list go quiet while every caller now spells the same struct type out. Fixture: fixtures/clump. |
| pack-anonymous-struct | github.com/BurntSushi/toml.item | 1 | blind-spot | One anonymous struct parameter is one canonical type, so both data-clump and long-parameter-list go quiet while every caller now spells the same struct type out. Fixture: fixtures/clump. |
| pack-anonymous-struct | github.com/BurntSushi/toml.item | 1 | blind-spot | One anonymous struct parameter is one canonical type, so both data-clump and long-parameter-list go quiet while every caller now spells the same struct type out. Fixture: fixtures/clump. |
| pack-anonymous-struct | github.com/BurntSushi/toml.item | 1 | blind-spot | One anonymous struct parameter is one canonical type, so both data-clump and long-parameter-list go quiet while every caller now spells the same struct type out. Fixture: fixtures/clump. |
| pack-anonymous-struct-lpl | internal/columbo.coordinationDecision | 1 | blind-spot | Same as pack-anonymous-struct, measured against long-parameter-list. |
| pack-anonymous-struct-lpl | internal/columbo.RoleCandidate | 1 | blind-spot | Same as pack-anonymous-struct, measured against long-parameter-list. |
| pack-anonymous-struct-lpl | internal/columbo.helperPair | 1 | blind-spot | Same as pack-anonymous-struct, measured against long-parameter-list. |
| pack-anonymous-struct-lpl | internal/columbo.engine | 1 | blind-spot | Same as pack-anonymous-struct, measured against long-parameter-list. |
| pack-anonymous-struct-lpl | internal/columbo.Detail | 1 | blind-spot | Same as pack-anonymous-struct, measured against long-parameter-list. |
| pack-anonymous-struct-lpl | internal/columbo.selectionSite | 1 | blind-spot | Same as pack-anonymous-struct, measured against long-parameter-list. |
| pack-anonymous-struct-lpl | internal/columbo.stageDefinition | 1 | blind-spot | Same as pack-anonymous-struct, measured against long-parameter-list. |
| pack-anonymous-struct-lpl | internal/columbo.tangleComparison | 1 | blind-spot | Same as pack-anonymous-struct, measured against long-parameter-list. |
| pack-anonymous-struct-lpl | internal/columbo.file | 1 | blind-spot | Same as pack-anonymous-struct, measured against long-parameter-list. |
| pack-anonymous-struct-lpl | internal/columbo.inspectionSpan | 1 | blind-spot | Same as pack-anonymous-struct, measured against long-parameter-list. |
| pack-anonymous-struct-lpl | internal/columbo.overwriteCandidate | 1 | blind-spot | Same as pack-anonymous-struct, measured against long-parameter-list. |
| pack-anonymous-struct-lpl | internal/columbo.selectedUse | 1 | blind-spot | Same as pack-anonymous-struct, measured against long-parameter-list. |
| pack-anonymous-struct-lpl | internal/columbo.selectedValue | 1 | blind-spot | Same as pack-anonymous-struct, measured against long-parameter-list. |
| pack-anonymous-struct-lpl | internal/columbo.advisorySite | 1 | blind-spot | Same as pack-anonymous-struct, measured against long-parameter-list. |
| pack-anonymous-struct-lpl | internal/columbo.Suppression | 1 | blind-spot | Same as pack-anonymous-struct, measured against long-parameter-list. |
| pack-anonymous-struct-lpl | internal/columbo.categoryBranch | 1 | blind-spot | Same as pack-anonymous-struct, measured against long-parameter-list. |
| pack-anonymous-struct-lpl | internal/columbo.guardedMatch | 1 | blind-spot | Same as pack-anonymous-struct, measured against long-parameter-list. |
| pack-anonymous-struct-lpl | internal/columbo.Report | 1 | blind-spot | Same as pack-anonymous-struct, measured against long-parameter-list. |
| pack-anonymous-struct-lpl | internal/columbo.roleGroup | 1 | blind-spot | Same as pack-anonymous-struct, measured against long-parameter-list. |
| pack-anonymous-struct-lpl | internal/columbo.Member | 1 | blind-spot | Same as pack-anonymous-struct, measured against long-parameter-list. |
| pack-anonymous-struct-lpl | internal/columbo.Config | 1 | blind-spot | Same as pack-anonymous-struct, measured against long-parameter-list. |
| pack-anonymous-struct-lpl | internal/columbo.SelectionContext | 1 | blind-spot | Same as pack-anonymous-struct, measured against long-parameter-list. |
| pack-anonymous-struct-lpl | internal/columbo.Cluster | 1 | blind-spot | Same as pack-anonymous-struct, measured against long-parameter-list. |
| pack-anonymous-struct-lpl | internal/columbo.variantGroup | 1 | blind-spot | Same as pack-anonymous-struct, measured against long-parameter-list. |
| pack-anonymous-struct-lpl | internal/columbo.metricCheck | 1 | blind-spot | Same as pack-anonymous-struct, measured against long-parameter-list. |
| pack-anonymous-struct-lpl | github.com/spf13/cobra.Command | 1 | blind-spot | Same as pack-anonymous-struct, measured against long-parameter-list. |
| pack-anonymous-struct-lpl | github.com/spf13/cobra/doc.GenManHeader | 1 | blind-spot | Same as pack-anonymous-struct, measured against long-parameter-list. |
| pack-anonymous-struct-lpl | github.com/go-chi/chi/v5.Context | 1 | blind-spot | Same as pack-anonymous-struct, measured against long-parameter-list. |
| pack-anonymous-struct-lpl | github.com/go-chi/chi/v5/middleware.ThrottleOpts | 1 | blind-spot | Same as pack-anonymous-struct, measured against long-parameter-list. |
| pack-anonymous-struct-lpl | github.com/BurntSushi/toml.Position | 1 | blind-spot | Same as pack-anonymous-struct, measured against long-parameter-list. |
| pack-anonymous-struct-lpl | github.com/BurntSushi/toml.item | 1 | blind-spot | Same as pack-anonymous-struct, measured against long-parameter-list. |
| prose-to-string-constant | cmd/releasecheck.<package-clause> | 1 | blind-spot | docs/prose-comments.md tells agents not to move prose into unused strings, but no rule enforces it. Fixture: fixtures/prose. |
| prose-to-string-constant | cmd/columbo.<package-clause> | 1 | blind-spot | docs/prose-comments.md tells agents not to move prose into unused strings, but no rule enforces it. Fixture: fixtures/prose. |
| prose-to-string-constant | internal/columbo.<package-clause> | 1 | blind-spot | docs/prose-comments.md tells agents not to move prose into unused strings, but no rule enforces it. Fixture: fixtures/prose. |
| prose-to-string-constant | internal/columbo.<package-clause> | 1 | blind-spot | docs/prose-comments.md tells agents not to move prose into unused strings, but no rule enforces it. Fixture: fixtures/prose. |
| prose-to-string-constant | internal/columbo.<package-clause> | 1 | blind-spot | docs/prose-comments.md tells agents not to move prose into unused strings, but no rule enforces it. Fixture: fixtures/prose. |
| prose-to-string-constant | internal/columbo.<package-clause> | 1 | blind-spot | docs/prose-comments.md tells agents not to move prose into unused strings, but no rule enforces it. Fixture: fixtures/prose. |
| prose-to-string-constant | internal/columbo.<package-clause> | 1 | blind-spot | docs/prose-comments.md tells agents not to move prose into unused strings, but no rule enforces it. Fixture: fixtures/prose. |
| prose-to-string-constant | internal/columbo.<package-clause> | 1 | blind-spot | docs/prose-comments.md tells agents not to move prose into unused strings, but no rule enforces it. Fixture: fixtures/prose. |
| prose-to-string-constant | internal/columbo.<package-clause> | 1 | blind-spot | docs/prose-comments.md tells agents not to move prose into unused strings, but no rule enforces it. Fixture: fixtures/prose. |
| prose-to-string-constant | internal/columbo.<package-clause> | 1 | blind-spot | docs/prose-comments.md tells agents not to move prose into unused strings, but no rule enforces it. Fixture: fixtures/prose. |
| prose-to-string-constant | internal/columbo.<package-clause> | 1 | blind-spot | docs/prose-comments.md tells agents not to move prose into unused strings, but no rule enforces it. Fixture: fixtures/prose. |
| prose-to-string-constant | internal/columbo.<package-clause> | 1 | blind-spot | docs/prose-comments.md tells agents not to move prose into unused strings, but no rule enforces it. Fixture: fixtures/prose. |
| prose-to-string-constant | internal/columbo.<package-clause> | 1 | blind-spot | docs/prose-comments.md tells agents not to move prose into unused strings, but no rule enforces it. Fixture: fixtures/prose. |
| prose-to-string-constant | internal/columbo.<package-clause> | 1 | blind-spot | docs/prose-comments.md tells agents not to move prose into unused strings, but no rule enforces it. Fixture: fixtures/prose. |
| prose-to-string-constant | internal/columbo.<package-clause> | 1 | blind-spot | docs/prose-comments.md tells agents not to move prose into unused strings, but no rule enforces it. Fixture: fixtures/prose. |
| prose-to-string-constant | internal/columbo.<package-clause> | 1 | blind-spot | docs/prose-comments.md tells agents not to move prose into unused strings, but no rule enforces it. Fixture: fixtures/prose. |
| prose-to-string-constant | internal/columbo.<package-clause> | 1 | blind-spot | docs/prose-comments.md tells agents not to move prose into unused strings, but no rule enforces it. Fixture: fixtures/prose. |
| prose-to-string-constant | internal/columbo.<package-clause> | 1 | blind-spot | docs/prose-comments.md tells agents not to move prose into unused strings, but no rule enforces it. Fixture: fixtures/prose. |
| prose-to-string-constant | internal/columbo.<package-clause> | 1 | blind-spot | docs/prose-comments.md tells agents not to move prose into unused strings, but no rule enforces it. Fixture: fixtures/prose. |
| prose-to-string-constant | internal/columbo.<package-clause> | 1 | blind-spot | docs/prose-comments.md tells agents not to move prose into unused strings, but no rule enforces it. Fixture: fixtures/prose. |
| prose-to-string-constant | internal/columbo.<package-clause> | 1 | blind-spot | docs/prose-comments.md tells agents not to move prose into unused strings, but no rule enforces it. Fixture: fixtures/prose. |
| prose-to-string-constant | internal/columbo.<package-clause> | 1 | blind-spot | docs/prose-comments.md tells agents not to move prose into unused strings, but no rule enforces it. Fixture: fixtures/prose. |
| prose-to-string-constant | internal/columbo.<package-clause> | 1 | blind-spot | docs/prose-comments.md tells agents not to move prose into unused strings, but no rule enforces it. Fixture: fixtures/prose. |
| prose-to-string-constant | internal/columbo.<package-clause> | 1 | blind-spot | docs/prose-comments.md tells agents not to move prose into unused strings, but no rule enforces it. Fixture: fixtures/prose. |
| prose-to-string-constant | internal/columbo.<package-clause> | 1 | blind-spot | docs/prose-comments.md tells agents not to move prose into unused strings, but no rule enforces it. Fixture: fixtures/prose. |
| prose-to-string-constant | github.com/BurntSushi/toml/ossfuzz.<package-clause> | 1 | blind-spot | docs/prose-comments.md tells agents not to move prose into unused strings, but no rule enforces it. Fixture: fixtures/prose. |
| redteam-replay | github.com/spf13/cobra/doc.GenMarkdownTreeCustom | 1 | blind-spot | 13 of 14 frozen agent evasions silence their case without improving the design (see calibration/redteam/*/meta.yaml technique). Three techniques silence any rule: a hand-written "Code generated ... DO NOT EDIT." header hides a whole file; a function literal in a package-level var is never a finding subject; a //nolint: prefix turns prose into a directive. |
| redteam-replay | github.com/spf13/cobra.(*Command).InheritedFlags | 1 | blind-spot | 13 of 14 frozen agent evasions silence their case without improving the design (see calibration/redteam/*/meta.yaml technique). Three techniques silence any rule: a hand-written "Code generated ... DO NOT EDIT." header hides a whole file; a function literal in a package-level var is never a finding subject; a //nolint: prefix turns prose into a directive. |
| redteam-replay | github.com/spf13/cobra.defaultHelpFunc | 1 | blind-spot | 13 of 14 frozen agent evasions silence their case without improving the design (see calibration/redteam/*/meta.yaml technique). Three techniques silence any rule: a hand-written "Code generated ... DO NOT EDIT." header hides a whole file; a function literal in a package-level var is never a finding subject; a //nolint: prefix turns prose into a directive. |
| redteam-replay | github.com/spf13/cobra/doc.GenReSTTreeCustom | 1 | blind-spot | 13 of 14 frozen agent evasions silence their case without improving the design (see calibration/redteam/*/meta.yaml technique). Three techniques silence any rule: a hand-written "Code generated ... DO NOT EDIT." header hides a whole file; a function literal in a package-level var is never a finding subject; a //nolint: prefix turns prose into a directive. |
| redteam-replay | github.com/spf13/cobra.writePostscript | 1 | blind-spot | 13 of 14 frozen agent evasions silence their case without improving the design (see calibration/redteam/*/meta.yaml technique). Three techniques silence any rule: a hand-written "Code generated ... DO NOT EDIT." header hides a whole file; a function literal in a package-level var is never a finding subject; a //nolint: prefix turns prose into a directive. |
| redteam-replay | github.com/spf13/cobra.<package-clause> | 1 | blind-spot | 13 of 14 frozen agent evasions silence their case without improving the design (see calibration/redteam/*/meta.yaml technique). Three techniques silence any rule: a hand-written "Code generated ... DO NOT EDIT." header hides a whole file; a function literal in a package-level var is never a finding subject; a //nolint: prefix turns prose into a directive. |
| redteam-replay | github.com/spf13/cobra.Gt | 1 | blind-spot | 13 of 14 frozen agent evasions silence their case without improving the design (see calibration/redteam/*/meta.yaml technique). Three techniques silence any rule: a hand-written "Code generated ... DO NOT EDIT." header hides a whole file; a function literal in a package-level var is never a finding subject; a //nolint: prefix turns prose into a directive. |
| redteam-replay | github.com/go-chi/chi/v5.(*Mux).handle | 1 | blind-spot | 13 of 14 frozen agent evasions silence their case without improving the design (see calibration/redteam/*/meta.yaml technique). Three techniques silence any rule: a hand-written "Code generated ... DO NOT EDIT." header hides a whole file; a function literal in a package-level var is never a finding subject; a //nolint: prefix turns prose into a directive. |
| redteam-replay | github.com/go-chi/chi/v5/middleware.SupressNotFound | 1 | blind-spot | 13 of 14 frozen agent evasions silence their case without improving the design (see calibration/redteam/*/meta.yaml technique). Three techniques silence any rule: a hand-written "Code generated ... DO NOT EDIT." header hides a whole file; a function literal in a package-level var is never a finding subject; a //nolint: prefix turns prose into a directive. |
| redteam-replay | github.com/go-chi/chi/v5.(*node).FindRoute | 1 | blind-spot | 13 of 14 frozen agent evasions silence their case without improving the design (see calibration/redteam/*/meta.yaml technique). Three techniques silence any rule: a hand-written "Code generated ... DO NOT EDIT." header hides a whole file; a function literal in a package-level var is never a finding subject; a //nolint: prefix turns prose into a directive. |
| redteam-replay | github.com/go-chi/chi/v5/middleware.RedirectSlashes | 1 | blind-spot | 13 of 14 frozen agent evasions silence their case without improving the design (see calibration/redteam/*/meta.yaml technique). Three techniques silence any rule: a hand-written "Code generated ... DO NOT EDIT." header hides a whole file; a function literal in a package-level var is never a finding subject; a //nolint: prefix turns prose into a directive. |
| redteam-replay | github.com/go-chi/chi/v5/middleware.(*defaultLogEntry).Write | 1 | blind-spot | 13 of 14 frozen agent evasions silence their case without improving the design (see calibration/redteam/*/meta.yaml technique). Three techniques silence any rule: a hand-written "Code generated ... DO NOT EDIT." header hides a whole file; a function literal in a package-level var is never a finding subject; a //nolint: prefix turns prose into a directive. |
| redteam-replay | github.com/go-chi/chi/v5/middleware.<package-clause> | 1 | blind-spot | 13 of 14 frozen agent evasions silence their case without improving the design (see calibration/redteam/*/meta.yaml technique). Three techniques silence any rule: a hand-written "Code generated ... DO NOT EDIT." header hides a whole file; a function literal in a package-level var is never a finding subject; a //nolint: prefix turns prose into a directive. |
| route-through-facade | internal/columbo.(choiceObjects).selector | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | internal/columbo.(*declaration).selection | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | internal/columbo.declarationRefLess | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | internal/columbo.(*dependencyCollector).components | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | internal/columbo.(*typeNormalizer).channel | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | internal/columbo.(*callGraph).resolve | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | internal/columbo.(summaryMetric).write | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | internal/columbo.(*helperCluster).displayedOverlaps | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | internal/columbo.(guardedPrefix).otherStatement | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | internal/columbo.normalizedField | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | internal/columbo.(*RoleCandidate).classify | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | internal/columbo.(*snapshotWriter).advisoryCoverage | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | internal/columbo.(*selectionScan).conversion | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | internal/columbo.selectionEvidence | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | internal/columbo.(*inspectionFile).classifyInspectionSource | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | internal/columbo.(*declaration).declReceipt | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | internal/columbo.configDocument | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | internal/columbo.(*complexityVisitor).markCalculated | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | internal/columbo.(inspectionInput).analyze | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | internal/columbo.(*complexityVisitor).visitFuncLit | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | internal/columbo.(*guardedNormalizer).namedConstant | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | internal/columbo.(*cosmeticInvestigation).expand | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | internal/columbo.(*RoleCandidate).merge | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | internal/columbo.(metricCheck).explain | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | internal/columbo.(*selectionScan).ifStatement | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/spf13/cobra.(*Command).CommandPathPadding | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/spf13/cobra.(*Command).GenPowerShellCompletionWithDesc | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/spf13/cobra.(*Command).SetOutput | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/spf13/cobra.(*Command).updateParentsPflags | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/spf13/cobra.RangeArgs | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/spf13/cobra.AddTemplateFuncs | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/spf13/cobra.(*Command).InitDefaultHelpCmd | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/spf13/cobra.(*Command).HasAvailableInheritedFlags | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/spf13/cobra/doc.manPreamble | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/spf13/cobra/doc.printOptions | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/spf13/cobra/doc.printOptionsReST | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/spf13/cobra.(*Command).GenZshCompletionNoDesc | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/spf13/cobra.(*Command).ExecuteContext | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/spf13/cobra.ExactValidArgs | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/spf13/cobra.(*Command).HasLocalFlags | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/spf13/cobra.(*Command).GenZshCompletion | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/spf13/cobra.nonCompletableFlag | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/spf13/cobra.(*Command).VersionTemplate | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/spf13/cobra.(*Command).CalledAs | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/spf13/cobra.(*Command).DisplayName | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/spf13/cobra.commandNameMatches | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/spf13/cobra.(*Command).Find | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/spf13/cobra.Eq | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/spf13/cobra.(*Command).AddCommand | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/spf13/cobra.(*Command).Help | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/go-chi/chi/v5.methodTypString | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/go-chi/chi/v5/middleware.cW | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/go-chi/chi/v5.setPathValue | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/go-chi/chi/v5/middleware.(HeaderRouter).RouteAny | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/go-chi/chi/v5.(*Mux).Delete | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/go-chi/chi/v5/middleware.Maybe | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/go-chi/chi/v5/middleware.(*flushHijackWriter).Flush | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/go-chi/chi/v5/middleware.(HeaderRoute).IsMatch | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/go-chi/chi/v5.(*node).getEdge | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/go-chi/chi/v5/middleware.(*http2FancyWriter).Flush | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/go-chi/chi/v5/middleware.Compress | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/go-chi/chi/v5.NewRouter | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/go-chi/chi/v5.(nodes).Sort | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/go-chi/chi/v5.(*Mux).Connect | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/go-chi/chi/v5.URLParam | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/go-chi/chi/v5.(*Mux).NotFoundHandler | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/go-chi/chi/v5/middleware.(*compressResponseWriter).Write | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/go-chi/chi/v5/middleware.RequestSize | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/go-chi/chi/v5/middleware.(*basicWriter).maybeWriteHeader | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/go-chi/chi/v5/middleware.encoderDeflate | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/go-chi/chi/v5/middleware.Throttle | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/go-chi/chi/v5.Chain | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/go-chi/chi/v5/middleware.(*httpFancyWriter).ReadFrom | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/go-chi/chi/v5.methodNotAllowedHandler | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/go-chi/chi/v5.(*Mux).Head | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/BurntSushi/toml.lexRawString | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/BurntSushi/toml/internal/toml-test.(Test).cmpTOMLArrays | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/BurntSushi/toml.lexLongUnicodeEscape | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/BurntSushi/toml.(*parser).valueInteger | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/BurntSushi/toml.(*parser).typeOfPrimitive | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/BurntSushi/toml/cmd/toml-test-decoder.init#1 | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/BurntSushi/toml.(*parser).asciiEscapeToUnicode | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/BurntSushi/toml.lexTableStart | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/BurntSushi/toml.runeOrEOF | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/BurntSushi/toml.(*lexer).errorf | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/BurntSushi/toml.lexBareName | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/BurntSushi/toml.isDigit | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/BurntSushi/toml/internal/toml-test.(Runner).List | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/BurntSushi/toml/cmd/toml-test-encoder.init#1 | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/BurntSushi/toml.(*lexer).peek | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/BurntSushi/toml.lexTableEnd | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/BurntSushi/toml.(*lexer).ignore | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/BurntSushi/toml/internal/toml-test.(Test).mismatch | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/BurntSushi/toml.(*lexer).errorPrevLine | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/BurntSushi/toml.(*MetaData).unifyAnything | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/BurntSushi/toml.(*lexer).accept | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/BurntSushi/toml.(tomlBaseType).String | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/BurntSushi/toml.(*parser).addImplicitContext | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/BurntSushi/toml.lexInlineTableValue | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/BurntSushi/toml.(*lexer).pop | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/google/uuid.(*NullUUID).UnmarshalBinary | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/google/uuid.NewMD5 | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/google/uuid.getV7Time | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/google/uuid.makeV7 | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/google/uuid.(invalidLengthError).Error | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/google/uuid.(Domain).String | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/google/uuid.SetRand | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/google/uuid.EnableRandPool | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/google/uuid.(UUID).ClockSequence | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/google/uuid.(UUID).ID | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/google/uuid.NewV7FromReader | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/google/uuid.New | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/google/uuid.Must | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/google/uuid.IsInvalidLengthError | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/google/uuid.(NullUUID).MarshalText | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/google/uuid.(UUIDs).Strings | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/google/uuid.(NullUUID).MarshalJSON | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/google/uuid.Parse | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/google/uuid.FromBytes | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/google/uuid.NewDCEGroup | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/google/uuid.NewSHA1 | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/google/uuid.(UUID).Variant | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/google/uuid.SetNodeID | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/google/uuid.(Time).UnixTime | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/google/uuid.getHardwareInterface | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| split-forwarding-helpers | internal/columbo.(guardedUpdateKey).group | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-forwarding-helpers | internal/columbo.(guardedUpdateKey).group | 4 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-forwarding-helpers | internal/columbo.(RoleCandidate).writeSnapshot | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-forwarding-helpers | internal/columbo.(RoleCandidate).writeSnapshot | 4 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-forwarding-helpers | internal/columbo.(RoleCandidate).writeSnapshot | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-forwarding-helpers | internal/columbo.(RoleCandidate).writeSnapshot | 4 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-forwarding-helpers | internal/columbo.(RoleCandidate).writeSnapshot | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-forwarding-helpers | internal/columbo.(RoleCandidate).writeSnapshot | 4 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-forwarding-helpers | internal/columbo.(RoleCandidate).writeSnapshot | 4 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-forwarding-helpers | internal/columbo.(guardedPrefix).preservesUntil | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-forwarding-helpers | internal/columbo.(guardedPrefix).preservesUntil | 4 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-forwarding-helpers | github.com/spf13/cobra.writeCommands | 2 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-forwarding-helpers | github.com/spf13/cobra.writeLocalNonPersistentFlag | 2 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-forwarding-helpers | github.com/spf13/cobra.writeRequiredNouns | 2 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-forwarding-helpers | github.com/spf13/cobra.writeRequiredNouns | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-forwarding-helpers | github.com/spf13/cobra.writeRequiredNouns | 4 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-forwarding-helpers | github.com/go-chi/chi/v5/middleware.init#1 | 4 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-forwarding-helpers | github.com/go-chi/chi/v5/middleware.init#1 | 4 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-forwarding-helpers | github.com/go-chi/chi/v5/middleware.init#1 | 4 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-forwarding-helpers | github.com/google/uuid.(UUID).URN | 4 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-forwarding-helpers-complexity | github.com/spf13/cobra.gen | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-forwarding-helpers-complexity | github.com/spf13/cobra.gen | 4 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | internal/columbo.coordinationIsolatedSlot | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | internal/columbo.coordinationNoBypass | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | internal/columbo.coordinationExitUnknown | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | internal/columbo.(*engine).inspectionFacts | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | internal/columbo.(*engine).inspectionFiles | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | internal/columbo.(*engine).inspectionFiles | 4 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | internal/columbo.(*engine).inspectionCall | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | internal/columbo.(*file).inspectionType | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | internal/columbo.(*file).inspectionType | 4 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/spf13/cobra.writePreamble | 2 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/spf13/cobra.writeFlags | 2 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/spf13/cobra.writeFlags | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/spf13/cobra.writeFlags | 4 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/spf13/cobra.writeRequiredFlag | 2 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/spf13/cobra.writeRequiredFlag | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/spf13/cobra.gen | 2 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/spf13/cobra.gen | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/spf13/cobra.gen | 4 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/spf13/cobra.genBashComp | 2 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/spf13/cobra.genBashComp | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/spf13/cobra.Gt | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/spf13/cobra.Gt | 4 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/spf13/cobra.(*Command).Find | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/spf13/cobra.(*Command).Find | 4 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/spf13/cobra.(*Command).Traverse | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/spf13/cobra.(*Command).SuggestionsFor | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/spf13/cobra.(*Command).UseLine | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/spf13/cobra.(*Command).DebugFlags | 2 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/spf13/cobra.(*Command).DebugFlags | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/spf13/cobra.(*Command).DebugFlags | 4 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/spf13/cobra.defaultUsageFunc | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/spf13/cobra.defaultUsageFunc | 4 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/spf13/cobra.defaultHelpFunc | 4 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/spf13/cobra.(ShellCompDirective).string | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/spf13/cobra.(ShellCompDirective).string | 4 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/spf13/cobra.(*Command).initCompleteCmd | 2 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/spf13/cobra.(*Command).initCompleteCmd | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/spf13/cobra.(*Command).initCompleteCmd | 4 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/spf13/cobra.completeRequireFlags | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/spf13/cobra.completeRequireFlags | 4 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/spf13/cobra/doc.GenYamlCustom | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/spf13/cobra/doc.genFlagResult | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/spf13/cobra.genFishComp | 2 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/spf13/cobra.genFishComp | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/spf13/cobra.genFishComp | 4 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/spf13/cobra.genPowerShellComp | 2 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/spf13/cobra.genPowerShellComp | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/spf13/cobra.genPowerShellComp | 4 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/spf13/cobra.genZshComp | 2 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/spf13/cobra.genZshComp | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/go-chi/chi/v5/middleware.(*Compressor).selectEncoder | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/go-chi/chi/v5/middleware.(*DefaultLogFormatter).NewLogEntry | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/go-chi/chi/v5/middleware.(*DefaultLogFormatter).NewLogEntry | 4 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/go-chi/chi/v5/middleware.Profiler | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/go-chi/chi/v5/middleware.Profiler | 4 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/go-chi/chi/v5/middleware.realIP | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/go-chi/chi/v5/middleware.(prettyStack).parse | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/go-chi/chi/v5/middleware.(prettyStack).parse | 4 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/go-chi/chi/v5/middleware.ThrottleWithOpts | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/go-chi/chi/v5/middleware.ThrottleWithOpts | 4 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/go-chi/chi/v5/middleware.NewWrapResponseWriter | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/go-chi/chi/v5.(*Mux).routeHTTP | 2 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/go-chi/chi/v5.(*Mux).routeHTTP | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/go-chi/chi/v5.(*Mux).routeHTTP | 4 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/go-chi/chi/v5.(*node).findEdge | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/go-chi/chi/v5.(*node).findEdge | 4 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/go-chi/chi/v5.(*node).routes | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/go-chi/chi/v5.patParamKeys | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/go-chi/chi/v5.(nodes).findEdge | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/go-chi/chi/v5.(nodes).findEdge | 4 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/BurntSushi/toml/cmd/tomlv.main | 2 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/BurntSushi/toml.(*Encoder).eMap | 2 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/BurntSushi/toml.(*Encoder).eMap | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/BurntSushi/toml.(*Encoder).eMap | 4 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/BurntSushi/toml.(*Encoder).eStruct | 2 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/BurntSushi/toml.(*Encoder).eStruct | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/BurntSushi/toml.(*Encoder).eStruct | 4 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/BurntSushi/toml/internal/tag.untag | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/BurntSushi/toml/internal/toml-test.(Test).cmpAsDatetimes | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/BurntSushi/toml/internal/toml-test.deepEqual | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/BurntSushi/toml.lexBool | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/BurntSushi/toml.lexBool | 4 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/BurntSushi/toml.(*parser).next | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/BurntSushi/toml.(*parser).valueInteger | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/BurntSushi/toml.(*parser).valueInteger | 4 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/BurntSushi/toml.(*parser).valueFloat | 4 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/BurntSushi/toml.typeFields | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/BurntSushi/toml.typeFields | 4 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/google/uuid.(UUID).Time | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |

## Untriaged survivors

Inject and evade survivors the operator's prediction cannot explain and calibration/triage.yaml does not cover. A surviving cure only means the cure did not get under the limit, so it is not a Columbo blind spot and is not listed. CI warns on every row here.

None.
