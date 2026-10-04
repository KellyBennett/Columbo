# Investigating JSON reports with jq

These reusable filters target Columbo's version 1 JSON report. They require
[jq 1.6 or newer](https://jqlang.org/) and leave Columbo's verdicts and policy
unchanged. Keep the original report: filtered views are investigation aids, not
replacement evidence or CI checks.

## Capture the report

Run Columbo from the Go module being investigated. Save stdout before querying
so findings (exit 1) are still available and jq cannot hide Columbo's exit code:

```bash
status=0
columbo --format json ./... > report.json || status=$?
case "$status" in
  0|1) ;; # Both have a complete report; 1 means unsuppressed FAIL cases.
  *) exit "$status" ;; # Investigation failed; do not query partial/empty output.
esac
```

In CI, preserve `status` and exit with it after processing the report. A successful
jq command says nothing about Columbo's verdict. Stderr diagnostics stay separate;
report warnings are also available in `.warnings`.

The examples below assume `recipes` is the path to this checkout's `docs/jq`
directory and `report.json` is the saved report:

```bash
recipes=/path/to/Columbo/docs/jq
```

## Start with failing cases

```bash
jq -f "$recipes/failures.jq" report.json
```

This returns the original version, summary and warnings plus an index of
unsuppressed FAIL cases with IDs, locations, clues (including comparisons and
sets), and policy-review notes. WARN and suppressed cases are omitted from the
index; the original summary still counts them. It intentionally omits receipts
and guidance, which are available in the full-case query below.

## Open a complete case or all cases for one symbol

Copy an ID from the index; replace the example value with an ID in your report:

```bash
jq --arg id 'C-0123456789' -f "$recipes/case.jq" report.json
```

The result is the entire case, including clues, clusters, diagnosis, leads,
avoid guidance, receipts and policy reviews. An unknown ID is an error, so a
stale ID cannot silently produce an empty result.

Use the exact qualified symbol from a case to investigate every smell affecting
that declaration, including WARN and suppressed cases:

```bash
jq --arg symbol 'example.com/shop/internal/orders.(*Service).Process' \
  -f "$recipes/symbol.jq" report.json
```

This returns an array of complete matching cases, or `[]` when none match. It
matches the primary symbol, not secondary helper or Data Clump participants;
inspect their clues/receipts in the full case. Use `--arg`, rather than inserting
values into jq expressions, so punctuation and quotes are handled safely.

## Drill into receipts

```bash
jq --arg id 'C-0123456789' --arg kind 'metric-contribution' \
  -f "$recipes/receipts.jq" report.json
```

This keeps the case's verdict, suppression status, clues, clusters and policy
reviews, selecting only the requested receipt kind. Source receipts retain their
physical line/byte ranges and complete `detail`, including `expansion` and
`expansion_sites`: separate copied contributions must not be merged merely
because they share a line or declaration chain.

Other useful kinds are `declaration`, `parameter`, `dependency`,
`dependency-inventory`, `own-access`, `foreign-access`, `helper-call`, and
`history`. `dependency` is scored evidence; `dependency-inventory` retains
additional unscored evidence. History records have `commit`, `committed_at` and
`files` instead of source ranges/detail, and describe file provenance rather
than architectural intent. History can be empty when disabled or unavailable;
check the original report's warnings. An unmatched kind returns `receipts: []`;
an unknown case ID is an error.

## Review provisional policy evidence

```bash
jq -f "$recipes/policy-reviews.jq" report.json
```

This returns complete cases carrying policy-review notes, including WARN and
suppressed cases. For CE-001, it preserves qualifying clusters and member order,
forwarded-input/dependency sets, overlap values and thresholds, original/expanded
metrics, receipt copy paths, and the literal review prompt. Review that evidence
with the human before changing code or requesting a suppression. A policy note
does not change a verdict. When no notes apply, the result is `[]`.

## Regression checks

From the checkout, run `bash scripts/test-jq-recipes.sh` with Go 1.25.1, jq 1.6+
and Git available. The script builds the current CLI and queries actual output
from the acceptance fixture, mixed FAIL/WARN/suppressed cases with history and
an unused-suppression warning, and a no-case module. It also checks missing IDs
and preservation of complete receipt and policy-review evidence. CI runs it in
the acceptance job; jq is not a runtime dependency of Columbo.
