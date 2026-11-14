# Calibration ledger design

Status: design for phase 9 of the [mutation calibration plan](mutation-calibration-plan.md).

The kill matrix says how often Columbo notices a mutant. It does not say whether
those mutants look like the cases people meet in real work. Inject operators can
produce a function 40 lines long that any rule notices. Real cases cluster near the
limit. When the plan was written, 19 of the 24 Feature Envy failures on Columbo's own
source exceeded the foreign-access limit by at most one. If evades near the limit
survive more often, an unweighted kill rate overstates what Columbo catches in
practice.

Phase 9 closes that gap without moving private code anywhere. Columbo can keep a
local ledger of the case shapes it has seen. `columbo calibrate --export` turns the
ledger into a small JSON file of counts. The calibration report uses those counts
to weight each operator's kill rate by how often real cases have that shape.

## Data shapes

### Case shape

A case shape is what the ledger knows about one case. It has four fields.

| Field | Values | Source in the snapshot |
|---|---|---|
| `smell` | a smell slug, such as `long-function` | `cases.smell` |
| `verdict` | `FAIL` or `WARN` | `cases.verdict` |
| `suppressed` | true or false | `cases.suppressed` |
| `excess` | `at-limit`, `near`, `over`, `far`, or `unmeasured` | the case's first limited clue |

The shape has no symbol, file, line, case ID, clue subject, or justification. Two
cases in different repositories with the same shape are indistinguishable.

### Excess

`excess` says how far the case is past the point where its rule fires. Columbo
computes it from the case's first clue, by ordinal, that carries a limit. On the
all-smell fixture, every rule's cases carry one. The first is `function-lines` for
Long Function, `parameters` for Long Parameter List, `foreign-accesses` for Feature
Envy, `clump-occurrences` for Data Clump, `duplicate-tokens` for Duplicate Code, and
`dependency-overlap` for Cosmetic Extraction.

1. The trigger is the smallest value that satisfies the clue's comparison. For `>=`,
   the trigger is the limit. For `>` on an integer clue, the trigger is the limit
   plus one. For `>` on a real clue, the trigger is the limit.
2. The ratio is the clue's value divided by the trigger.
3. The band is the first row of this table whose bound the ratio does not exceed.

| Band | Ratio | Long Function at the default limit of 10 |
|---|---|---|
| `at-limit` | at most 1 | 11 lines |
| `near` | at most 1.25 | 12 or 13 lines |
| `over` | at most 2 | 14 to 22 lines |
| `far` | above 2 | 23 lines or more |

A case with no limited clue, a comparison other than `>` or `>=`, or a trigger of
zero or less is `unmeasured`. A ratio is relative, so cases from repositories with
different configured limits still share a scale.

### Ledger row

The ledger has one row per case per repository.

| Column | Type | Meaning |
|---|---|---|
| `repo` | 32 bytes | HMAC-SHA256 of the repository identity, keyed by the ledger secret |
| `case_key` | 32 bytes | HMAC-SHA256 of the case ID, keyed by the ledger secret |
| `smell`, `verdict`, `suppressed`, `excess` | the case shape | `excess` is the band's ordinal, 0 to 4 |

The primary key is `(repo, case_key)`. A second table holds the ledger secret: 32
random bytes, created once with the ledger and never exported. A view,
`ledger_buckets`, groups rows by shape and counts cases and distinct repositories.

### Export document

```json
{
  "format": "columbo-ledger-export",
  "version": 1,
  "floor": {"cases": 5, "repos": 3},
  "excess": [
    {"name": "at-limit", "max_ratio": 1},
    {"name": "near", "max_ratio": 1.25},
    {"name": "over", "max_ratio": 2},
    {"name": "far"}
  ],
  "rules": [
    {
      "smell": "long-function",
      "buckets": [
        {"verdict": "FAIL", "suppressed": false, "excess": "at-limit", "cases": 9, "repos": 4}
      ]
    }
  ]
}
```

A bucket is one `(smell, verdict, suppressed, excess)` cell. Rules sort by smell.
Buckets sort by verdict, then suppressed, then band order. Every string value comes
from a closed vocabulary: the format name, smell slugs, verdicts, and band names.

