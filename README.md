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
      - uses: actions/checkout@v7
        with:
          fetch-depth: 0
          persist-credentials: false
      - uses: KellyBennett/Columbo@main
```

The action publishes findings as check annotations and retains the full evidence as an artifact. A failing analysis blocks CI. Pin the action to a commit for reproducible runs.

You can also run Columbo locally to investigate findings before pushing:

Install with Go 1.25.1 and run from your Go module:

```sh
go install github.com/KellyBennett/Columbo/cmd/columbo@latest
columbo ./...
```

No configuration is required. Columbo currently analyzes production code; test files are excluded.

### Pull request feedback

Columbo can also comment on pull requests so your team can judge its findings during review. Turn it on with `feedback: true` and grant `pull-requests: write`:

```yaml
name: Columbo
on: pull_request
permissions:
  contents: read
jobs:
  columbo:
    permissions:
      contents: read
      checks: write
      pull-requests: write
    runs-on: ubuntu-24.04
    steps:
      - uses: actions/checkout@v7
      - uses: KellyBennett/Columbo@main
        with:
          feedback: true
```

The default checkout is enough. Columbo fetches the base commit by its SHA with the checkout's credentials, so keep `persist-credentials` at its default in a private repository.

Columbo analyzes the pull request's base and head with the head's configuration. It comments only on findings the pull request introduced or made worse. Existing findings in code the pull request didn't touch stay quiet.

Reviewers see one comment per finding, on the first changed line of the code it points at. Each comment says what Columbo found and why. React 👍 if the finding is right and 👎 if it's a nitpick.

When a later push clears a finding, Columbo replies on its thread and resolves it. The comment and its reactions stay. If the finding comes back, the thread reopens.

GitHub lets the workflow token resolve threads only when the job also has `contents: write`. Grant it if you want cleared threads collapsed. Without it, Columbo still replies "Cleared" once and the run logs a warning; the thread is resolved on the first later run that has the permission.

Feedback is advisory. It never changes the check or fails the job. Pull requests from forks are skipped with a warning, because their token can't comment. If the base commit can't be analyzed, nothing is posted and the check summary says so.

## Supported Languages

- Go

### Coming Soon

- JavaScript
- Python
- Ruby
