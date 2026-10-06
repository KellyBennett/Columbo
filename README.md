# Columbo 🕵️

*Just one more thing… about that function.*

**Agent-first architectural enforcement. Strict on purpose.**

**Put Columbo in your CI to get yourself out of the code review loop.** Its goal is to mechanically close off structural paths by which coupling can spread through agent-written code.

Columbo supplies your agents with evidence and leads for investigating each finding, so they can understand what triggered it, what architectural pressure the rule is protecting against, and how to change the design rather than merely hide the evidence.

## Why Columbo?

Agentic coding changes the economics of architectural enforcement. Refactoring is cheaper, so CI can afford to reject structural shortcuts that a human team might reasonably tolerate in isolation.

Columbo does not need to prove that every failing occurrence is already causing pain. A FAIL means the source contains a structural form that the configured policy has chosen not to permit. The local code may be perfectly understandable; the policy can still reject it because allowing that shape leaves a coupling vector available to accumulate across the system.

Each finding separates the mechanical evidence Columbo can prove from the architectural diagnosis it suggests. The evidence determines whether the configured invariant was crossed. The diagnosis and leads help the agent remove the coupling vector instead of making the detector unable to see it.

That creates a tight feedback loop:

1. The agent writes code.
2. Columbo detects a prohibited structural shape and fails CI.
3. Columbo gives the agent evidence, architectural context, and leads.
4. The agent removes the coupling vector and reruns the check.

Columbo is strict on purpose. A particular occurrence can still be defensible, but "this instance seems harmless" is not enough to defeat a system-wide invariant. Suppressions exist for genuine exceptions; they stay exceptional.

The goal is to let you delegate implementation while CI continuously applies architectural pressure toward code that is easier to change.

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

Prose comments fail by default; recognized machine directives are allowed. Protect intentional omissions with regression tests. See the [comment policy](docs/prose-comments.md).

For an opt-in, non-enforcing investigation of repeated choice-set construction, see [experimental choice-set evidence](docs/choice-set-evidence.md).

[Configuration and rules](SPEC.md) · [Evidence query guide](docs/sqlite-schema.md) · [Development](docs/development.md)

## Supported Languages

- Go

### Coming Soon

- JavaScript
- Python
- Ruby