### Mutant shape

The calibration side needs the same shape for a mutant. A mutant's shape is the
shape of its subject case, which is the case the mutant acts on.

| Class | Subject case |
|---|---|
| inject | the target-smell case the mutant added |
| evade | the target-smell case the parent inject added |
| cure | the target-smell case on the site symbol in the unmutated baseline |

The harness stores each finding's ratio with the finding. The report assigns the
band with the bounds from the export. A mutant with no subject case, such as an
inject that survived, is `unshaped`. The report counts unshaped mutants but leaves
them out of both rates.

### Weighted kill rate

For a target smell `s`, let `w_b` be the export's case count in band `b`, summed over
verdict and suppressed, and let `p_b = w_b / sum(w)`. For a group of mutants of one
class that target `s`, let `n_b` be the scored mutants in band `b` (killed, survived,
or conflict, the same population as the existing `kill_rate`), and `k_b` the killed
share of them.

```text
unweighted = sum(killed_b) / sum(n_b)
weighted   = sum(p_b * k_b for n_b > 0) / sum(p_b for n_b > 0)
covered    = sum(p_b for n_b > 0)
```

A band with `p_b > 0` and `n_b = 0` is uncovered. The report names it and shows the
real weight it carries, so the weighted rate never hides that it covers only part
of the real distribution.

## Decisions

### Record only when the user passes `--ledger`

`columbo --ledger [packages...]` records the analyzed cases in
`~/.columbo/ledger.sqlite`. Without the flag, Columbo writes nothing outside its
snapshot. The flag is a boolean and defaults to false. The GitHub Action never
passes it.

| Option | Why not |
|---|---|
| Always record | Writing to the home directory on every run is a new side effect that nobody asked for. CI runners would collect throwaway ledgers. |
| A key in `.columbo.yml` | The configuration belongs to the repository and is committed. A repository should not decide to write into each contributor's home directory. SPEC's configuration schema also rejects unknown keys, so every older Columbo would fail on the new key. |
| A `COLUMBO_LEDGER` environment variable | Columbo reads no environment variable today apart from the Go build environment, which SPEC treats as an analysis input. An inherited variable would also record mutant copies. The calibration harness runs `columbo` thousands of times on mutated corpora, and every injected case would land in the ledger as a real one. The Action's runner script already uses `COLUMBO_*` names for its own inputs. |
| `--ledger PATH` | A path on every invocation invites typos that split one ledger into two, which breaks deduplication and repository counts. The plan names one location, and one location keeps `calibrate --export` free of arguments. |

The flag form also matches SPEC's CLI rules without new grammar. It precedes the
package patterns, repeated flags take their last value, and `--ledger=wat` exits 2
like any invalid boolean. A user who wants to record all their work can wrap
`columbo` in a script or alias. An environment opt-in can be added later if the
flag proves too easy to forget.

### Read the saved snapshot, not the in-memory report

The ledger reads case shapes from the published snapshot through a sqlc query,
`CaseShapes`, after publication and before the summary. SPEC already requires the
summary to read the saved database. The calibration harness reads the same
snapshot tables, so the ledger and the kill matrix compute excess from the same
evidence.

### Never let the ledger change a verdict

Recording happens after the snapshot is published. If recording fails, Columbo
prints `columbo: ledger not recorded: <reason>` to stderr and keeps the exit status
it would have had. The snapshot and the stdout summary are unchanged. SPEC requires
identical source, toolchain, history, and configuration to produce identical
enforcement results. A full disk or an unwritable home directory must not turn a
pass into a failure.

Exit 2 was the alternative, on the grounds that the user explicitly asked for a
side effect that did not happen. That would make a CI verdict depend on the state
of a home directory, so the warning wins.

### Identify a repository by its root commits

The three-repository floor needs a repository identity that stays on the machine,
and it has to count each repository once. If one repository can produce three keys,
its own cases pass the floor alone, and the export publishes counts that come from a
single repository.

