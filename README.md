# Columbo 🕵️

*Just one more thing… about that function.*

Columbo is a strict code-smell detective for Go. It follows suspicious code to possible architectural trouble, shows the evidence, and leaves leads for a better design. Long functions, tangled decisions, too many dependencies, and suspiciously tidy helper extractions all deserve a closer look.

Run it locally or let it patrol CI. Configured violations fail the build, giving you—or your coding agent—a case to work on.

## Why Columbo?

Named after the persistent TV detective, Columbo keeps following the clues and asking awkward questions. Splitting a sprawling function into `stepOne`, `stepTwo`, and `stepThree`? Just one more thing: did the design actually improve?

## How to use it

Build with Go 1.25.1, then run from the Go module you want to investigate:

```bash
go install github.com/KellyBennett/Columbo/cmd/columbo@latest
# Or, from a checkout:
go build -o columbo ./cmd/columbo
```

```bash
# Investigate all packages
columbo

# Focus on one part of the project
columbo ./internal/orders/...

# Get a report your tools can read
columbo --format json ./...
```

Use `.columbo.yml` to adjust thresholds and choose which smells warn, fail, or stay off. Each case includes evidence, a possible diagnosis, refactoring leads, and shortcuts to avoid. Follow the leads, improve the design, and run it again.

In CI, `0` means no failing cases, `1` means there's a case to solve, and `2` means something prevented the investigation.

Curious about the full case file? See [SPEC.md](SPEC.md).

## Development

```bash
go test ./...
go vet ./...
```

Acceptance tests pin Go 1.25.1, Linux/amd64, and a fixed build environment. The suite includes all seven default FAIL smells and checked-in text/JSON goldens. Refresh goldens deliberately with `UPDATE_GOLDEN=1 go test ./internal/columbo`.

CE-001 is a provisional dogfooding policy. Ordinary builds and tests support it; `go run ./cmd/releasecheck` deliberately fails until the policy is resolved before public v1 release.

Cgo inputs are rejected with exit 2 when the loader cannot establish original physical-source/type correspondence; generated compiler wrappers never substitute for source receipts.

