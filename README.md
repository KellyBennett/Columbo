# Columbo 🕵️

*Just one more thing… about that function.*

**Agent-first static analysis. Strict on purpose.**

**Put Columbo in your CI to get yourself out of the code review loop.** Its goal is to stop you having to inspect every agent-written change for coupling, tangled responsibilities, and decisions that will make your codebase harder to change.

Columbo supplies your agents with evidence and leads for investigating each finding, so they can understand what triggered it, where to look next, and how to improve the design.

## Why Columbo?

When your agents write code that couples unrelated responsibilities, tangles dependencies, or makes the codebase harder to change and maintain over time, Columbo finds it.

Each finding tells the agent what coupling risk Columbo detected, shows the concrete evidence behind it, identifies the code involved, and points to where to investigate next. Instead of spending tokens rediscovering the shape of the problem from scratch, the agent gets a focused starting point for understanding the design issue, changing the code, and running the check again.

That creates a tight feedback loop:

1. The agent writes code.
2. Columbo catches suspicious structure and fails CI.
3. Columbo gives the agent evidence and context for the finding.
4. The agent improves the design and reruns the check.

Columbo is strict on purpose. A finding can still be defensible, and an agent can make that case. But exceptions stay exceptional.

The goal is to let you delegate implementation without personally reconstructing the design problems in every agent-written change.

## Use it

To run it in GitHub Actions:

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

The action publishes findings as check annotations and retains the full evidence as an artifact. Failing findings block CI. Pin the action to a commit for reproducible runs.

You can also run Columbo locally to investigate findings before pushing:

Install with Go 1.25.1 and run from your Go module:

```sh
go install github.com/KellyBennett/Columbo/cmd/columbo@latest
columbo ./...
```

No configuration is required. Columbo currently analyzes production code; test files are excluded.

[Configuration and rules](SPEC.md) · [Evidence query guide](docs/sqlite-schema.md) · [Development](docs/development.md)

## Supported Languages

- Go

### Coming Soon

- JavaScript
- Python
- Ruby
