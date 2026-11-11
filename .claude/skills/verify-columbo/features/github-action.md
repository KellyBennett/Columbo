# GitHub Action analyze step

Consumers use `KellyBennett/Columbo@main` as a composite action. The action builds Columbo from the action revision, analyzes the consumer module, publishes a `Columbo findings` check through the GitHub Checks API, uploads the evidence dir as an artifact, and then exits with the analyzer's code. Locally, only the analyze step (`scripts/run-action.sh`) is driven. The publisher (`scripts/github-check.py`) is outward-facing, so it is proven only through its offline unit tests.

## Sub-features

- `action-analyze` writes `build.txt`, `provenance.txt`, `report.sqlite`, `report.txt`, and `exit-status.txt` to a fresh evidence dir, and deletes the built binary.
- `action-outputs` appends `evidence=<dir>` and `exit-code=<0|1|2>` to `GITHUB_OUTPUT`. The script itself exits 0 even when the analysis exits 1.
- `action-staged-input` passes `--staged=<true|false>`. Any other value exits 2.
- `action-publisher-offline` runs the publisher's tests against a real snapshot without network access.

## How to get to it (user POV)

- Add `- uses: KellyBennett/Columbo@main` to a workflow. See `README.md`.

## Driving it with columbo-verify

Preconditions:

- Baseline from the [README](./README.md). Run `$V stage "$RUN" tangle tangle` and `mkdir -p "$RUN/action" && : > "$RUN/action/github-output"`.

- **Analyze (staged).** From the repo root, run:
  `PATH="$(cat "$RUN/go-bin"):$PATH" RUNNER_TEMP="$RUN/action" GITHUB_OUTPUT="$RUN/action/github-output" GITHUB_SHA=local GITHUB_ACTION_PATH="$PWD" GITHUB_WORKSPACE="$RUN/subjects" COLUMBO_WORKING_DIRECTORY=tangle COLUMBO_STAGED=true COLUMBO_CONFIG= bash scripts/run-action.sh > "$RUN/action/console.txt" 2>&1`
  The script exits `0`. `github-output` contains `evidence=$RUN/action/columbo-evidence.XXXXXX` and `exit-code=1`. That evidence dir lists `build.txt exit-status.txt provenance.txt report.sqlite report.txt` and has no `columbo` binary.
- **Bad staged input.** Rerun with `COLUMBO_STAGED=maybe`. The script exits `2` and prints `staged must be true or false`.
- **Publisher offline.** Run `COLUMBO_TEST_SNAPSHOT=<a --staged=false --no-history snapshot from fixture-all> python3 -m unittest discover -s scripts -p test_github_check.py`. Expect `OK` (22 tests at the time of writing).
- **Runner tests.** Run `python3 -m unittest discover -s scripts -p test_run_action.py`.

## Gotchas

- Never run `scripts/github-check.py` directly. It POSTs to `api.github.com` with whatever `GH_TOKEN` is in the environment, and it has no dry-run.
- `run-action.sh` rebuilds Columbo from `GITHUB_ACTION_PATH` with `go -C`, so `go` must be on `PATH`. The `$RUN/bin` binary is not used.
- `run-action.sh` calls `sha256sum`. Recent macOS ships it, but older macOS does not. If it is missing, the script stops (`set -e`) before it writes `exit-code`.
- Artifact upload, check publication, and final exit enforcement happen in `action.yml` steps that do not run locally. Verifying them requires a real workflow run on GitHub.
