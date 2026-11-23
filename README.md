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

Reviewers see one comment per finding, on the first changed line of the code it points at. Each comment says what Columbo found and why, and offers leads as places to start looking rather than a prescribed fix. React 👍 if the finding led to a real design problem, even one the leads didn't name, and 👎 only if the evidence is wrong or there's nothing worth changing underneath it.

When a later push clears a finding, Columbo replies on its thread and resolves it. The comment and its reactions stay. If the finding comes back, the thread reopens.

GitHub lets the workflow token resolve threads only when the job also has `contents: write`. Grant it if you want cleared threads collapsed. Without it, Columbo still replies "Cleared" once and the run logs a warning; the thread is resolved on the first later run that has the permission.

Feedback is advisory. It never changes the check or fails the job. Pull requests from forks are skipped with a warning, because their token can't comment. If the base commit can't be analyzed, nothing is posted and the check summary says so.

#### Reading the feedback

`columbo-feedback` gathers your team's reactions and replies into a SQLite file on your machine. Reactions describe reviewer responses, rather than establishing whether a stricter design would help. Collection only reads from GitHub. The optional pilot runner sends code and evidence to the agent you configure.

```bash
go install github.com/KellyBennett/Columbo/cmd/columbo-feedback@latest
columbo-feedback collect --repo your-org/your-repo
columbo-feedback status
```

```text
rule                 👍  👎  missed
feature-envy          0   0       1
long-parameter-list   4   1       0
no rule               0   0       2
```

`collect` saves every pull request Columbo has commented on, including closed pull requests. It captures comments, reactions, all current PR commits and their patches, reviews, thread resolution, timeline events, and CI artifact metadata. Each collection preserves an observation, so earlier observed comments and reactions survive later changes. `status` uses the latest state. Repeat `--repo` for more repositories. The store defaults to `~/.columbo/feedback.sqlite`; choose another with `--store`. For GitHub Enterprise, pass `--api-url` or set `GITHUB_API_URL`.

`collect` needs a token that can read the repository's pull requests. It uses `GH_TOKEN` if set, otherwise `gh auth token`.

`status` counts 👍 and 👎 per rule on Columbo's finding comments. To tell Columbo it missed something, start a pull request comment with `Columbo missed:` and name the rule, for example `Columbo missed: feature-envy in Parse`. Misses that name no rule are counted under "no rule".

To investigate whether following a finding produces a design with less coupling or better cohesion, retain Git history and run a pilot:

```bash
columbo-feedback collect --repo your-org/your-repo --checkout /path/to/work-repo --artifacts
columbo-feedback pilot --repo your-org/your-repo --out /path/to/pilot --limit 3 --allow-reconstructed
```

The pilot tries and tests refactorings, including for thumbs-down findings, in separate checkouts. It records an initial design assessment before providing reviewer reactions and later code. Read [the pilot instructions](docs/feedback-pilot.md) for installation from this PR, agent configuration, evidence limits, and the resulting reports.

## Supported Languages

- Go

### Coming Soon

- JavaScript
- Python
- Ruby
