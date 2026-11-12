# Configuration

Columbo reads `.columbo.yml` from the module root when the file exists, or reads the path given with `--config PATH`. The file can change smell severities (`off`/`warn`/`fail`), thresholds, history, and exclude globs. Keys left out keep their defaults. The defaults are in the Configuration section of `SPEC.md`. An unknown key, a wrong type, or a missing explicit `--config` file exits 2.

## Sub-features

- `config-severity` downgrades a smell to `warn`. The case shows `WARN`, and the totals move from failed to warned.
- `config-threshold` raises a threshold, which removes cases that were under it.
- `config-explicit` loads a file named by `--config PATH`. A missing explicit path exits 2.
- `config-invalid` rejects unknown keys, duplicates, nulls, aliases, and wrong types with exit 2 and no snapshot.

## How to get to it (user POV)

- Create `.columbo.yml` in the module root.
- Pass `--config path/to/file.yml`. In the action, this is the `config:` input.

## Driving it with columbo-verify

Preconditions:

- Baseline from the [README](./README.md). Run `$V stage "$RUN" all`.

- **Override.** Write `$RUN/subjects/all/.columbo.yml` containing `version: 1`, `severity: {long-function: warn}`, and `thresholds: {parameters: 5}`, each on its own line in block YAML. Run `$V drive "$RUN" all cfg --staged=false --no-history`. The totals are `9 failed, 1 warned, 0 suppressed`. The `LongFunction` case shows `long-function: WARN`. No `fixture.Parameters` case remains. `fixture.Dependencies` still fails, with 6 parameters against a new limit of 5.
- **Unknown key.** Replace the file with `version: 1` and `bogus: true`, then drive with label `cfg-bad`. Exit is `2`, `stderr.txt` is `columbo: unknown configuration key bogus`, and there is no snapshot.
- **Missing explicit file.** Run `$V drive "$RUN" all cfg-missing --config nope.yml`. Exit is `2`, and `stderr.txt` is `columbo: open <subject>/nope.yml: no such file or directory`.
- **Reset.** Delete `$RUN/subjects/all/.columbo.yml` before you reuse the subject.

## Gotchas

- Severity changes cases but not stages. In the default staged mode, an override can look like it did nothing. Use `--staged=false` to see verdicts.
- `--no-history` overrides `history.enabled: true`.
- A supplied `exclude:` list replaces the defaults rather than merging with them. Generated files are always excluded.
