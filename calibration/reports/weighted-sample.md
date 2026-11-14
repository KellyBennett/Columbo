# Weighted calibration sample

This is the output of `calibrate report --weights reports/ledger-export-sample.json` against a
`calibrate.sh <db> all 8` run over Columbo's own module at d598538 and the four modules in
`corpus.txt`. Both inputs come from public code only, and the sections below are unedited.
`ledger-sample.sh` regenerates the export.

## Real case shapes

Each band cell is that band's share of the smell's exported cases, then its count, from buckets with at least 5 cases in at least 3 repositories.

| smell | cases | at-limit | near | over | far | unmeasured |
|---|---|---|---|---|---|---|
| data-clump | 8 | 100% (8) |  |  |  |  |
| duplicate-code | 26 |  | 58% (15) | 42% (11) |  |  |
| excessive-dependencies | 19 | 53% (10) | 47% (9) |  |  |  |
| feature-envy | 48 | 27% (13) | 23% (11) | 23% (11) | 27% (13) |  |
| high-cognitive-complexity | 124 | 15% (18) | 21% (26) | 41% (51) | 23% (29) |  |
| long-function | 212 | 8% (18) | 16% (33) | 40% (85) | 36% (76) |  |
| prose-comment | 79 |  |  |  | 100% (79) |  |

## Weighted kill rate

Each row counts scored inject, evade, and cure mutants by the excess band of their subject case. Unweighted is killed over scored. Weighted averages each band's kill rate by that band's share of the target smell's real cases, over the bands that have scored mutants. Covered is the share of real cases those bands hold. A band cell shows killed over scored, and uncovered marks a band that holds real cases but no scored mutant. An unshaped mutant has no subject case, such as an inject that added nothing, and counts in neither rate.

