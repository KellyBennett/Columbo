# Source suppressions

A user can suppress one smell on one function by adding `// columbo:ignore <smell-id> -- <justification>` to the function's doc comment. The case keeps its original verdict and evidence. It is marked suppressed, counted only under `suppressed`, and stops counting toward the all-rule gate. A malformed or misplaced directive is fatal. A valid directive that matches no case produces a warning.

## Sub-features

- `suppress-apply` adds a `suppression <file>:<line>: <justification>` line to the case, and the totals move from failed to suppressed.
- `suppress-stored` writes a `suppressions` row with `applied=1` and the case ID.
- `suppress-invalid` fails with exit 2, `columbo: <file>:<line>: invalid columbo suppression`, empty stdout, and no snapshot when the justification has fewer than 10 non-space characters, the smell is unknown, or the directive is not attached to a function.
- `suppress-unused` warns about a valid directive whose smell does not fire on that function, without failing.

## How to get to it (user POV)

- Edit Go source: put the directive in the doc comment directly above `func`, with no blank line between them.

## Driving it with columbo-verify

Preconditions:

- Baseline from the [README](./README.md). Run `$V stage "$RUN" all`.

- **Before.** Run `$V drive "$RUN" all sup-before --staged=false --no-history`. The totals are `11 failed, 0 warned, 0 suppressed`.
- **Apply.** Insert `// columbo:ignore long-function -- fixture body is long on purpose` on the line above `func LongFunction() {` in `$RUN/subjects/all/all.go`. Run `$V drive "$RUN" all sup-after --staged=false --no-history`. The totals are `10 failed, 0 warned, 1 suppressed`, and the `LongFunction` case still shows `long-function: FAIL` plus `suppression all.go:13: fixture body is long on purpose`.
- **Stored.** Run `sqlite3 -readonly "$RUN/evidence/sup-after/report.sqlite" "select smell,symbol,line,applied,case_id,justification from suppressions"`. The result is `long-function|fixture.LongFunction|13|1|C-b10bccb46b|fixture body is long on purpose`.
- **Invalid.** Change the justification to `short` and drive with label `sup-invalid`. Exit is `2`, `stderr.txt` is `columbo: all.go:13: invalid columbo suppression`, `stdout.txt` is empty, and there is no `report.sqlite`.
- **Unused.** Change the directive's smell to `feature-envy` and drive with label `sup-unused`. Exit is `1`, because the 11 failures are back. The output contains a warning `Unused suppression for feature-envy on fixture.LongFunction.`.

## Gotchas

- A blank line between the directive and `func` detaches it, and the run fails. It is not silently ignored.
- `data-clump` cannot be suppressed. That directive is fatal by design.
- In the default staged mode, a suppression does not change the gate, because ordinary cases do not block it. Prove suppression accounting with `--staged=false`.
- Remove or restore the directive before you reuse the `all` subject for another feature, or stage a fresh copy under a new name.
