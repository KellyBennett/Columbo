# Calibration report

## Runs

The latest run per corpus module, and how many of its mutants are current. Each operator's numbers come from the newest run that measured it on that corpus.

| id | corpus_module | corpus_version | columbo_version | config_hash | validity | baseline_findings | mutants |
|---|---|---|---|---|---|---|---|
| 4 | github.com/BurntSushi/toml | v1.5.0 | dev+sha256:fed57a1047b131e1 | default | build | 191 | 439 |
| 1 | github.com/KellyBennett/Columbo | 6440c9d | dev+sha256:fed57a1047b131e1 | 1c5272bd253a7d8f | build | 53 | 609 |
| 3 | github.com/go-chi/chi/v5 | v5.2.1 | dev+sha256:fed57a1047b131e1 | default | build | 133 | 423 |
| 5 | github.com/google/uuid | v1.6.0 | dev+sha256:fed57a1047b131e1 | default | build | 31 | 241 |
| 2 | github.com/spf13/cobra | v1.9.1 | dev+sha256:fed57a1047b131e1 | default | build | 136 | 602 |

## Outcomes by operator

Invalid mutants failed vet or tests and are excluded from every rate.

| class | operator | target | killed | survived | equivalent | conflict | id_drift | invalid | kill_rate |
|---|---|---|---|---|---|---|---|---|---|
| cure | delete-prose-comments | prose-comment | 72 | 6 | 0 | 0 | 0 | 0 | 92% |
| cure | extract-function | long-function | 50 | 132 | 0 | 2 | 0 | 0 | 27% |
| cure | extract-function-complexity | high-cognitive-complexity | 36 | 124 | 0 | 5 | 0 | 0 | 22% |
| cure | introduce-parameter-object | data-clump | 2 | 2 | 0 | 3 | 0 | 0 | 29% |
| cure | move-function | feature-envy | 9 | 0 | 0 | 0 | 0 | 0 | 100% |
| cure | split-by-collaborators | excessive-dependencies | 4 | 0 | 0 | 1 | 0 | 1 | 80% |
| evade | copy-fields-to-locals | feature-envy | 17 | 0 | 0 | 0 | 0 | 0 | 100% |
| evade | decision-to-map-lookup | repeated-variant-decision | 0 | 14 | 0 | 0 | 0 | 0 | 0% |
| evade | hide-behind-getters | feature-envy | 17 | 0 | 0 | 0 | 0 | 0 | 100% |
| evade | isolate-helper-calls | cosmetic-extraction | 55 | 0 | 0 | 0 | 0 | 0 | 100% |
| evade | pack-anonymous-struct | data-clump | 0 | 23 | 0 | 0 | 0 | 0 | 0% |
| evade | pack-anonymous-struct-lpl | long-parameter-list | 0 | 31 | 0 | 0 | 0 | 0 | 0% |
| evade | prose-to-string-constant | prose-comment | 0 | 26 | 0 | 0 | 0 | 0 | 0% |
| evade | redteam-replay | data-clump | 1 | 1 | 0 | 0 | 0 | 0 | 50% |
| evade | redteam-replay | excessive-dependencies | 0 | 2 | 0 | 0 | 0 | 0 | 0% |
| evade | redteam-replay | feature-envy | 0 | 2 | 0 | 0 | 0 | 0 | 0% |
| evade | redteam-replay | high-cognitive-complexity | 1 | 1 | 0 | 0 | 0 | 0 | 50% |
| evade | redteam-replay | long-function | 1 | 1 | 0 | 0 | 0 | 0 | 50% |
| evade | redteam-replay | long-parameter-list | 0 | 1 | 0 | 0 | 0 | 0 | 0% |
| evade | redteam-replay | prose-comment | 1 | 1 | 0 | 0 | 0 | 0 | 50% |
| evade | redteam-replay | repeated-variant-decision | 0 | 1 | 0 | 0 | 0 | 0 | 0% |
| evade | rename-and-relabel | duplicate-code | 47 | 0 | 0 | 0 | 0 | 1 | 100% |
| evade | reorder-params | data-clump | 23 | 0 | 0 | 0 | 0 | 0 | 100% |
| evade | route-through-facade | excessive-dependencies | 0 | 125 | 0 | 0 | 0 | 0 | 0% |
| evade | split-forwarding-helpers | long-function | 58 | 16 | 0 | 0 | 0 | 0 | 78% |
| evade | split-forwarding-helpers-complexity | high-cognitive-complexity | 15 | 0 | 0 | 0 | 0 | 0 | 100% |
| inject | add-collaborators | excessive-dependencies | 125 | 0 | 121 | 0 | 0 | 0 | 100% |
| inject | add-prose-comment | prose-comment | 26 | 0 | 0 | 0 | 0 | 0 | 100% |
| inject | duplicate-block | duplicate-code | 48 | 0 | 77 | 0 | 0 | 0 | 100% |
| inject | inline-function | long-function | 44 | 0 | 52 | 0 | 0 | 2 | 100% |
| inject | inline-function-complexity | high-cognitive-complexity | 10 | 0 | 102 | 0 | 0 | 2 | 100% |
| inject | inline-parameter-object | data-clump | 23 | 1 | 37 | 0 | 0 | 0 | 96% |
| inject | inline-parameter-object-lpl | long-parameter-list | 31 | 0 | 0 | 0 | 0 | 0 | 100% |
| inject | move-method-to-collaborator | feature-envy | 25 | 0 | 20 | 0 | 0 | 1 | 100% |
| inject | move-method-to-function | feature-envy | 55 | 0 | 16 | 0 | 0 | 0 | 100% |
| inject | repeat-variant-decision | repeated-variant-decision | 12 | 0 | 9 | 0 | 0 | 0 | 100% |
| inject | split-long-function | cosmetic-extraction | 52 | 57 | 0 | 0 | 0 | 0 | 48% |
| neutral | move-decl-to-file |  | 0 | 0 | 106 | 0 | 0 | 0 |  |
| neutral | noop |  | 0 | 0 | 5 | 0 | 0 | 0 |  |
| neutral | rename-decl |  | 0 | 0 | 110 | 0 | 0 | 0 |  |
| neutral | rename-local |  | 0 | 0 | 124 | 0 | 0 | 0 |  |
| neutral | reorder-decls |  | 0 | 0 | 88 | 0 | 2 | 0 |  |

