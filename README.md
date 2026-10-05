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
    runs-on: ubuntu-24.04
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25.1'
      - name: Install Columbo
        run: CGO_ENABLED=0 go install github.com/KellyBennett/Columbo/cmd/columbo@latest
      - name: Investigate
        run: columbo ./...
      - name: Keep the evidence
        if: always()
        uses: actions/upload-artifact@v4
        with:
          name: columbo-evidence
          path: columbo-*.sqlite
          if-no-files-found: warn
```

For reproducible CI, replace `@latest` with the Columbo version or commit you want to use. Other CI systems can run the same install and analysis commands; keep Columbo's exit status so findings fail the build.

In GitHub Actions, Columbo automatically adds file-and-line annotations to the check, including the PR's **Files changed** view where GitHub can display them. FAIL findings become errors, WARN findings become warnings, and suppressed cases stay in the summary and evidence file. Each annotation includes the case ID, symbol, stored metrics and comparisons, and any required policy review. These are check annotations, not posted review comments or approvals; the existing `contents: read` permission is sufficient.

Use `--github-annotations` to enable annotations explicitly or `--github-annotations=false` to disable them. Local runs keep the ordinary summary by default. GitHub may limit how many annotations it displays; the full summary and SQLite artifact retain every finding.

Exit `0` means no unsuppressed failing findings, `1` means findings need attention, and `2` means the run failed. Each completed analysis saves a fresh SQLite evidence file and prints its location. Keep that file even when findings fail CI; the [query guide](docs/sqlite-schema.md) explains how to investigate it.

[Contributing and development](docs/development.md) · [Full specification](SPEC.md)
