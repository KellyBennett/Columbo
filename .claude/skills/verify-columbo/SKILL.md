---
name: verify-columbo
description: Build and drive the Columbo CLI (`columbo ./...`) against disposable Go modules and capture stdout, stderr, exit code, and the SQLite snapshot as proof. Use when you need to show that a Columbo change works as a user sees it — staged gate, all-rule diagnostics, suppressions, configuration, snapshot evidence, or the GitHub Action's analyze step — rather than only passing `go test`.
---

# Verify Columbo

Columbo is a Go CLI. A user runs `columbo [flags] ./...` from inside a Go module. It prints a summary and writes a SQLite snapshot. Exit `0` means the gate is clear, `1` means it is blocked, and `2` means the invocation failed. The secondary surface is the GitHub composite action (`action.yml`). This skill drives the action's analyze step locally. It never drives the publisher, because the publisher writes to the GitHub Checks API.

Everything goes through one helper. Run it from the repo root:

```sh
V=.claude/skills/verify-columbo/scripts/columbo-verify
```

## Launch

```sh
RUN=$($V launch)
```

This creates a fresh, symlink-free run dir under `$TMPDIR`. It builds `cmd/columbo` from the current checkout into `$RUN/bin/columbo`, and records the commit in `$RUN/evidence/columbo-commit.txt`. Launch is ready when it prints the `RUN` path and exits 0.

- Columbo needs the `go` command at analysis time, not only for the build, because it loads packages through `go list`. Launch records which Go to use in `$RUN/go-bin`. It uses `go` from `PATH` when it matches `go.mod`'s version (1.25.1). Otherwise it falls back to `mise exec go@1.25.1`. Without either, launch exits 2.
- Relaunch after every code change. The binary does not rebuild itself.
- Run dirs are independent, so parallel runs are safe. Columbo has no server, port, or shared state.

## Doctor

```sh
$V doctor "$RUN"
```

This is read-only. It checks that the binary exists, that `columbo --version` and `go version` work, and that the run-dir path is symlink-free. Columbo refuses symlinked source roots, and `/tmp` on macOS is a symlink. It prints `WARN` when the binary was built from a different commit than the current HEAD, or from a dirty tree. Treat that as "relaunch before proving anything". Run doctor first whenever a result looks wrong.

## Drive

Copy a subject module into the run, then analyze it:

```sh
$V stage "$RUN" <name> [fixture-all | tangle | /path/to/module]   # default fixture-all
$V drive "$RUN" <name> <label> [columbo flags...]                  # runs: columbo --output … [flags] ./...
$V sql   "$RUN" <label> <query.sql>                                # read-only; bare names resolve to docs/sqlite/
```

- `fixture-all` is `internal/columbo/testdata/all`. It has no stage issues, and `--staged=false` yields `11 failed, 0 warned, 0 suppressed`.
- `tangle` is `subjects/tangle` in this skill. Its staged run blocks on `Stage Untangle Behavior: active` (`nested-field-decision: 1 issues`).
- Edit files under `$RUN/subjects/<name>/`, never under `testdata/`. Golden tests depend on those files.
- Each label is single-use, so every command keeps its own evidence. `drive` always sets `--output`, so a user-supplied `--output` would conflict. Test output-path behavior with the raw binary instead, as shown in [evidence-snapshot](features/evidence-snapshot.md).
- Subjects are not git repositories. Every run warns `history-unavailable` unless you pass `--no-history`. To exercise history evidence, `git init` and commit inside the subject.

Per-feature recipes are in [features/README.md](features/README.md).

## Evidence

Every `drive` writes `$RUN/evidence/<label>/` with the following files:

| File | Proves |
| --- | --- |
| `command.txt` | the exact cwd and argv the user would type |
| `stdout.txt` / `stderr.txt` | the summary as the user sees it, and diagnostics |
| `exit-code.txt` | the gate decision (0 / 1 / 2) |
| `report.sqlite` | the stored verdicts. Absent when the run failed with exit 2 before publication. |

Proof standards:

- Drive the built binary from inside the subject, exactly as a user would. Do not call `columbo.Run` or `Analyze` from a test as a substitute.
- Pair what the user sees with the stored state. Confirm a claim in stdout with a `sql` query against `report.sqlite`, for example `unsuppressed-failures.sql`, `stages.sql`, or a `suppressions` select. The exit code comes from the snapshot, so they must agree.
- Prove a change with before and after runs under two labels on the same subject. A single after-run shows only an end state.
- Check side effects. Exit-2 runs must leave no snapshot and an empty stdout. Successful runs must not write inside the subject: `ls` the subject afterward.
- For the action's analyze step, the evidence is its evidence dir and `GITHUB_OUTPUT` file. The publisher is covered only by its offline unit tests, which mock `urlopen`. Never run `scripts/github-check.py` with a real token.

## Cleanup

```sh
$V cleanup "$RUN"
```

Removes `$RUN/bin` and `$RUN/subjects`. It keeps `$RUN/evidence/`, which is where the proof lives. Report that path to the user. Cleanup deletes only paths inside a dir that carries the `.columbo-verify` marker, and it kills no processes, because Columbo leaves none. Delete the whole `$RUN` only after the evidence has been reported or copied elsewhere.
