# All-rule diagnostics

`columbo --staged=false ./...` turns off stage enforcement and reports every case. Each case shows its ID, smell, symbol and location, stored verdict, triggering metrics with limits, policy-review notes, and correlations. A totals line follows. Exit is 1 when any unsuppressed FAIL exists and 0 otherwise.

## Sub-features

- `diag-cases` prints a `CASE C-<10 hex> <symbol> <file>:<start>-<end>` block for each finding, with `<smell>: FAIL|WARN` and metric lines such as `function-lines fixture.LongFunction: 11 (limit > 10)`.
- `diag-policy` prints `Policy <ID>:` and `Review:` lines on cases that have a provisional policy (for example `CE-001` and `SUC-001`).
- `diag-totals` prints `Columbo: <n> failed, <n> warned, <n> suppressed`.
- `diag-exit` exits 1 on an unsuppressed FAIL, 0 when none remain, and 2 on invalid flags.

## How to get to it (user POV)

- Run `columbo --staged=false ./...` from a module root.
- Use the GitHub Action with `staged: 'false'`.

## Driving it with columbo-verify

Preconditions:

- Baseline from the [README](./README.md).

- **Full report.** Run `$V stage "$RUN" all` and then `$V drive "$RUN" all diag --staged=false --no-history`. Exit is `1`. `stdout.txt` has 11 `CASE ` lines (`grep -c '^CASE '`) and contains `Columbo: 11 failed, 0 warned, 0 suppressed`.
- **Specific case.** `stdout.txt` contains `CASE C-b10bccb46b fixture.LongFunction all.go:13-25`, followed by `long-function: FAIL`.
- **Stored match.** Run `$V sql "$RUN" diag unsuppressed-failures.sql`. It returns 11 data rows, and the first is `C-b10bccb46b|long-function|fixture.LongFunction|all.go|13|25|FAIL|…`.
- **Clean subject.** Run `$V stage "$RUN" tangle tangle` and then `$V drive "$RUN" tangle diag-clean --staged=false --no-history`. Exit is `0`, and the totals are `Columbo: 0 failed, 0 warned, 0 suppressed`. The tangle subject blocks only the staged gate.
- **Bad flag.** Run `$V drive "$RUN" all badflag --format=json`. Exit is `2`, `stdout.txt` is empty, `stderr.txt` starts with `columbo: flag provided but not defined: -format`, and no `report.sqlite` exists.

## Gotchas

- Case IDs are stable across line shifts. A changed ID for an unchanged declaration is a regression, not noise.
- `CASE` blocks give the compact view. The full why, diagnosis, leads, and receipts are only in SQLite, so prove them with `sql`.
- `fixture-all` mirrors `internal/columbo/testdata/all-summary.golden`. If your stdout differs from the golden, the golden test should be failing too. Check that before you blame the harness.