## Outcomes by corpus

| corpus_module | class | killed | survived | equivalent | conflict | id_drift | invalid |
|---|---|---|---|---|---|---|---|
| github.com/BurntSushi/toml | cure | 36 | 81 | 0 | 2 | 0 | 1 |
| github.com/BurntSushi/toml | evade | 30 | 33 | 0 | 0 | 0 | 0 |
| github.com/BurntSushi/toml | inject | 88 | 14 | 61 | 0 | 0 | 0 |
| github.com/BurntSushi/toml | neutral | 0 | 0 | 92 | 0 | 1 | 0 |
| github.com/KellyBennett/Columbo | cure | 14 | 16 | 0 | 0 | 0 | 0 |
| github.com/KellyBennett/Columbo | evade | 57 | 110 | 0 | 0 | 0 | 0 |
| github.com/KellyBennett/Columbo | inject | 153 | 9 | 145 | 0 | 0 | 4 |
| github.com/KellyBennett/Columbo | neutral | 0 | 0 | 101 | 0 | 0 | 0 |
| github.com/go-chi/chi/v5 | cure | 54 | 77 | 0 | 6 | 0 | 0 |
| github.com/go-chi/chi/v5 | evade | 29 | 34 | 0 | 0 | 0 | 0 |
| github.com/go-chi/chi/v5 | inject | 62 | 11 | 54 | 0 | 0 | 1 |
| github.com/go-chi/chi/v5 | neutral | 0 | 0 | 95 | 0 | 0 | 0 |
| github.com/google/uuid | cure | 20 | 10 | 0 | 0 | 0 | 0 |
| github.com/google/uuid | evade | 11 | 31 | 0 | 0 | 0 | 1 |
| github.com/google/uuid | inject | 43 | 1 | 71 | 0 | 0 | 0 |
| github.com/google/uuid | neutral | 0 | 0 | 53 | 0 | 0 | 0 |
| github.com/spf13/cobra | cure | 49 | 80 | 0 | 3 | 0 | 0 |
| github.com/spf13/cobra | evade | 109 | 37 | 0 | 0 | 0 | 0 |
| github.com/spf13/cobra | inject | 105 | 23 | 103 | 0 | 0 | 0 |
| github.com/spf13/cobra | neutral | 0 | 0 | 92 | 0 | 1 | 0 |

## Kill matrix

Valid inject and evade mutants, by operator and by every smell they added anywhere. Read across a row to see which rules notice an operator; down a column to see what a rule notices.

| operator | smell | mutants |
|---|---|---|
| add-collaborators | long-function | 191 |
| add-collaborators | excessive-dependencies | 125 |
| add-prose-comment | prose-comment | 26 |
| copy-fields-to-locals | feature-envy | 17 |
| copy-fields-to-locals | long-function | 9 |
| copy-fields-to-locals | high-cognitive-complexity | 4 |
| duplicate-block | duplicate-code | 48 |
| duplicate-block | long-function | 19 |
| duplicate-block | high-cognitive-complexity | 10 |
| duplicate-block | feature-envy | 6 |
| duplicate-block | data-clump | 6 |
| duplicate-block | excessive-dependencies | 2 |
| duplicate-block | repeated-variant-decision | 1 |
| duplicate-block | long-parameter-list | 1 |
| hide-behind-getters | feature-envy | 17 |
| hide-behind-getters | long-function | 5 |
| hide-behind-getters | high-cognitive-complexity | 4 |
| inline-function | long-function | 44 |
| inline-function | duplicate-code | 9 |
| inline-function | high-cognitive-complexity | 7 |
| inline-function | excessive-dependencies | 7 |
| inline-function | feature-envy | 1 |
| inline-function-complexity | long-function | 35 |
| inline-function-complexity | high-cognitive-complexity | 10 |
| inline-function-complexity | duplicate-code | 10 |
| inline-function-complexity | feature-envy | 7 |
| inline-function-complexity | excessive-dependencies | 7 |
| inline-parameter-object | long-parameter-list | 50 |
| inline-parameter-object | feature-envy | 29 |
| inline-parameter-object | data-clump | 23 |
| inline-parameter-object | excessive-dependencies | 12 |
| inline-parameter-object | duplicate-code | 6 |
| inline-parameter-object | long-function | 3 |
| inline-parameter-object | cosmetic-extraction | 3 |
| inline-parameter-object-lpl | long-parameter-list | 31 |
| inline-parameter-object-lpl | feature-envy | 12 |
| inline-parameter-object-lpl | data-clump | 6 |
| inline-parameter-object-lpl | excessive-dependencies | 5 |
| isolate-helper-calls | cosmetic-extraction | 55 |
| isolate-helper-calls | long-function | 34 |
| isolate-helper-calls | high-cognitive-complexity | 20 |
| isolate-helper-calls | data-clump | 14 |
| isolate-helper-calls | long-parameter-list | 10 |
| isolate-helper-calls | feature-envy | 9 |
| isolate-helper-calls | excessive-dependencies | 2 |
| move-method-to-collaborator | feature-envy | 25 |
| move-method-to-function | feature-envy | 55 |
| move-method-to-function | excessive-dependencies | 1 |
| move-method-to-function | duplicate-code | 1 |
| pack-anonymous-struct | long-function | 20 |
| pack-anonymous-struct | feature-envy | 19 |
| pack-anonymous-struct | duplicate-code | 18 |
| pack-anonymous-struct | excessive-dependencies | 9 |
| pack-anonymous-struct-lpl | long-function | 19 |
| pack-anonymous-struct-lpl | feature-envy | 12 |
| pack-anonymous-struct-lpl | duplicate-code | 8 |
| pack-anonymous-struct-lpl | excessive-dependencies | 5 |
| redteam-replay | long-function | 2 |
| redteam-replay | high-cognitive-complexity | 1 |
| rename-and-relabel | duplicate-code | 47 |
| rename-and-relabel | long-function | 19 |
| rename-and-relabel | high-cognitive-complexity | 10 |
| rename-and-relabel | feature-envy | 6 |
| rename-and-relabel | excessive-dependencies | 2 |
| rename-and-relabel | long-parameter-list | 1 |
| reorder-params | data-clump | 23 |
| reorder-params | long-parameter-list | 22 |
| reorder-params | feature-envy | 19 |
| reorder-params | excessive-dependencies | 9 |
| reorder-params | duplicate-code | 6 |
| reorder-params | long-function | 3 |
| repeat-variant-decision | repeated-variant-decision | 12 |
| route-through-facade | feature-envy | 78 |
| route-through-facade | long-function | 49 |
| split-forwarding-helpers | cosmetic-extraction | 41 |
| split-forwarding-helpers | long-function | 39 |
| split-forwarding-helpers | duplicate-code | 28 |
| split-forwarding-helpers | long-parameter-list | 20 |
| split-forwarding-helpers | data-clump | 17 |
| split-forwarding-helpers | high-cognitive-complexity | 12 |
| split-forwarding-helpers | excessive-dependencies | 9 |
| split-forwarding-helpers-complexity | long-function | 15 |
| split-forwarding-helpers-complexity | cosmetic-extraction | 14 |
| split-forwarding-helpers-complexity | high-cognitive-complexity | 12 |
| split-forwarding-helpers-complexity | duplicate-code | 12 |
| split-forwarding-helpers-complexity | data-clump | 5 |
| split-forwarding-helpers-complexity | long-parameter-list | 3 |
| split-forwarding-helpers-complexity | feature-envy | 1 |
| split-long-function | long-function | 70 |
| split-long-function | cosmetic-extraction | 52 |
| split-long-function | high-cognitive-complexity | 39 |
| split-long-function | long-parameter-list | 31 |
| split-long-function | data-clump | 19 |
| split-long-function | feature-envy | 11 |
| split-long-function | excessive-dependencies | 6 |
| split-long-function | repeated-variant-decision | 2 |
| split-long-function | duplicate-code | 2 |

