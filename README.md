# Columbo 🕵️

*Just one more thing… about that function.*

Columbo is a strict code-smell detective for Go. It follows suspicious code to possible architectural trouble, shows the evidence, and leaves leads for a better design. Long functions, tangled decisions, too many dependencies, and suspiciously tidy helper extractions all deserve a closer look.

Run it locally or let it patrol CI. Configured violations fail the build, giving you—or your coding agent—a case to work on.

## Why Columbo?

Named after the persistent TV detective, Columbo keeps following the clues and asking awkward questions. Splitting a sprawling function into `stepOne`, `stepTwo`, and `stepThree`? Just one more thing: did the design actually improve?

## How to use it

**Coming soon:** this repository currently contains the specification; the CLI isn't implemented yet. Once available, run it from your Go module:

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
