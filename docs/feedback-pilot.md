# Investigating findings with a pilot

The pilot tests whether following Columbo's leads produces less coupling or better cohesion, and records the cost of the change. Existing code can be reasonable and still offer an opportunity under Columbo's deliberately strict policy. A thumbs-down reaction does not close the investigation.

## Install the tools

Install both commands from `main` to include the merged pilot changes. You need Go 1.25.1 or later, Git, a GitHub login that can read the work repository, and a configured agent. The default agent is the Claude Code CLI.

```bash
go install github.com/KellyBennett/Columbo/cmd/columbo@main
go install github.com/KellyBennett/Columbo/cmd/columbo-feedback@main
```

Ensure the Go installation's bin directory is on `PATH`, or pass the analyzer's absolute path with `--columbo`. The pilot records executable hashes and the requested model, so installing a different executable requires a new pilot output directory.

## Collect the evidence

Use a full local clone of the work repository. Your current branch and uncommitted changes are left alone. Git evidence comes from committed objects, not uncommitted files.

```bash
columbo-feedback collect \
  --repo your-org/your-repo \
  --checkout /path/to/work-repo \
  --artifacts
```

`GH_TOKEN` takes precedence over `gh auth token`. GitHub Enterprise uses `--api-url https://github.example.com/api/v3`, or `GITHUB_API_URL`. Reading Actions artifacts requires permission to read Actions as well as the repository's PRs. Missing optional permissions become recorded evidence gaps.

The SQLite store defaults to `~/.columbo/feedback.sqlite`. Git repositories and downloaded ZIP archives default to `~/.columbo/feedback.sqlite.evidence`. Override them with `--store` and `--evidence`. Use one `--repo` per invocation when specifying `--checkout`. Collect other repositories separately. Repeat `--pr 123` on `collect` to gather specific PRs without scanning repository-wide comments. A targeted collection does not advance the repository discovery cursor.

Every successful PR collection appends an observation containing the captured API responses, review submissions, original and current comment locations, replies, reactions, thread state, timeline, commit details, and evidence gaps. The current tables still track today's state for `status`. Earlier observations survive edited comments, withdrawn reactions, and rewritten PR histories. Existing version 1 stores migrate automatically, but their unobserved earlier history cannot be recovered.

The collector retains relevant revisions in an independent bare Git repository and pins each available commit against garbage collection. It tries the local clone first and its origin for missing objects. A shallow clone or an unavailable force-pushed commit remains a documented limit. Collect again before starting a new pilot to capture later team changes.

With `--artifacts`, the collector downloads artifacts whose names contain `columbo`, up to 128 MiB each. Original snapshots and other outputs remain in their ZIP archives. Expired or inaccessible artifacts are recorded as gaps. New feedback-enabled CI runs also retain `feedback-provenance.json`, with the checked-out revision, PR head, analyzer hash and version, configuration, Go version, and case IDs. A review anchor alone does not establish the analyzed revision.

## Run a small pilot

Start with a few cases to check that the agent and project validation work in your environment:

```bash
columbo-feedback pilot \
  --repo your-org/your-repo \
  --out /path/to/pilot \
  --limit 3 \
  --allow-reconstructed
```

`--allow-reconstructed` permits trials at review-anchor revisions when original analysis provenance is missing or ambiguous. Your existing week of feedback will usually need it. Such trials remain labeled as reconstructions. The earliest collected comment may already have been edited, so the agent must verify its receipts against the starting code. Without the flag, those cases are excluded with an explanation in `plan.json`.

The sample rotates through thumbs-down, thumbs-up, unreacted, mixed, and other reaction groups, and through rules within each group. In actual-changes groups, findings with later changes to the flagged file take priority across rules. The report counts all eligible candidates with relevant changes, no response, or unavailable evidence before the sample limit. `--seed` makes selection repeatable. This deliberately balanced sample does not estimate population-wide usefulness. Use `--pr 123` or repeat `--case C-...` to choose specific findings. Cases are investigated separately, even when they overlap.

The default checks are `go build ./...`, `go vet ./...`, and `go test` on the affected packages. The checks run against the original and available later PR head, and against an experimental candidate for thumbs-down findings. Repeat `--check` to replace the defaults with your project's commands, which run verbatim. Commands are interpreted by `/bin/sh`. Use `--module path/to/module` for a Go module below the repository root.

The trial's change set is the files changed by the experimental candidate and by the later PR head, both compared with the starting revision. The affected packages are the packages containing changed Go files, plus every package in the module that imports them, directly or indirectly, including through tests. Each revision resolves the same change set against its own package graph, so the original, candidate and later code are tested on comparable packages. Build and vet stay module-wide. Tests run on the whole module when the change set touches `go.mod`, `go.sum`, `go.work`, a file outside the module, or any non-Go file such as embedded files or test data, and when `go list` fails or no package matches. Use `--full-tests` to always test the whole module. `result.json` records the changed paths and each revision's test scope with its reason, and the report names the covered packages. Passing tests in affected packages says nothing about packages outside that set.

The runner freezes the starting Columbo configuration and uses its exact contents for all comparisons. It prefers captured CI configuration when available, then the starting checkout's `.columbo.yml`, then the defaults. Use `--config settings.yml` for another configuration file within the module. Each reconstructed snapshot records the frozen configuration. Changing thresholds in later code therefore cannot masquerade as a design improvement.

```bash
columbo-feedback pilot \
  --repo your-org/your-repo \
  --out /path/to/pilot \
  --allow-reconstructed \
  --limit 3 \
  --check 'go build ./...' \
  --check 'go vet ./...' \
  --check 'go test ./...'
```

