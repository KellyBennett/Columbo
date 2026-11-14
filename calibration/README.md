# Mutation calibration harness

This module implements [the mutation calibration plan](../docs/mutation-calibration-plan.md).
It changes Go code in known ways, runs `columbo` before and after, and records whether
Columbo noticed. It reads only Columbo's SQLite snapshot. It never edits Columbo's
configuration or rules.

## Run it

```sh
go build -o /tmp/columbo ../cmd/columbo
go build -o /tmp/calibrate ./cmd/calibrate
/tmp/calibrate run --corpus <module root> --module <module path> --version <version> \
  --columbo /tmp/columbo --db calibration.sqlite --operators all --workers 8
/tmp/calibrate report --db calibration.sqlite
```

On macOS the harness resolves `TMPDIR` itself, because Columbo refuses a source root behind
the `/var` symlink. Run `go test` in this module the same way the harness does:
`TMPDIR=$(cd $TMPDIR && pwd -P)/ go test ./...`.

## Operator contract

An operator implements `harness.Operator` (and `harness.Evader` when it runs on top of a
killed inject mutant). Register it in `operators/registry.go`.

- `Sites` lists every place the operator applies, in a deterministic order. An inject or
  cure site skips declarations where the baseline already has (inject) or lacks (cure) the
  target smell, because the score is the difference against the unmutated code.
- `Doses` lists the doses to try at a site. Use `[]int{1}` when dose has no meaning.
- `Apply` returns whole new file contents keyed by module-relative path (`nil` deletes).
  It never mutates the shared `go/ast` trees: build text with `splice` and pass every
  rewritten Go file through `finish`, which formats it and repairs imports.
- `Expectation.Touched` lists the declaration symbols a finding may anchor to and still
  count. Symbols use Columbo's format (`symbolOf`).
- `Expectation.Prediction` states the expected kill when the operator can compute it from
  metrics in the rewritten source, with a reason that names the metric and the limit.
  A survivor the prediction explains is scored `equivalent`.
- `Expectation.Catches` (evade only) lists the smells that count as Columbo still noticing.
- `Expectation.Renames` and `Moves` (neutral only) predict which case IDs may change.

A mutant that fails `go vet` or the mutated packages' tests is `invalid` and discarded.
Prefer a narrower operator to one that produces many invalid mutants.

Each operator lands with a fixture module under `fixtures/` and a test in
`operators/fixture_test.go` style that shows its kill (or, for neutral operators, zero
drift).

## Statuses

| Class | killed | survived | other |
|---|---|---|---|
| inject | target smell added on a touched symbol | not added, prediction did not explain it | `equivalent`: prediction said no kill |
| evade | a `Catches` smell still added on a touched symbol | the evade silenced the smell | |
| neutral | | | `equivalent`: no change; `id-drift`: a case ID changed unexplained |
| cure | target removed, nothing added | target remains | `conflict`: target removed, another smell added |

## Chains

A chain applies several operators in a row, rereading the code after each step, and checks
one of two invariants against the unmutated code. A chain of neutral operators must not
change any case ID beyond what the composed renames and moves predict. A chain containing an
inject predicted to fire must still be noticed, by the same rule the pairwise scorers use.
Cure operators are left out, since a cure is meant to clear a smell. Any other chain, such as
evades with no inject predicted to fire, has no invariant and is skipped.

`FuzzChains` in `operators/chains_test.go` picks a fixture with its first byte and up to four
steps with the rest. A violation is shrunk one step at a time to a 1-minimal chain. A shrunk
chain that matches a file in `chains/`, or a two-step plant-and-hide pair whose hide
`triage.yaml` lists as a blind spot, is known and passes. Anything else fails with the JSON to
save. Each input copies the module and runs Columbo, so expect about one input a second:

```sh
TMPDIR=$(cd $TMPDIR && pwd -P)/ go test -run '^$' -fuzz FuzzChains -fuzztime 10m ./operators/
```

Go saves a failing input under `operators/testdata/fuzz`, which `.gitignore` excludes because
the repository's database-boundary check rejects new `testdata` directories. Save the printed
chain under `chains/` instead. `TestSavedChainsStillViolate` replays every saved chain and fails once
Columbo notices one, so a closed blind spot is retired on purpose.

## Agent trials and lead checks

Plan phase 8 asks two questions outside CI. Does an agent handed Columbo's output fix the
problem Columbo reports? Does each lead that names a refactoring clear its own case when that
refactoring is applied? `agent-trials.sh` answers both and writes
[reports/agent-trials.md](reports/agent-trials.md):

```sh
./agent-trials.sh calibration.sqlite all
```

The `leads` step runs no model. The `trials` step runs the `claude` CLI and costs money. A
rerun skips trials the database already holds, keyed by mutant, repeat, agent command, model,
and prompt version, and retries rows where the agent never started. `WORKERS` sets how many
agents run at once (default 4). `OUT` keeps each trial's prompt, transcript, and patches.