| Option | Why not |
|---|---|
| The module root's absolute path | Two worktrees or clones of one repository would count as two repositories. Columbo's own development uses several worktrees at once. |
| The `go.mod` module path | A repository with three modules, or one module at `/v2` and `/v3`, counts as three repositories. The first version of this design used it, and a review turned 2 cases from one repository into an exported bucket of 6 cases in 3 repositories. |
| The Git remote URL | A clone can have no remote, several remotes, or a renamed one. |
| A random ID stored in the repository | Writing into the analyzed repository is worse than writing into the home directory. |
| A plain hash of any of these | Anyone with the ledger can confirm a guessed module path or commit by hashing it. |

The repository key is HMAC-SHA256 of the repository identity, keyed by the ledger
secret. In a Git work tree, the identity is every root commit reachable from `HEAD`,
from `git rev-list --max-parents=0 HEAD`, sorted. Every clone and worktree of a
repository has the same root commits, and so does every module and major version in
it. Unrelated repositories have different ones. A fork shares its upstream's root
commits and counts as the same repository. That errs toward fewer repositories,
which makes the floor drop more buckets, not fewer.

Two Git states have no usable identity, and recording refuses both with the usual
warning. A shallow clone reports its shallow boundary as its root commits, so its
identity would change with every fetch. A work tree with no commit has no `HEAD`.
Recording either one under a stand-in identity would give the repository a second
key once it had a real one.

Outside Git, or where Git is not installed, the identity is the module path without
its major-version suffix. `module.SplitPathVersion` from `golang.org/x/mod` removes
`/vN` and the `gopkg.in` form `.vN`, so copies of one module and its major versions
share a key. Separate modules in one source tree without Git count as separate
repositories, because nothing outside Git says they belong together. A tree that
later gains Git also gets a new key.

One Git history still splits a repository. Merging an unrelated history adds a root
commit, so the identity changes at that merge, and cases recorded before and after it
count under two keys. Such a merge needs `--allow-unrelated-histories` and is rare.
Keying on one root commit, such as the oldest, would instead merge two repositories
that each imported the same third-party history.

The key protects a ledger that is copied somewhere without its secret. It does not
protect a ledger copied whole, because the secret lives in the same file. The export
is the privacy boundary, and it never contains a key.

### Ask Git for the identity even under `--no-history`

SPEC says that disabled history does not access Git, so that an analysis never
depends on Git when the user turned history off. The repository identity is not
analysis evidence. It never reaches the snapshot, the summary, or the exit status.
It does have to stay the same for every run of one repository.

| Option | Why not |
|---|---|
| Use the module path whenever history is disabled | A user who passes `--no-history` on some runs records each repository under two keys, which is the defect that root commits fix. |
| Refuse to record whenever history is disabled | A repository whose committed configuration sets `history.enabled: false` could never be recorded, and a module outside Git would need history turned on only to watch it fail. |

So whenever `--ledger` is set, recording runs two read-only Git commands in the
module root: `git rev-parse --is-shallow-repository` and
`git rev-list --max-parents=0 HEAD`. SPEC's history section now limits its rule to
history evidence and names this exception. `--ledger` is an explicit opt-in, so the
user who passes it asks for the Git access.

### Refuse links and nonregular files at the ledger path

The snapshot refuses a destination that is a symlink or not a regular file. The
ledger now applies the same rule to `~/.columbo` and `~/.columbo/ledger.sqlite`.
Before, a link at either path made Columbo write through it to wherever it pointed,
and a directory at the ledger path, or a file at `~/.columbo`, reached SQLite, which
reported `out of memory`. Recording and export both check the two paths with
`os.Lstat` before SQLite opens anything, and name the path they refuse.

### Count each case once, with its latest shape

The case key is HMAC-SHA256 of the case ID with the same secret. A run upserts one
row per case, so a repeat run of the same commit writes the same rows, and fifty
runs count once. When a later run sees the same case with a new shape, such as a
function that grew, the row takes the new shape.

The case ID is the right unit because SPEC makes it stable across edits that do not
change the smell, file, symbol, or Data Clump key. A case that the user fixes stays
in the ledger, because it was real work. The alternatives were worse. Keeping one
row per run lets a busy repository dominate. Keeping only each repository's latest
run forgets every case the user fixed, and fixed cases are the ones that show what
work looked like.