## Operator families no rule notices

Valid inject or evade operators whose mutants never added any finding. Each is a candidate missing rule.

| operator | valid_mutants |
|---|---|
| decision-to-map-lookup | 14 |
| prose-to-string-constant | 26 |

## Redundant rules

A smell whose every killing mutant also added another given smell. The second rule already notices everything the first does on this corpus.

None.

## Dose curves

For each inject operator and site, the first dose that killed. A site with no killing dose is counted under never.

| operator | first_killing_dose | sites |
|---|---|---|
| add-collaborators | 1 | 4 |
| add-collaborators | 2 | 1 |
| add-collaborators | 3 | 5 |
| add-collaborators | 4 | 11 |
| add-collaborators | 5 | 16 |
| add-collaborators | 6 | 23 |
| add-collaborators | 7 | 39 |
| add-collaborators | 8 | 26 |
| add-prose-comment | 1 | 4 |
| duplicate-block | never | 77 |
| duplicate-block | 1 | 48 |
| inline-function | never | 41 |
| inline-function | 1 | 24 |
| inline-function | 2 | 3 |
| inline-function | 3 | 3 |
| inline-function-complexity | never | 71 |
| inline-function-complexity | 1 | 6 |
| inline-function-complexity | 3 | 1 |
| inline-function-complexity | 7 | 1 |
| inline-parameter-object | never | 20 |
| inline-parameter-object | 1 | 4 |
| inline-parameter-object | 2 | 8 |
| inline-parameter-object-lpl | 1 | 31 |
| move-method-to-collaborator | never | 19 |
| move-method-to-collaborator | 1 | 20 |
| move-method-to-collaborator | 2 | 1 |
| move-method-to-collaborator | 3 | 2 |
| move-method-to-function | never | 16 |
| move-method-to-function | 1 | 55 |
| repeat-variant-decision | 2 | 9 |
| split-long-function | never | 29 |
| split-long-function | 2 | 6 |
| split-long-function | 3 | 24 |
| split-long-function | 4 | 1 |

## Prediction agreement

Inject and cure mutants whose operator predicted the outcome. Disagreements are either harness metric bugs or Columbo behavior worth a look.

| operator | predicted | predicted_kill_but_survived | predicted_survive_but_killed |
|---|---|---|---|
| add-collaborators | 246 | 0 | 0 |
| add-prose-comment | 26 | 0 | 0 |
| delete-prose-comments | 0 | 0 | 0 |
| duplicate-block | 125 | 0 | 0 |
| extract-function | 184 | 0 | 1 |
| extract-function-complexity | 165 | 0 | 0 |
| inline-function | 96 | 0 | 1 |
| inline-function-complexity | 112 | 0 | 0 |
| inline-parameter-object | 61 | 1 | 0 |
| inline-parameter-object-lpl | 31 | 0 | 0 |
| introduce-parameter-object | 0 | 0 | 0 |
| move-function | 9 | 0 | 0 |
| move-method-to-collaborator | 45 | 0 | 0 |
| move-method-to-function | 71 | 0 | 0 |
| repeat-variant-decision | 21 | 0 | 0 |
| split-by-collaborators | 5 | 0 | 0 |
| split-long-function | 0 | 0 | 0 |

## Cure conflicts

