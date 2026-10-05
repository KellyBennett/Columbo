# Columbo 🕵️

*Just one more thing… about that function.*

Columbo is a strict code-smell detective for Go. It catches signs of architectural trouble, explains the evidence, and gives you—or your coding agent—leads for a better design.

## Why Columbo?

Getting code to pass a linter isn't the same as making it easier to understand and change. A sprawling function can become five tiny helpers while keeping all the same tangled responsibilities.

Columbo keeps asking the awkward question: did the design actually improve?

Its philosophy is simple:

- **Better design, not smaller numbers.** Long functions, tangled decisions, and too many dependencies are clues to investigate
- **Show the evidence.** Findings explain what Columbo saw, why it matters, and where to look next
- **Make CI feedback useful.** A failing build sends the developer or agent back for another design pass, with concrete leads instead of a score to game

Columbo's checks run without an LLM. They're deliberately strict; a finding is a reason to investigate, not proof that there's only one right design.

## Set it up

Install with Go 1.25.1, then run from your Go module:

```sh
CGO_ENABLED=0 go install github.com/KellyBennett/Columbo/cmd/columbo@latest
columbo ./...
```

Columbo currently analyzes production Go code only. Files ending in `_test.go` are excluded from findings and from the references, interfaces, and suppressions used as evidence. Test-specific rules may be added later.

No configuration is required. To change thresholds or choose which smells fail, warn, or stay off, add [`.columbo.yml`](SPEC.md#configuration).

### In CI

For GitHub Actions, save this as `.github/workflows/columbo.yml`:

```yaml
name: Columbo
on: [push, pull_request]
permissions:
  contents: read
jobs:
  columbo:
    permissions:
      contents: read
      checks: write
    runs-on: ubuntu-24.04
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
          persist-credentials: false
      - uses: KellyBennett/Columbo@main
```

The action builds Columbo from its own revision, analyzes all Go packages, publishes findings, and uploads the log, SQLite report, build provenance, and publication status as `columbo-evidence`. Findings and upload failures fail the job after evidence is retained. It selects Go from your module's `go.mod`.

Use `@main` to dogfood the latest Columbo, or pin the action to a commit for reproducible CI. Optional inputs are `working-directory`, `config`, `go-version`, `artifact-name`, and check `name`. For multiple invocations, give each a different artifact name. Other CI systems can use the CLI and retain its exit status and report.

The publisher action reads the completed SQLite snapshot and creates a separate **Columbo findings** check on the PR head commit. It sends every unsuppressed finding in batches of at most 50, including file/line ranges, case IDs, stored verdicts, metrics, diagnoses, leads and required policy reviews. FAIL becomes failure, WARN becomes warning. Suppressed cases remain in the evidence. An analysis failure (exit 2) creates a failing check without reading a stale report. Publication errors also fail CI and record the confirmed annotation count; the action never claims a partial upload is complete.

This uses GitHub's Checks API rather than the ten-error-per-step workflow-command channel. Grant `checks: write` only to the publishing job. The default token is the workflow's `github.token`; no personal token is needed. The adapter requires Python 3 on GitHub.com runners. Fork PR tokens may lack write permission; use a trusted publishing workflow rather than giving untrusted PR code a stronger token. Check annotations appear in the check and, where GitHub can display them, beside code in **Files changed**. They are not posted review comments or approvals.

The full log and SQLite artifact retain all evidence.

Exit `0` means no unsuppressed failing findings, `1` means findings need attention, and `2` means the run failed. Each completed analysis saves a fresh SQLite evidence file and prints its location. Keep that file even when findings fail CI; the [query guide](docs/sqlite-schema.md) explains how to investigate it.

[Contributing and development](docs/development.md) · [Full specification](SPEC.md)