### How a trial runs

`calibrate trials run` reads `trials.tsv`, applies each listed inject operator on its corpus,
and keeps the mutants Columbo kills that still pass `go vet` and the tests. Each smell gets
`--per-smell` trials (default 5). They take one mutant per corpus before any corpus gives a
second site or dose, and repeat mutants when a smell has fewer than five, because agent
nondeterminism is part of the spread. No fixture gives `inline-function-complexity` a kill, so
`high-cognitive-complexity` trials use cobra v1.9.1 from `corpus.txt`. `trials run` warns, before
and after the run, about any planned smell that planted no killed mutant.

The agent works in a fresh copy of the mutant. The prompt (`trials/prompt.go`, versioned by
`PromptVersion`) holds Columbo's `--staged=false` output for the cases the mutant added, and
each case's why, diagnosis, leads, and avoid guidance from the snapshot. It never says how the
code changed. The default command is `claude -p --model sonnet` with `--safe-mode` and
`--restricted`, so the user's settings, `CLAUDE.md`, plugins, and MCP servers stay out, and
Claude Code's file tools stay inside the workspace. The allowed tools let the agent start
`go build`, `go vet`, `go test`, and `columbo`, plus the read-only commands Claude Code always
allows. The run stops at 40 turns or 20 minutes. `--model` runs a different model, and
`--agent` replaces the whole command.

### The agent is not isolated

The allowed-tools list limits which commands the agent starts, not what they do. `go test`
compiles and runs test code, and the agent can write that code, so a trial can do anything
the user running it can do. This was accepted rather than solved:

- The code is pinned public code: the fixtures in this repository, and cobra v1.9.1 fetched
  checksum-verified into the module cache.
- Each agent works in a fresh temporary copy of its mutant.
- The agent inherits only `PATH`, `HOME`, `USER`, `TMPDIR`, and the Go toolchain and cache
  variables, plus the settings the harness adds. The `claude` CLI needs `USER` and `HOME` to
  find its own login. No other token, socket, or credential reaches the agent or its tests.

Run trials only where that is acceptable. A container would isolate them, and the harness
does not build one. The same reasoning reverses an earlier rule for trials only.
`calibrate.sh` runs downloaded corpora at build validity, so their code never runs. Trials run
every corpus's tests, cobra's included, because whether the tests still pass is half of what a
trial measures. Lead checks on downloaded corpora keep build validity.

### How a trial is scored

After the agent stops, the harness diffs the workspace against the mutant, runs `go vet` and
the tests, and runs Columbo again. It compares the result with the unmutated baseline:

- **Cleared.** No case of the planted smell appears beyond the baseline, anywhere in the module.
- **Other remaining.** The mutant's other added cases that Columbo still reports.
- **Introduced.** Cases that neither the baseline nor the mutant had.
- **Out of scope gone.** Cases the prompt said to leave alone that the fix removed or moved.
- **Gamed.** A new suppression or generated-code marker. A gamed trial never counts as cleared.
  A changed `.columbo.yml` is restored before scoring and flagged as `config`.

Fix distance compares token sequences from `go/scanner` over every Go file that the original,
the mutant, or the fix changed. Comments count as tokens, because a `prose-comment` mutant
changes nothing else. All semicolons fold into one token, so line breaks inside an expression
cost nothing. The normalized distance is the token edits from the fix to the original divided
by the token edits from the mutant to the original. A fix at 0 restored the original. A fix
at 1 got no closer than the mutant. A fix above 1 moved further away.

Distance measures closeness to the code that existed before the mutation, not design
quality. A different good refactoring scores above 0, and above 1 when it restructures more
than the mutant changed. Moving a function to another file counts as a deletion plus an
insertion. Read the distance beside the cleared, tests, and introduced columns, never alone.
For `inline-function` mutants the inlined helper stays in the file, so calling it again
restores the original at distance 0.

### How a lead check runs

`calibrate trials leads` reads every lead Columbo wrote on each natural case of a corpus and
looks it up in `leadRefactorings` in `trials/leads.go`. A lead names a refactoring only when
its text tells the reader to perform one. A question or an instruction to inspect names none.
For a lead that names a refactoring with a cure operator, the checker applies that operator at
every site and dose aimed at the case, meaning it rewrites one of the case's declarations in
one of its files. It scores each application against the lead's own case, which may be a
different smell from the one the operator targets. The best application decides the outcome:
`cleared`, `conflict` (cleared but another rule fired), `survived`, or `invalid`. A lead whose
operator has no site at the case is `not-applicable`. A lead whose refactoring has no operator
is `no-operator`. Lead text missing from the table is `unclassified`, and the report lists it.

## Decision trail

`decisions.tsv` logs each decision with its evidence, one row per decision.