A cure that cleared its target but set off another rule. If a proper refactoring trips a different rule, that code has no green path.

| operator | site_symbol | dose | size | reason |
|---|---|---|---|---|
| extract-function | github.com/go-chi/chi/v5.(*node).setEndpoint | 3 | 245 | cleared target but added data-clump@v5.endpoint |
| extract-function | github.com/go-chi/chi/v5.(nodes).findEdge | 3 | 188 | cleared target but added long-parameter-list@v5.(nodes).findEdgePart2 |
| extract-function-complexity | github.com/BurntSushi/toml.(*MetaData).unify | 3 | 482 | cleared target but added long-function@toml.(*MetaData).unifyPart3; moved repeated-variant-decision@toml.(*MetaData).unifyPart3 |
| extract-function-complexity | github.com/BurntSushi/toml.(*parser).setValue | 3 | 235 | cleared target but added long-parameter-list@toml.(*parser).setValuePart2 |
| extract-function-complexity | github.com/go-chi/chi/v5.(*node).findEdge | 2 | 141 | cleared target but added long-parameter-list@v5.findEdgePart2 |
| extract-function-complexity | github.com/go-chi/chi/v5/middleware.ThrottleWithOpts | 3 | 174 | cleared target but added cosmetic-extraction@middleware.ThrottleWithOpts |
| extract-function-complexity | github.com/spf13/cobra.(*Command).SuggestionsFor | 2 | 189 | cleared target but added cosmetic-extraction@cobra.(*Command).SuggestionsFor |
| introduce-parameter-object | github.com/go-chi/chi/v5/middleware.(prettyStack).decorateLine | 1 | 108 | cleared target but added feature-envy@middleware.(prettyStack).decorateFuncCallLine, feature-envy@middleware.(prettyStack).decorateLine, feature-envy@middleware.(prettyStack).decorateSourceLine |
| introduce-parameter-object | github.com/spf13/cobra.genBashComp | 1 | 132 | cleared target but added duplicate-code@cobra.(*Command).genBashCompletion, feature-envy@cobra.genFishComp |
| introduce-parameter-object | github.com/spf13/cobra/doc.GenMarkdownTreeCustom | 1 | 196 | cleared target but added duplicate-code@doc.GenMarkdownTreeCustom, feature-envy@doc.GenMarkdownTreeCustom, feature-envy@doc.GenReSTTreeCustom, feature-envy@doc.GenYamlTreeCustom |
| split-by-collaborators | github.com/go-chi/chi/v5.(*Mux).Mount | 1 | 303 | cleared target but added long-function@v5.(*Mux).mountPart |

## Conflicting rules

Which rules the conflicts set off, per cure operator.

| operator | smell | added |
|---|---|---|
| extract-function | long-parameter-list | 1 |
| extract-function | data-clump | 1 |
| extract-function-complexity | long-parameter-list | 2 |
| extract-function-complexity | cosmetic-extraction | 2 |
| extract-function-complexity | repeated-variant-decision | 1 |
| extract-function-complexity | long-function | 1 |
| introduce-parameter-object | feature-envy | 7 |
| introduce-parameter-object | duplicate-code | 2 |
| split-by-collaborators | long-function | 1 |

## Cost asymmetry

Per smell: the smallest surviving evade against the smallest successful cure, in changed AST nodes. The README promises a violation is harder to silence than to properly refactor; that holds where the evade is larger.

| smell | smallest_surviving_evade | surviving_evades | smallest_green_cure | green_cures | promise |
|---|---|---|---|---|---|
| data-clump | 18 | 24 | 126 | 2 | fails |
| excessive-dependencies | 41 | 127 | 57 | 4 | fails |
| feature-envy | 9 | 2 | 128 | 9 | fails |
| high-cognitive-complexity | 49 | 1 | 26 | 36 | holds |
| long-function | 12 | 17 | 41 | 50 | fails |
| long-parameter-list | 53 | 32 | no cure reached green | 0 | fails (no green cure) |
| prose-comment | 1 | 27 | 1 | 72 | fails |
| repeated-variant-decision | 38 | 15 | no cure reached green | 0 | fails (no green cure) |

## Neutral drift

| operator | site_symbol | reason |
|---|---|---|
| reorder-decls | flag_groups.go | unexplained duplicate-code C-c912a2e518 on cobra.(*Command).MarkFlagsMutuallyExclusive |
| reorder-decls | lex.go | unexplained duplicate-code C-5e9a8436cb on toml.lexLongUnicodeEscape |

## Triaged survivors

