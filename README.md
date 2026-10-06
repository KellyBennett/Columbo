# Columbo 🕵️

*Just one more thing… about that function.*

**Agent-first static analysis. Strict on purpose.**

**Put Columbo in your CI to get yourself out of the code review loop.** Its goal is to stop you having to inspect every agent-written change for coupling, tangled responsibilities, and decisions that will make your codebase harder to change.

Columbo supplies your agents with evidence and leads for investigating each finding, so they can understand what triggered it, where to look next, and how to improve the design.

## Why Columbo?

A normal linter on a human-run project usually stays out of your way. Columbo deliberately gets in the way. Humans will hate working in your codebase. Your agents can handle it.

Writing more rules in `CLAUDE.md` won't reliably make an agent get the design right up front. You need a feedback loop:

1. The agent writes code.
2. Columbo catches structural problems and fails CI.
3. The agent investigates the evidence, improves the design, and runs it again.

Columbo gives agents little leeway. It flags suspicious code even when that code might be defensible. Its checks run without an LLM.

An agent can argue that a finding would make the design worse. The human driving it decides whether to grant an exception. Exceptions should stay exceptional.

Columbo is one piece of a development workflow that lets you delegate implementation without personally policing the design of every change.

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
CGO_ENABLED=0 go install github.com/KellyBennett/Columbo/cmd/columbo@latest
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
