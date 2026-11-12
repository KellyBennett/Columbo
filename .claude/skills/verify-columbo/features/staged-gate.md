# Staged gate

Running `columbo ./...` with no flags enforces ordered refactoring stages: Untangle Behavior, then Assign Ownership, then Consolidate Shared Behavior. The first stage with collector issues is `active`, and later stages are `locked`. Columbo prints the active stage's task and evidence, and exits 1. When every stage is `cleared`, it exits 0. Ordinary FAIL cases alone do not block this gate.

## Sub-features

- `staged-cleared` reports all stages `cleared` and exits 0, even when the subject has FAIL cases.
- `staged-active` reports the first stage with issues as `active` and later stages as `locked`, prints `Task:` and `Evidence` lines, and exits 1.
- `staged-transition` clears the stage once the tangled code is fixed, with no other change.
- `staged-stored` stores the stage states and issues in the snapshot (`stages.sql`).

## How to get to it (user POV)

- Run `columbo ./...` (or `columbo` with no arguments) from a Go module root.
- Run the GitHub Action with the default `staged: true` input. See [github-action](./github-action.md).

## Driving it with columbo-verify

Preconditions:

- Baseline from the [README](./README.md).

- **Cleared despite failures.** Run `$V stage "$RUN" all` and then `$V drive "$RUN" all staged-clear --no-history`. Exit is `0`. `stdout.txt` contains `Stage Untangle Behavior: cleared`, `Stage Assign Ownership: cleared`, and `Stage Consolidate Shared Behavior: cleared`, and has no `CASE ` lines.
- **Active stage blocks.** Run `$V stage "$RUN" tangle tangle` and then `$V drive "$RUN" tangle staged-active --no-history`. Exit is `1`. `stdout.txt` contains `Stage Untangle Behavior: active`, `Collector nested-field-decision: 1 issues`, `Stage Assign Ownership: locked`, a `Task:` line, and an `Evidence TC-… nested-field-decision: tangle.Item.Name` line.
- **Stored state.** Run `$V sql "$RUN" staged-active stages.sql`. The first result row is `0|Untangle Behavior|active|0|1|…`, and the `active_stage_issues` row has kind `nested-field-decision`.
- **Transition.** In `$RUN/subjects/tangle/tangle.go`, replace the body of `update` with `switch x.Name { case "A": x.Quality++; case "B": default: x.Quality--; x.SellIn-- }`, formatted one statement per line. Each name category becomes its own branch. Then run `$V drive "$RUN" tangle staged-fixed --no-history`. Exit is `0` and every stage is `cleared`.

## Gotchas

- In staged mode, the totals line (`Columbo: N failed…`) and the `CASE` blocks are absent. Asserting on them proves nothing. Use [all-rule diagnostics](./all-rule-diagnostics.md) for cases.
- `cleared` means only that the collectors found no issues. The output says this, and so should your proof.
- Without `--no-history`, the subject is not a git repo, so the output gains a `history-unavailable` warning on stdout and stderr. The warning is expected and is not a failure.
- The transition rewrite must change behavior categories, not just reformat them. A guard that is still nested still trips `nested-field-decision`.