| operator | site_symbol | dose | status | reason |
|---|---|---|---|---|
| decision-to-map-lookup | internal/columbo.choiceRepresentationVariant | 1 | blind-spot | A map from the same variants to values repeats the variant knowledge at a second site, but Columbo counts only switches and if-chains as decision sites. Fixture: fixtures/variants. |
| decision-to-map-lookup | internal/columbo.genericOriginVariant | 1 | blind-spot | A map from the same variants to values repeats the variant knowledge at a second site, but Columbo counts only switches and if-chains as decision sites. Fixture: fixtures/variants. |
| decision-to-map-lookup | internal/columbo.overwriteBits | 1 | blind-spot | A map from the same variants to values repeats the variant knowledge at a second site, but Columbo counts only switches and if-chains as decision sites. Fixture: fixtures/variants. |
| decision-to-map-lookup | internal/columbo.overwriteBitsVariant | 1 | blind-spot | A map from the same variants to values repeats the variant knowledge at a second site, but Columbo counts only switches and if-chains as decision sites. Fixture: fixtures/variants. |
| decision-to-map-lookup | internal/columbo.overwriteBits | 1 | blind-spot | A map from the same variants to values repeats the variant knowledge at a second site, but Columbo counts only switches and if-chains as decision sites. Fixture: fixtures/variants. |
| decision-to-map-lookup | internal/columbo.overwriteBitsVariant | 1 | blind-spot | A map from the same variants to values repeats the variant knowledge at a second site, but Columbo counts only switches and if-chains as decision sites. Fixture: fixtures/variants. |
| decision-to-map-lookup | internal/columbo.declarationSubjectsVariant | 1 | blind-spot | A map from the same variants to values repeats the variant knowledge at a second site, but Columbo counts only switches and if-chains as decision sites. Fixture: fixtures/variants. |
| decision-to-map-lookup | internal/columbo.binaryVariant | 1 | blind-spot | A map from the same variants to values repeats the variant knowledge at a second site, but Columbo counts only switches and if-chains as decision sites. Fixture: fixtures/variants. |
| decision-to-map-lookup | internal/columbo.visitTypeVariant | 1 | blind-spot | A map from the same variants to values repeats the variant knowledge at a second site, but Columbo counts only switches and if-chains as decision sites. Fixture: fixtures/variants. |
| decision-to-map-lookup | github.com/google/uuid.stringVariant | 1 | blind-spot | A map from the same variants to values repeats the variant knowledge at a second site, but Columbo counts only switches and if-chains as decision sites. Fixture: fixtures/variants. |
| decision-to-map-lookup | github.com/google/uuid.stringVariant | 1 | blind-spot | A map from the same variants to values repeats the variant knowledge at a second site, but Columbo counts only switches and if-chains as decision sites. Fixture: fixtures/variants. |
| decision-to-map-lookup | github.com/google/uuid.timeVariant | 1 | blind-spot | A map from the same variants to values repeats the variant knowledge at a second site, but Columbo counts only switches and if-chains as decision sites. Fixture: fixtures/variants. |
| decision-to-map-lookup | github.com/google/uuid.stringVariant | 1 | blind-spot | A map from the same variants to values repeats the variant knowledge at a second site, but Columbo counts only switches and if-chains as decision sites. Fixture: fixtures/variants. |
| decision-to-map-lookup | github.com/google/uuid.stringVariant | 1 | blind-spot | A map from the same variants to values repeats the variant knowledge at a second site, but Columbo counts only switches and if-chains as decision sites. Fixture: fixtures/variants. |
| pack-anonymous-struct | internal/columbo.coordinationDecision | 1 | blind-spot | One anonymous struct parameter is one canonical type, so both data-clump and long-parameter-list go quiet while every caller now spells the same struct type out. Fixture: fixtures/clump. |
| pack-anonymous-struct | internal/columbo.coordinationDecision | 1 | blind-spot | One anonymous struct parameter is one canonical type, so both data-clump and long-parameter-list go quiet while every caller now spells the same struct type out. Fixture: fixtures/clump. |
| pack-anonymous-struct | internal/columbo.coordinationDecision | 1 | blind-spot | One anonymous struct parameter is one canonical type, so both data-clump and long-parameter-list go quiet while every caller now spells the same struct type out. Fixture: fixtures/clump. |
| pack-anonymous-struct | internal/columbo.RoleCandidate | 1 | blind-spot | One anonymous struct parameter is one canonical type, so both data-clump and long-parameter-list go quiet while every caller now spells the same struct type out. Fixture: fixtures/clump. |
| pack-anonymous-struct | internal/columbo.DeclarationEvidence | 1 | blind-spot | One anonymous struct parameter is one canonical type, so both data-clump and long-parameter-list go quiet while every caller now spells the same struct type out. Fixture: fixtures/clump. |
| pack-anonymous-struct | internal/columbo.selectionSite | 1 | blind-spot | One anonymous struct parameter is one canonical type, so both data-clump and long-parameter-list go quiet while every caller now spells the same struct type out. Fixture: fixtures/clump. |
| pack-anonymous-struct | internal/columbo.file | 1 | blind-spot | One anonymous struct parameter is one canonical type, so both data-clump and long-parameter-list go quiet while every caller now spells the same struct type out. Fixture: fixtures/clump. |
| pack-anonymous-struct | internal/columbo.file | 1 | blind-spot | One anonymous struct parameter is one canonical type, so both data-clump and long-parameter-list go quiet while every caller now spells the same struct type out. Fixture: fixtures/clump. |
| pack-anonymous-struct | internal/columbo.inspectionSpan | 1 | blind-spot | One anonymous struct parameter is one canonical type, so both data-clump and long-parameter-list go quiet while every caller now spells the same struct type out. Fixture: fixtures/clump. |
| pack-anonymous-struct | internal/columbo.inspectionSpan | 1 | blind-spot | One anonymous struct parameter is one canonical type, so both data-clump and long-parameter-list go quiet while every caller now spells the same struct type out. Fixture: fixtures/clump. |
| pack-anonymous-struct | internal/columbo.advisorySite | 1 | blind-spot | One anonymous struct parameter is one canonical type, so both data-clump and long-parameter-list go quiet while every caller now spells the same struct type out. Fixture: fixtures/clump. |
| pack-anonymous-struct | internal/columbo.Report | 1 | blind-spot | One anonymous struct parameter is one canonical type, so both data-clump and long-parameter-list go quiet while every caller now spells the same struct type out. Fixture: fixtures/clump. |
| pack-anonymous-struct | internal/columbo.Report | 1 | blind-spot | One anonymous struct parameter is one canonical type, so both data-clump and long-parameter-list go quiet while every caller now spells the same struct type out. Fixture: fixtures/clump. |
| pack-anonymous-struct | internal/columbo.Report | 1 | blind-spot | One anonymous struct parameter is one canonical type, so both data-clump and long-parameter-list go quiet while every caller now spells the same struct type out. Fixture: fixtures/clump. |
| pack-anonymous-struct | internal/columbo.Report | 1 | blind-spot | One anonymous struct parameter is one canonical type, so both data-clump and long-parameter-list go quiet while every caller now spells the same struct type out. Fixture: fixtures/clump. |
| pack-anonymous-struct | internal/columbo.engine | 1 | blind-spot | One anonymous struct parameter is one canonical type, so both data-clump and long-parameter-list go quiet while every caller now spells the same struct type out. Fixture: fixtures/clump. |
| pack-anonymous-struct | github.com/spf13/cobra.Command | 1 | blind-spot | One anonymous struct parameter is one canonical type, so both data-clump and long-parameter-list go quiet while every caller now spells the same struct type out. Fixture: fixtures/clump. |
| pack-anonymous-struct | github.com/go-chi/chi/v5.Context | 1 | blind-spot | One anonymous struct parameter is one canonical type, so both data-clump and long-parameter-list go quiet while every caller now spells the same struct type out. Fixture: fixtures/clump. |
| pack-anonymous-struct | github.com/BurntSushi/toml.item | 1 | blind-spot | One anonymous struct parameter is one canonical type, so both data-clump and long-parameter-list go quiet while every caller now spells the same struct type out. Fixture: fixtures/clump. |
| pack-anonymous-struct | github.com/BurntSushi/toml.item | 1 | blind-spot | One anonymous struct parameter is one canonical type, so both data-clump and long-parameter-list go quiet while every caller now spells the same struct type out. Fixture: fixtures/clump. |
| pack-anonymous-struct | github.com/BurntSushi/toml.item | 1 | blind-spot | One anonymous struct parameter is one canonical type, so both data-clump and long-parameter-list go quiet while every caller now spells the same struct type out. Fixture: fixtures/clump. |
| pack-anonymous-struct | github.com/BurntSushi/toml.item | 1 | blind-spot | One anonymous struct parameter is one canonical type, so both data-clump and long-parameter-list go quiet while every caller now spells the same struct type out. Fixture: fixtures/clump. |
| pack-anonymous-struct | github.com/BurntSushi/toml.item | 1 | blind-spot | One anonymous struct parameter is one canonical type, so both data-clump and long-parameter-list go quiet while every caller now spells the same struct type out. Fixture: fixtures/clump. |
| pack-anonymous-struct-lpl | internal/columbo.coordinationDecision | 1 | blind-spot | Same as pack-anonymous-struct, measured against long-parameter-list. |
| pack-anonymous-struct-lpl | internal/columbo.RoleCandidate | 1 | blind-spot | Same as pack-anonymous-struct, measured against long-parameter-list. |
| pack-anonymous-struct-lpl | internal/columbo.helperPair | 1 | blind-spot | Same as pack-anonymous-struct, measured against long-parameter-list. |
| pack-anonymous-struct-lpl | internal/columbo.variantGroup | 1 | blind-spot | Same as pack-anonymous-struct, measured against long-parameter-list. |
| pack-anonymous-struct-lpl | internal/columbo.Detail | 1 | blind-spot | Same as pack-anonymous-struct, measured against long-parameter-list. |
| pack-anonymous-struct-lpl | internal/columbo.selectionSite | 1 | blind-spot | Same as pack-anonymous-struct, measured against long-parameter-list. |
| pack-anonymous-struct-lpl | internal/columbo.stageDefinition | 1 | blind-spot | Same as pack-anonymous-struct, measured against long-parameter-list. |
| pack-anonymous-struct-lpl | internal/columbo.tangleComparison | 1 | blind-spot | Same as pack-anonymous-struct, measured against long-parameter-list. |
| pack-anonymous-struct-lpl | internal/columbo.file | 1 | blind-spot | Same as pack-anonymous-struct, measured against long-parameter-list. |
| pack-anonymous-struct-lpl | internal/columbo.inspectionSpan | 1 | blind-spot | Same as pack-anonymous-struct, measured against long-parameter-list. |
| pack-anonymous-struct-lpl | internal/columbo.overwriteCandidate | 1 | blind-spot | Same as pack-anonymous-struct, measured against long-parameter-list. |
| pack-anonymous-struct-lpl | internal/columbo.selectedValue | 1 | blind-spot | Same as pack-anonymous-struct, measured against long-parameter-list. |
| pack-anonymous-struct-lpl | internal/columbo.advisorySite | 1 | blind-spot | Same as pack-anonymous-struct, measured against long-parameter-list. |
| pack-anonymous-struct-lpl | internal/columbo.Suppression | 1 | blind-spot | Same as pack-anonymous-struct, measured against long-parameter-list. |
| pack-anonymous-struct-lpl | internal/columbo.categoryBranch | 1 | blind-spot | Same as pack-anonymous-struct, measured against long-parameter-list. |
| pack-anonymous-struct-lpl | internal/columbo.guardedMatch | 1 | blind-spot | Same as pack-anonymous-struct, measured against long-parameter-list. |
| pack-anonymous-struct-lpl | internal/columbo.Report | 1 | blind-spot | Same as pack-anonymous-struct, measured against long-parameter-list. |
| pack-anonymous-struct-lpl | internal/columbo.engine | 1 | blind-spot | Same as pack-anonymous-struct, measured against long-parameter-list. |
| pack-anonymous-struct-lpl | internal/columbo.roleGroup | 1 | blind-spot | Same as pack-anonymous-struct, measured against long-parameter-list. |
| pack-anonymous-struct-lpl | internal/columbo.Member | 1 | blind-spot | Same as pack-anonymous-struct, measured against long-parameter-list. |
| pack-anonymous-struct-lpl | internal/columbo.Config | 1 | blind-spot | Same as pack-anonymous-struct, measured against long-parameter-list. |
| pack-anonymous-struct-lpl | internal/columbo.parameter | 1 | blind-spot | Same as pack-anonymous-struct, measured against long-parameter-list. |
| pack-anonymous-struct-lpl | internal/columbo.SelectionContext | 1 | blind-spot | Same as pack-anonymous-struct, measured against long-parameter-list. |
| pack-anonymous-struct-lpl | internal/columbo.Cluster | 1 | blind-spot | Same as pack-anonymous-struct, measured against long-parameter-list. |
| pack-anonymous-struct-lpl | internal/columbo.Case | 1 | blind-spot | Same as pack-anonymous-struct, measured against long-parameter-list. |
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
| redteam-replay | github.com/spf13/cobra/doc.GenMarkdownTreeCustom | 1 | blind-spot | 10 of 14 frozen agent evasions silence their case without improving the design (see calibration/redteam/*/meta.yaml technique). A hand-written "Code generated ... DO NOT EDIT." header still hides a whole file, now with a generated-exclusion warning. The //nolint: prefix on prose is closed (text after the directive's second // is prose), and a function literal in a package-level var is a finding subject under FS-001. |
| redteam-replay | github.com/spf13/cobra.(*Command).InheritedFlags | 1 | blind-spot | 10 of 14 frozen agent evasions silence their case without improving the design (see calibration/redteam/*/meta.yaml technique). A hand-written "Code generated ... DO NOT EDIT." header still hides a whole file, now with a generated-exclusion warning. The //nolint: prefix on prose is closed (text after the directive's second // is prose), and a function literal in a package-level var is a finding subject under FS-001. |
| redteam-replay | github.com/spf13/cobra.defaultHelpFunc | 1 | blind-spot | 10 of 14 frozen agent evasions silence their case without improving the design (see calibration/redteam/*/meta.yaml technique). A hand-written "Code generated ... DO NOT EDIT." header still hides a whole file, now with a generated-exclusion warning. The //nolint: prefix on prose is closed (text after the directive's second // is prose), and a function literal in a package-level var is a finding subject under FS-001. |
| redteam-replay | github.com/spf13/cobra/doc.GenReSTTreeCustom | 1 | blind-spot | 10 of 14 frozen agent evasions silence their case without improving the design (see calibration/redteam/*/meta.yaml technique). A hand-written "Code generated ... DO NOT EDIT." header still hides a whole file, now with a generated-exclusion warning. The //nolint: prefix on prose is closed (text after the directive's second // is prose), and a function literal in a package-level var is a finding subject under FS-001. |
| redteam-replay | github.com/spf13/cobra.writePostscript | 1 | blind-spot | 10 of 14 frozen agent evasions silence their case without improving the design (see calibration/redteam/*/meta.yaml technique). A hand-written "Code generated ... DO NOT EDIT." header still hides a whole file, now with a generated-exclusion warning. The //nolint: prefix on prose is closed (text after the directive's second // is prose), and a function literal in a package-level var is a finding subject under FS-001. |
| redteam-replay | github.com/spf13/cobra.<package-clause> | 1 | blind-spot | 10 of 14 frozen agent evasions silence their case without improving the design (see calibration/redteam/*/meta.yaml technique). A hand-written "Code generated ... DO NOT EDIT." header still hides a whole file, now with a generated-exclusion warning. The //nolint: prefix on prose is closed (text after the directive's second // is prose), and a function literal in a package-level var is a finding subject under FS-001. |
| redteam-replay | github.com/spf13/cobra.Gt | 1 | blind-spot | 10 of 14 frozen agent evasions silence their case without improving the design (see calibration/redteam/*/meta.yaml technique). A hand-written "Code generated ... DO NOT EDIT." header still hides a whole file, now with a generated-exclusion warning. The //nolint: prefix on prose is closed (text after the directive's second // is prose), and a function literal in a package-level var is a finding subject under FS-001. |
| redteam-replay | github.com/go-chi/chi/v5/middleware.SupressNotFound | 1 | blind-spot | 10 of 14 frozen agent evasions silence their case without improving the design (see calibration/redteam/*/meta.yaml technique). A hand-written "Code generated ... DO NOT EDIT." header still hides a whole file, now with a generated-exclusion warning. The //nolint: prefix on prose is closed (text after the directive's second // is prose), and a function literal in a package-level var is a finding subject under FS-001. |
| redteam-replay | github.com/go-chi/chi/v5.(*node).FindRoute | 1 | blind-spot | 10 of 14 frozen agent evasions silence their case without improving the design (see calibration/redteam/*/meta.yaml technique). A hand-written "Code generated ... DO NOT EDIT." header still hides a whole file, now with a generated-exclusion warning. The //nolint: prefix on prose is closed (text after the directive's second // is prose), and a function literal in a package-level var is a finding subject under FS-001. |
| redteam-replay | github.com/go-chi/chi/v5/middleware.(*defaultLogEntry).Write | 1 | blind-spot | 10 of 14 frozen agent evasions silence their case without improving the design (see calibration/redteam/*/meta.yaml technique). A hand-written "Code generated ... DO NOT EDIT." header still hides a whole file, now with a generated-exclusion warning. The //nolint: prefix on prose is closed (text after the directive's second // is prose), and a function literal in a package-level var is a finding subject under FS-001. |
| route-through-facade | internal/columbo.(choiceObjects).selector | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | internal/columbo.(*declaration).selection | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | internal/columbo.declarationRefLess | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | internal/columbo.newDeclaration | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | internal/columbo.(*dependencyCollector).components | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | internal/columbo.(*clusterScan).statement | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | internal/columbo.(*typeNormalizer).channel | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | internal/columbo.(*callResolver).selection | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | internal/columbo.(*callGraph).resolve | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | internal/columbo.(summaryMetric).write | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | internal/columbo.(guardedPrefix).otherStatement | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | internal/columbo.normalizedField | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | internal/columbo.(*RoleCandidate).classify | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | internal/columbo.(*snapshotWriter).advisoryCoverage | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | internal/columbo.emptyReport | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | internal/columbo.(*complexityVisitor).markCalculated | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | internal/columbo.(*declaration).scoredEvidence | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | internal/columbo.(inspectionInput).analyze | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | internal/columbo.(*overwriteScan).identifier | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | internal/columbo.(*complexityVisitor).visitFuncLit | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | internal/columbo.(*declaration).event | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | internal/columbo.(*guardedNormalizer).namedConstant | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | internal/columbo.(*cosmeticInvestigation).metrics | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | internal/columbo.(*RoleCandidate).merge | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | internal/columbo.(*declaration).guardedParameterTarget | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
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
| route-through-facade | github.com/spf13/cobra/doc.GenMarkdownCustom | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
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
| route-through-facade | github.com/go-chi/chi/v5/middleware.Recoverer | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/go-chi/chi/v5.methodTypString | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/go-chi/chi/v5/middleware.cW | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/go-chi/chi/v5.setPathValue | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/go-chi/chi/v5/middleware.(HeaderRouter).RouteAny | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/go-chi/chi/v5.(*Mux).Delete | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/go-chi/chi/v5/middleware.Maybe | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/go-chi/chi/v5/middleware.(*flushHijackWriter).Flush | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/go-chi/chi/v5/middleware.(HeaderRoute).IsMatch | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/go-chi/chi/v5.(*node).getEdge | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/go-chi/chi/v5/middleware.Timeout | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/go-chi/chi/v5/middleware.(*http2FancyWriter).Flush | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/go-chi/chi/v5/middleware.Compress | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/go-chi/chi/v5.NewRouter | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/go-chi/chi/v5.(nodes).Sort | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/go-chi/chi/v5.(*Mux).Connect | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/go-chi/chi/v5.URLParam | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/go-chi/chi/v5/middleware.NewWrapResponseWriter | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/go-chi/chi/v5.(*Mux).NotFoundHandler | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/go-chi/chi/v5/middleware.(*compressResponseWriter).Write | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/go-chi/chi/v5/middleware.RequestSize | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/go-chi/chi/v5/middleware.(*basicWriter).maybeWriteHeader | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/go-chi/chi/v5/middleware.encoderDeflate | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/go-chi/chi/v5/middleware.Throttle | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/go-chi/chi/v5.Chain | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
| route-through-facade | github.com/BurntSushi/toml.(*MetaData).unify | 1 | blind-spot | The scored dependency set excludes unexported same-package types by design, so one private struct with forwarding methods hides any number of collaborators. SPEC only says not to recommend a facade; nothing detects one. Fixture: fixtures/deps. |
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
| split-forwarding-helpers | github.com/spf13/cobra.writeRequiredNouns | 2 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-forwarding-helpers | github.com/spf13/cobra.writeRequiredNouns | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-forwarding-helpers | github.com/go-chi/chi/v5/middleware.init#1 | 4 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-forwarding-helpers | github.com/go-chi/chi/v5/middleware.init#1 | 4 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-forwarding-helpers | github.com/go-chi/chi/v5/middleware.init#1 | 4 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-forwarding-helpers | github.com/google/uuid.(UUID).URN | 4 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | internal/columbo.coordinationNoBypass | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | internal/columbo.coordinationExitUnknown | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | internal/columbo.(*engine).inspectionFacts | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | internal/columbo.(*engine).inspectionFiles | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | internal/columbo.(*engine).inspectionFiles | 4 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | internal/columbo.(*engine).inspectionCall | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | internal/columbo.(*file).inspectionType | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | internal/columbo.(*file).inspectionType | 4 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/spf13/cobra.writeRequiredFlag | 2 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/spf13/cobra.genBashComp | 2 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
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
| split-long-function | github.com/spf13/cobra.(ShellCompDirective).string | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/spf13/cobra.(ShellCompDirective).string | 4 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/spf13/cobra.(*Command).initCompleteCmd | 2 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/spf13/cobra.(*Command).initCompleteCmd | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/spf13/cobra.(*Command).initCompleteCmd | 4 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/spf13/cobra.completeRequireFlags | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/spf13/cobra.genFishComp | 2 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/spf13/cobra.genFishComp | 4 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/spf13/cobra.genPowerShellComp | 2 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/spf13/cobra.genPowerShellComp | 4 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/spf13/cobra.genZshComp | 2 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/go-chi/chi/v5/middleware.Profiler | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/go-chi/chi/v5/middleware.Profiler | 4 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/go-chi/chi/v5/middleware.realIP | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/go-chi/chi/v5/middleware.(prettyStack).parse | 4 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/go-chi/chi/v5.(*Mux).routeHTTP | 2 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/go-chi/chi/v5.(*node).findEdge | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/go-chi/chi/v5.(*node).findEdge | 4 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/go-chi/chi/v5.(*node).routes | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/go-chi/chi/v5.patParamKeys | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/go-chi/chi/v5.(nodes).findEdge | 3 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/go-chi/chi/v5.(nodes).findEdge | 4 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/BurntSushi/toml/cmd/tomlv.main | 2 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/BurntSushi/toml.(*Encoder).eMap | 2 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
| split-long-function | github.com/BurntSushi/toml.(*Encoder).eStruct | 2 | blind-spot | Helpers that forward the parent's parameters unchanged but share fewer than 75% of their dependencies fall under cosmetic-dependency-overlap (probe: cobra writePreamble, 2 of 3 shared, 0.67). At 0.5, splits of real long functions caught rise from 20 to 49 of 100 with one new natural case; see docs/calibration-tuning-proposals.md proposal 4 and PR #60. |
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
| inline-parameter-object | internal/columbo.Suppression | 1 | equivalent | The expanded {file, smell, symbol} parameters join a clump Columbo already reports, anchored at the Case struct, whose fields Suppression shares. One case per clump family means no new case appears, and the operator's prediction does not check the baseline for an existing family. |

## Untriaged survivors

Inject and evade survivors the operator's prediction cannot explain and calibration/triage.yaml does not cover. A surviving cure only means the cure did not get under the limit, so it is not a Columbo blind spot and is not listed. CI warns on every row here.

None.