When the starting and later revisions have identical files, or the later diff does not touch the flagged file, an actual-changes trial records `no-response` without calling an agent or running checks. This records inaction rather than a negative design judgment. Missing later revisions, unavailable diffs, and missing flagged-file locations remain evidence gaps. Thumbs-down exploration still runs when the team made no response.

Each remaining trial first records a design assessment without supplying reactions or later code. The agent reads the full starting codebase and chooses a hypothetical future change as a criterion for comparing designs.

For thumbs-up findings, the agent evaluates the team's actual changes against the original code. It does not propose or implement alternative refactorings. The runner checks the original and actual code with the same validation commands and frozen Columbo configuration. Missing later code or failed checks remain explicit limits on the assessment.

Only thumbs-down findings receive exploratory refactoring, with up to two attempts by default. The runner independently runs validation and Columbo, preserving failing attempts too. A fresh agent invocation then receives the discussion and later code to assess the finding, suggested direction, experiment, and team response separately. Mixed, unreacted, and other reaction groups also use actual changes only. An absent experiment is recorded as not applicable, rather than a failed experiment.

Each phase receives a fresh agent context. Withholding later evidence is an experimental procedure, not a filesystem security boundary. Default read-only phases are checked for source edits. The trial checkouts have only the starting revision's reachable history until the review phase.

`--attempts`, `--turns`, `--timeout`, and `--check-timeout` bound effort. Defaults are two refactoring attempts, thirty turns per default Claude invocation, ten minutes per agent invocation, and five minutes per check or analyzer invocation. Failed checks do not turn an unverified change into a demonstrated benefit. A failed attempt does not establish that no useful refactoring exists.

Trials run one at a time by default. `--parallel 3` runs up to three trials at once, each in its own checkouts and execution directory, and prefixes progress lines with the trial's label. Project checks and agents then run concurrently. Test suites that share external resources, such as a database or fixed ports, must tolerate that, or keep `--parallel 1`. Interrupting the pilot stops every running trial. The runner writes `report.md` once all trials finish or stop. `--parallel` is not part of the frozen settings, so a resumed pilot may use a different value.

The runner records policy changes and test edits for review. A candidate that changes `.columbo.yml` or adds a suppression cannot be marked as a verified behavior-preserving candidate. The reviewer must examine changed assertions for weakened tests. Passing tests supports behavior preservation only within their coverage.

## Configure the agent

The default command uses Claude Code's safe and restricted modes, fresh sessions, and phase-specific tools. Refactoring permits file edits and Go build, vet, test, and Columbo commands. Assessment and final review use read-only tools. These phases request schema-validated JSON from Claude Code and read its `structured_output` result. Review prompts provide exact permitted commands for the available checkouts. The git allowlist permits show, log, and diff in the current directory and with each available checkout path. `--model` selects a model. Your existing Anthropic gateway or supported cloud-provider configuration can be supplied through the inherited environment. Use repeated `--pass-env VARIABLE_NAME` flags for additional gateway, cloud-provider, or project variables. The runner records their names, but does not record their values as configuration. Command output is retained in logs.

For another agent or a company wrapper, supply an executable and repeated arguments. Providing any `--agent-arg` replaces the default Claude arguments. The runner replaces `{phase}`, `{model}`, and `{turns}` within each argument, supplies the prompt on stdin, and sets the working directory to the disposable checkout. Assessment and review must return the JSON requested in their prompts. The runner also accepts a JSON object within prose or the last complete JSON code fence. A parse or validation failure gets one fresh invocation with the error and required schema. Both attempts retain their prompts and outputs. Process failures and cancellation do not trigger that format retry. Refactoring may return plain text.

```bash
columbo-feedback pilot \
  --repo your-org/your-repo \
  --out /path/to/pilot \
  --allow-reconstructed \
  --agent /path/to/company-agent \
  --agent-arg '{phase}' \
  --agent-arg '{model}' \
  --model your-company-model
```

Agent invocations use the configured service and may incur its normal charges. Checkouts keep experiments separate from your working tree, while builds, tests, and agents still execute with your account's permissions.

## Review and resume

Open `report.md` in the output directory. It separates reviewer reactions from agent judgments and links to each trial's `result.json`. The design judgments are beneficial, tradeoff, no demonstrated benefit, or inconclusive. The separate `no-response` team outcome means there was no relevant later code change to judge. They are assessments for human review, not established ground truth.

`plan.json` freezes the selected evidence and settings. Trial execution directories contain prompts, stdout, stderr, initial assessments, candidate patches for every attempt, validation commands and timings, reconstructed Columbo snapshots and case lists, collected discussion, and the later PR code when available. Original CI artifacts remain in the evidence directory. Preserve the store, evidence directory, and pilot output together when bringing the results back for further analysis. Paths are absolute, so resume on the same machine and directory layout.

Rerun the same command and output path to resume. Completed trials are skipped. Pilot plans from earlier policies require a new output directory because sampling, no-response reporting and test scoping have changed. Their results remain available without being silently reused. Incomplete trials restart in a new execution directory, preserving previous attempts and logs. A lock prevents simultaneous runs in one output directory. Changes to settings, executable hashes, or the evidence sample require a new output directory.

During review, inspect whether a change removed or narrowed a coupling capability, merely moved it, or introduced a new cost. Clearing the finding is supporting evidence. A later improvement alone does not establish that Columbo caused it, and unavailable history remains unavailable.
