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

## Weight kill rates by real case shapes

An unweighted kill rate counts a mutant 40 lines over the limit the same as one a line
over. To weight each band by how often real cases have that shape, pass the export of a
Columbo case ledger to the report:

```sh
columbo calibrate --export > export.json
/tmp/calibrate report --db calibration.sqlite --weights export.json
```

The report then adds two sections. `Real case shapes` shows how each smell's exported cases
split across the excess bands. `Weighted kill rate` places each scored mutant in the band of
its subject case and weights each band's kill rate by that band's share of the target
smell's real cases. [The ledger design](../docs/calibration-ledger-design.md) defines the
bands and the formula.

| Class | Subject case |
|---|---|
| inject | the target-smell case the mutant added |
| evade | the target-smell case the parent inject added |
| cure | the target-smell case on the site symbol in the baseline |

A mutant with no subject case is `unshaped`. Every surviving inject is unshaped, and so is
a frozen red-team evade, which has no parent. A row with any unshaped mutant shows its
`kill rate` but a `withheld` weighted rate, because its shaped mutants are not the population
the kill rate counts. A band with exported cases and no scored mutant shows `uncovered`.
`covered` is the share of exported cases in the bands that the weighted rate includes. A
band whose cases the export's floor dropped shows blank.

The harness stores a `ratio` with each finding. The ratio is the value of the case's first
limited clue divided by the smallest value that triggers it. `calibrate run` adds the column
to an older `calibration.sqlite` when it starts a run. `calibrate report` never writes, and
reads an older database as if every ratio were missing. Findings recorded without a ratio
put their mutants in `unmeasured` until you rerun them.

`ledger-sample.sh` builds a reproducible export from public code only. It records Columbo's
own module and every module in `corpus.txt` into a throwaway ledger under a temporary HOME,
then exports it. It never reads or writes your real `~/.columbo`.

```sh
./ledger-sample.sh reports/ledger-export-sample.json
```

`reports/ledger-export-sample.json` is that export, and `reports/weighted-sample.md` is the
weighted report it produces against a `calibrate.sh <db> all 8` run over the same modules.

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

## Decision trail

`decisions.tsv` logs each decision with its evidence, one row per decision.