### Keep the ledger SQL behind the existing database boundary

`scripts/check-database-boundary.sh` allows raw SQL only in
`internal/snapshotdb/lifecycle.go` and the sqlc output that `sqlc.yaml` generates.
The ledger follows the same rule.

- `internal/snapshotdb/ledger_schema.sql` and `internal/snapshotdb/ledger_queries.sql`
  join the existing sqlc entry. sqlc generates `ledger_queries.sql.go`, and the
  boundary manifest and the lint exclusion list that one file next to the other four.
- `lifecycle.go` gains `RecordLedger` and `OpenLedger`. The ledger file has its own
  SQLite application ID and `user_version`, so a snapshot can never open as a ledger
  or the reverse. A ledger with an unknown version is refused, not migrated.
- The snapshot schema does not change, and its version stays 9.

A second sqlc package, such as `internal/ledgerdb`, would separate the two
databases more cleanly. It would also need a second lifecycle file, a second
manifest entry shape, and new rules in every boundary script and guard test. That
widens the security boundary to save one shared `Querier` interface. The shared
interface is the smaller cost.

### Floor the export in SQL, and publish no totals

`LedgerBuckets` reads the `ledger_buckets` view and keeps only buckets with at least
5 cases and at least 3 distinct repositories. The Go code that writes the export
receives counts only. It never sees a repository key or a case key, so no later
change to the JSON writer can leak one by accident.

The export has no totals: no total per rule, no ledger size, and no count of dropped
buckets. A total would let a reader subtract the kept buckets and recover the count
of a dropped one. For the same reason, the bucket is the full joint cell. Two
separate distributions per rule, one by excess and one by verdict and suppression,
would each sum to the rule total and leak small cells by subtraction.

### Say plainly that the difference between two exports is not protected

Each export meets the floor on its own. Two exports of one ledger, taken at
different times, do not. If a user exports, records one more repository, and exports
again, the difference between the two documents is that repository's contribution,
however small. A bucket that reads 6 cases in 3 repositories and then 8 cases in 4
says that the new repository has 2 cases in that bucket.

| Option | Why not |
|---|---|
| Round every count, for example down to a multiple of 5 | A difference still shows whenever a new repository moves a count across a rounding boundary, so rounding narrows the leak without closing it. It also costs the weights their precision in the buckets just above the floor. |
| Add random noise to every count | Fresh noise on each export averages away over repeated exports. Closing the leak needs a privacy budget spent across every export a user ever shares, which is a differential privacy design and out of scope for phase 9. |

The export stays exact. The `calibrate --help` text, the README, SPEC, and this
document all tell the user to share a single export, and say that the floor does not
protect the difference between two. A user who shares a second export later also
publishes the difference.

### Prove the privacy rule with a test that tries to break it

A Go test builds three fixture modules with distinctive module paths, directory
names, file names, function names, and suppression justifications. It runs Columbo
with `--ledger` on each, exports, and checks the JSON three ways:

- None of the distinctive strings appears.
- No case ID from the fixtures' snapshots appears, and no run of 8 or more
  hexadecimal characters appears.
- Every JSON string value belongs to the closed vocabulary, and every key is a known
  field.

The fixtures produce enough cases that some buckets pass the floor. A test that
exports an empty document would pass vacuously, so the test also requires at least
one bucket. A separate test drives the ledger directly and checks the floor at its
edges: 4 cases are dropped and 5 kept, 2 repositories are dropped and 3 kept.

Another test records layouts that are each one repository next to two unrelated
repositories, and requires each exported bucket to count 3 repositories. The layouts
are three modules in one Git repository, a module moved to `/v2` inside Git, two
clones of one Git repository, three copies of one module outside Git, and one module
outside Git at major versions 1, 2, and 3. Under the module-path key, the first two
and the last fail.

### Define "per operator" as the rule's distribution applied to each operator

