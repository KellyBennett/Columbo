# Columbo verification map

This directory is the maintained source for verifying Columbo's user-facing behavior. Read this index before you drive the CLI, then use the matching feature file as the recipe.

## Baseline preconditions

- `RUN=$(.claude/skills/verify-columbo/scripts/columbo-verify launch)` succeeded from the repo root at the commit under test.
- `columbo-verify doctor "$RUN"` prints no `FAIL` and no `WARN`.
- Each feature stages its own subject under `$RUN/subjects/`. Never analyze a module in place, including this repo's `testdata/`.

## Driving conventions

- `V=.claude/skills/verify-columbo/scripts/columbo-verify`.
- Analyze through `$V drive`. It runs `columbo --output <evidence>/report.sqlite [flags] ./...` from the subject root.
- Use the raw binary `$RUN/bin/columbo` only for invocations that `drive` cannot express, such as a custom `--output`. Prefix it with `PATH="$(cat "$RUN/go-bin"):$PATH"`.
- Flags precede package patterns. Treat each command as literal.
- To mutate a subject, edit the copy under `$RUN/subjects/<name>/`, then drive again with a new label.

## Proof and skip reporting

- CLI proof is `command.txt`, `stdout.txt`, `stderr.txt`, and `exit-code.txt` from one label.
- Stored-state proof is a `$V sql` query against the same label's `report.sqlite`.
- A behavior change needs a before label and an after label on the same subject.
- Record the feature file and sub-feature ID with each label you report.
- If you cannot reach a path, report the attempted command and the unmet precondition. Do not substitute a different entry point.

## Feature entry contract

Each feature file starts with an H1 title and one paragraph that describes the user-visible behavior. It then has exactly four H2 sections, in this order: `Sub-features`, `How to get to it (user POV)`, `Driving it with columbo-verify`, and `Gotchas`.

## Features

- [Staged gate](./staged-gate.md) covers the default `columbo ./...` run: stage states, the active task, and exit 0 or 1.
- [All-rule diagnostics](./all-rule-diagnostics.md) covers `--staged=false`: every case, its verdict and metrics, totals, and exit codes.
- [Source suppressions](./suppressions.md) covers `// columbo:ignore` directives: their accounting, validation failures, and unused warnings.
- [Configuration](./configuration.md) covers `.columbo.yml` and `--config`: severity and threshold overrides, and rejection of invalid config.
- [Evidence snapshot](./evidence-snapshot.md) covers the SQLite output, the `docs/sqlite` queries, and output-path refusals.
- [GitHub Action analyze step](./github-action.md) covers `scripts/run-action.sh` run locally, plus the publisher's offline tests.