| class | operator | target | shaped | unshaped | unweighted | weighted | covered | at-limit | near | over | far | unmeasured |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| cure | (all) | data-clump | 7 | 0 | 29% | 33% | 100% | 2/6 |  | 0/1 |  |  |
| cure | introduce-parameter-object | data-clump | 7 | 0 | 29% | 33% | 100% | 2/6 |  | 0/1 |  |  |
| cure | (all) | excessive-dependencies | 4 | 0 | 100% | 100% | 53% | 4/4 | uncovered |  |  |  |
| cure | split-by-collaborators | excessive-dependencies | 4 | 0 | 100% | 100% | 53% | 4/4 | uncovered |  |  |  |
| cure | (all) | feature-envy | 9 | 0 | 100% | 100% | 100% | 1/1 | 4/4 | 2/2 | 2/2 |  |
| cure | move-function | feature-envy | 9 | 0 | 100% | 100% | 100% | 1/1 | 4/4 | 2/2 | 2/2 |  |
| cure | (all) | high-cognitive-complexity | 52 | 0 | 35% | 33% | 100% | 12/13 | 5/6 | 1/21 | 0/12 |  |
| cure | extract-function-complexity | high-cognitive-complexity | 52 | 0 | 35% | 33% | 100% | 12/13 | 5/6 | 1/21 | 0/12 |  |
| cure | (all) | long-function | 71 | 0 | 32% | 34% | 100% | 10/11 | 5/5 | 8/31 | 0/24 |  |
| cure | extract-function | long-function | 71 | 0 | 32% | 34% | 100% | 10/11 | 5/5 | 8/31 | 0/24 |  |
| cure | (all) | prose-comment | 32 | 0 | 97% | 97% | 100% | 1/1 |  | 2/2 | 28/29 |  |
| cure | delete-prose-comments | prose-comment | 32 | 0 | 97% | 97% | 100% | 1/1 |  | 2/2 | 28/29 |  |
| evade | (all) | cosmetic-extraction | 21 | 0 | 100% | no real cases |  | 5/5 | 1/1 | 11/11 | 4/4 |  |
| evade | isolate-helper-calls | cosmetic-extraction | 21 | 0 | 100% | no real cases |  | 5/5 | 1/1 | 11/11 | 4/4 |  |
| evade | (all) | data-clump | 26 | 0 | 50% | 50% | 100% | 11/22 |  | 2/4 |  |  |
| evade | pack-anonymous-struct | data-clump | 13 | 0 | 0% | 0% | 100% | 0/11 |  | 0/2 |  |  |
| evade | reorder-params | data-clump | 13 | 0 | 100% | 100% | 100% | 11/11 |  | 2/2 |  |  |
| evade | (all) | duplicate-code | 18 | 0 | 100% | 100% | 100% |  | 4/4 | 7/7 | 7/7 |  |
| evade | rename-and-relabel | duplicate-code | 18 | 0 | 100% | 100% | 100% |  | 4/4 | 7/7 | 7/7 |  |
| evade | (all) | excessive-dependencies | 40 | 0 | 0% | 0% | 53% | 0/40 | uncovered |  |  |  |
| evade | route-through-facade | excessive-dependencies | 40 | 0 | 0% | 0% | 53% | 0/40 | uncovered |  |  |  |
| evade | (all) | feature-envy | 12 | 0 | 100% | 100% | 50% | 2/2 | uncovered | 10/10 | uncovered |  |
| evade | copy-fields-to-locals | feature-envy | 6 | 0 | 100% | 100% | 50% | 1/1 | uncovered | 5/5 | uncovered |  |
| evade | hide-behind-getters | feature-envy | 6 | 0 | 100% | 100% | 50% | 1/1 | uncovered | 5/5 | uncovered |  |
| evade | (all) | long-function | 18 | 0 | 67% | 66% | 64% | 3/3 | 3/4 | 6/11 | uncovered |  |
| evade | split-forwarding-helpers | long-function | 18 | 0 | 67% | 66% | 64% | 3/3 | 3/4 | 6/11 | uncovered |  |
| evade | (all) | long-parameter-list | 14 | 0 | 0% | no real cases |  | 0/6 | 0/2 | 0/4 | 0/2 |  |
| evade | pack-anonymous-struct-lpl | long-parameter-list | 14 | 0 | 0% | no real cases |  | 0/6 | 0/2 | 0/4 | 0/2 |  |
| evade | (all) | prose-comment | 9 | 0 | 0% |  | 0% | 0/9 |  |  | uncovered |  |
| evade | prose-to-string-constant | prose-comment | 9 | 0 | 0% |  | 0% | 0/9 |  |  | uncovered |  |
| evade | (all) | repeated-variant-decision | 14 | 0 | 0% | no real cases |  | 0/9 |  | 0/5 |  |  |
| evade | decision-to-map-lookup | repeated-variant-decision | 14 | 0 | 0% | no real cases |  | 0/9 |  | 0/5 |  |  |
| inject | (all) | cosmetic-extraction | 24 | 32 | 100% | no real cases |  | 3/3 | 3/3 | 14/14 | 4/4 |  |
| inject | split-long-function | cosmetic-extraction | 24 | 32 | 100% | no real cases |  | 3/3 | 3/3 | 14/14 | 4/4 |  |
| inject | (all) | data-clump | 13 | 0 | 100% | 100% | 100% | 11/11 |  | 2/2 |  |  |
| inject | inline-parameter-object | data-clump | 13 | 0 | 100% | 100% | 100% | 11/11 |  | 2/2 |  |  |
| inject | (all) | duplicate-code | 18 | 0 | 100% | 100% | 100% |  | 4/4 | 7/7 | 7/7 |  |
| inject | duplicate-block | duplicate-code | 18 | 0 | 100% | 100% | 100% |  | 4/4 | 7/7 | 7/7 |  |
| inject | (all) | excessive-dependencies | 40 | 0 | 100% | 100% | 53% | 40/40 | uncovered |  |  |  |
| inject | add-collaborators | excessive-dependencies | 40 | 0 | 100% | 100% | 53% | 40/40 | uncovered |  |  |  |
| inject | (all) | feature-envy | 38 | 0 | 100% | 100% | 100% | 10/10 | 4/4 | 17/17 | 7/7 |  |
| inject | move-method-to-collaborator | feature-envy | 11 | 0 | 100% | 100% | 100% | 5/5 | 1/1 | 3/3 | 2/2 |  |
| inject | move-method-to-function | feature-envy | 27 | 0 | 100% | 100% | 100% | 5/5 | 3/3 | 14/14 | 5/5 |  |
| inject | (all) | high-cognitive-complexity | 3 | 0 | 100% | 100% | 44% | uncovered | 2/2 | uncovered | 1/1 |  |
| inject | inline-function-complexity | high-cognitive-complexity | 3 | 0 | 100% | 100% | 44% | uncovered | 2/2 | uncovered | 1/1 |  |
| inject | (all) | long-function | 12 | 0 | 100% | 100% | 100% | 1/1 | 4/4 | 5/5 | 2/2 |  |
| inject | inline-function | long-function | 12 | 0 | 100% | 100% | 100% | 1/1 | 4/4 | 5/5 | 2/2 |  |
| inject | (all) | long-parameter-list | 14 | 0 | 100% | no real cases |  | 6/6 | 2/2 | 4/4 | 2/2 |  |
| inject | inline-parameter-object-lpl | long-parameter-list | 14 | 0 | 100% | no real cases |  | 6/6 | 2/2 | 4/4 | 2/2 |  |
| inject | (all) | prose-comment | 9 | 0 | 100% |  | 0% | 9/9 |  |  | uncovered |  |
| inject | add-prose-comment | prose-comment | 9 | 0 | 100% |  | 0% | 9/9 |  |  | uncovered |  |
| inject | (all) | repeated-variant-decision | 12 | 0 | 100% | no real cases |  | 8/8 |  | 4/4 |  |  |
| inject | repeat-variant-decision | repeated-variant-decision | 12 | 0 | 100% | no real cases |  | 8/8 |  | 4/4 |  |  |