The plan asks for aggregates per rule and per operator. Real code carries no
operator, so the export cannot count anything per operator. It counts per rule.
`calibrate report --weights` turns that into a per-operator number. Each operator
targets one smell, so its mutants are weighted by that smell's real distribution.
The report shows a weighted rate for each operator and for each class and smell
across operators.

The weighted rate is most informative for evade and cure operators. An inject kill
follows from the metric crossing its limit, and in `calibration/reports/canonical.md`
every inject operator but one kills 100% of its scored mutants. Evade and cure rates
range from 0% to 100%, and the weighting asks how much of that range depends on
shape.

### Record each finding's ratio in the harness

Today `calibration.sqlite` stores no metric for any finding, so no query can place
a mutant in a band. The harness has to record one number per finding. The change
is additive: a nullable `ratio` column on `baseline_findings` and `mutant_findings`,
read from the same first limited clue, with an `ALTER TABLE` for existing result
databases. The `ALTER TABLE` runs when a run begins, not when the store opens, so
`calibrate report` never writes. It reads a database from before the change, even a
read-only one, as if every ratio were missing. This reaches beyond
`calibration/report`, which phase 8 also edits, so the change stays small and
touches only the finding read and write paths.

The ratio, not the band, goes into `calibration.sqlite`. The bounds come from the
export at report time, so the band table lives in one place, Columbo. The trigger
rule exists twice, once in Columbo's Go and once in the harness's SQL, because the
calibration module cannot import Columbo's internal packages. The export carries a
format version, and the report refuses a version it does not know. A change to the
shape rule has to bump that version, which forces the harness to follow.

### Dispatch `calibrate` before flag parsing

`columbo calibrate --export` is the name the plan uses. Columbo parses flags before
package patterns, so `columbo calibrate` is currently an analysis of the package
pattern `calibrate`. `Run` now checks whether the first argument is exactly
`calibrate` and, if so, runs the subcommand. A module whose import path is literally
`calibrate` can still be analyzed with `columbo -- calibrate` or `columbo .`.

`columbo calibrate` accepts `--export` and `--help` only. It writes the JSON
document to stdout, so the user reads it before sharing it. A missing ledger exits 2
with a message that names the path and the `--ledger` flag.

## SPEC changes

- **CLI and exit codes.** Add `--ledger` to the supported flags and add the
  `columbo calibrate --export` form, with the subcommand rule above.
- **Git history.** Disabled history does not access Git for history evidence.
  `--ledger` recording still runs Git to read the repository identity.
- **One SQLite evidence snapshot.** Say that the calibration export is not a
  findings report. It carries no case, location, or evidence, so the rule that there
  is no JSON export of findings still holds.
- **A new section, Calibration ledger.** State when Columbo records, the ledger
  location and identity, the link rule, the row contents, the repository identity,
  deduplication, the excess definition, the failure rule, the export format, the
  floor, and that the floor does not protect the difference between two exports.
- **Configuration.** No change. The section says so, because a configuration key
  was considered and rejected.

## Checks against SPEC and the boundary

- **Deterministic Core.** The ledger never feeds back into analysis. Recording
  happens after publication, and a failure changes neither the snapshot nor the
  exit status.
- **Evidence Before Interpretation.** Excess comes from a stored clue with its
  stored limit and comparison. Anyone can recompute a case's band from its snapshot.
- **One SQLite evidence snapshot.** The snapshot stays immutable and
  self-contained. The ledger is a separate file with its own application ID. It
  accumulates across runs by design, which is why it is not the snapshot.
- **Database boundary.** All ledger SQL is sqlc input under `internal/snapshotdb`,
  and the only hand-written database code is in `lifecycle.go`. No new `testdata`
  directory, build constraint, or inline lint exclusion is added.

## Open questions

- **Band bounds.** The bounds of 1, 1.25, and 2 come from Columbo's own cases, which
  mostly fall between 1 and 2 times the trigger. A larger ledger may show that `near` needs
  splitting. Changing the bounds changes the export version.
- **Recording by default for one user.** If the flag proves too easy to forget, an
  environment variable or a per-user configuration file could turn recording on.
  Either needs the harness to strip it first.
