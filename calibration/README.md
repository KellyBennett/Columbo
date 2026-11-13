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

## Decision trail

`decisions.tsv` logs each decision with its evidence, one row per decision.
