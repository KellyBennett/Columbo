# Provisional optional cooperating-method source inspection, candidate v1

Status: experimental opt-in navigation. This candidate is not the frozen paired
experiment and has not established autonomous efficacy or cross-corpus benefit.
It changes no default collector, schema, stage, threshold, suppression or fix.
The older frozen instructions remain byte-identical. Review and refactoring
choices still require source reasoning and authorization.

## What this adds

`cmd/columbo-inspect` reuses the existing physical-source package loader, Go type
identities and local call resolver. It emits JSON to stdout, without analyzing
smells or editing source. The Go loader may use its normal build/module caches.
A separate Python 3 SQLite reader binds those facts to an ordinary report and
complete source manifest. Both commands must run with the report's exact Go
version, package patterns, configuration and loader environment. Build the new
command with the repository's Go 1.25.1 requirement. It is ordinary production Go
included in full self-check and full non-test source review, without build tags.

The resulting queue contains every primary and peer association from every
unsuppressed non-PASS ordinary case except prose-comment. It has no length,
complexity, receiver-name, size or rank filter. Excluded cases and associations
remain explicit. Exact source ranges and symbols establish method identity;
canonical Go receiver type identity deduplicates seeds, including aliases and
generic instantiations. A prose-only lead cannot seed itself; an independently
eligible association to the same method remains eligible.

For each receiver seed, keep all source-defined methods in the selected runtime,
including short methods and generated/configuration-excluded supporting context.
The file inventory lists every Go file, including tests, generated files and
non-selected build/package/module variants; those alternatives are never merged
into runtime definitions. Generated status and ordinary-target status are
separate fields. A promoted method is not invented as another source definition.

Declared directly held fields and recursively embedded local storage carry exact
field/type ranges. Held fields do not expand recursively. Repeated embedding
paths remain separate; cycles stop explicitly. Builtin, compound, imported and
unavailable local definitions remain unresolved navigation, not guessed state.
All actual calls in seed methods are listed. A statically resolved selected-local
method adds exactly its target body range, with no recursive expansion of that
body's calls. The receiver-expression and available lexical identifier binding
are source witnesses; neither proves value-alias origin, ownership, dynamic
interface dispatch, return/handle lifetime, mutation or any invariant.

## Reproducible inputs and execution

Keep outputs outside the source root. Never overwrite a prior queue. Freeze the
source before collecting facts and do not edit it while any command is pending.
In controlled experiments, keep all Go/build/loader/analyzer commands under the
existing serialized runner, retaining exact argv, environment, pins, source
before/after manifests, logs, queue time, active time and failures. The generic
commands below do not replace that experiment-specific wrapper.

```sh
CGO_ENABLED=0 go build -o /absolute/evidence/columbo-inspect ./cmd/columbo-inspect
/absolute/evidence/columbo-inspect --root /absolute/source ./... > /absolute/evidence/facts.json
python3 scripts/cooperating-method-inspection-v1/inspect.py \
  --database /absolute/evidence/ordinary.sqlite \
  --facts /absolute/evidence/facts.json \
  --manifest /absolute/evidence/source-manifest.json \
  --binding /absolute/evidence/binding.json \
  --scan-receipt /absolute/evidence/ordinary-receipt.json \
  --output /absolute/evidence/queue.json
```

A source manifest has `files`, an ordered array of `{path, sha256}`, and `sha256`,
the SHA-256 of that array encoded with sorted object keys and compact separators.
It covers the complete source tree excluding `.git` directories. Paths are
relative and symlinks are refused. Other manifest serialization conventions must
be explicitly mapped by exact path/byte equality, never compared as though their
identity strings used the same serialization.

The scan receipt follows the preserved runner contract: command, cwd, environment,
terminal `child_exited` status, actual exit 0 or 1, and complete `source_before`
and `source_after` manifests. A staged report cannot substitute for ordinary
selection. Configuration must be source-manifest-bound; external configuration
files and unfamiliar scan command formats are refused rather than guessed.
`--config` on the facts command is relative to `--root` unless absolute.

The binding JSON contains:

- `database_sha256`, `facts_sha256`, `manifest_sha256`, `scan_receipt_sha256`:
  SHA-256 of those exact input files
- `source_identity`: the manifest's complete identity
- `database_path_at_scan`: the exact output argument in the original scan receipt
- `database_hash_provenance`: `scan-time` only with preserved scan-time evidence,
  otherwise `current-observation`
- `patterns`: exact package pattern array; mechanically compared to scan argv
- `configuration`: the facts' exact decoded config object, with source-file hash
  independently checked against the scan's configuration path and source manifest
- `scope_provenance`: a factual description of where source/report/scope binding
  comes from, including any relocation or manifest serialization correspondence

Schema 8 stores paths/ranges but no file-content hashes. A report hash observed
now is not a scan-time attestation. The external manifest/receipt bindings are
necessary provenance and are not signatures or independent proof of authorship.
Stale files, extra files, mismatched loader environment/configuration, invalid
ranges, malformed/duplicate paths and unbound source are rejected. Nested field,
call, receiver, lexical-binding and target ranges are rehashed too.

## Review obligations and negative lenses

Every seed starts `not-reached`. Inventory generation never marks semantic review
complete. For a reached seed, read every method body, direct/promoted storage and
justified one-hop context; distinguish complete-receiver reach from partial
inspection. Keep all unreached seeds and unresolved relationships visible.

Record exact co-update, read, returned/replacement-handle and selection witnesses,
a concrete coordinated next-change burden, and the smallest ownership alternative
with its API/state/ordering/proof costs. Preserve independent field policies,
cohesive list/accessor lifecycles, subset rather than mirror membership,
heterogeneous borrow/copy and return contracts, first-only versus cardinality
selection, and generic/alias/embedding identity. Similar fields or calls alone
justify no shared owner. A reasoned retention names the rejected alternative and
why it costs more than it buys. `Not-reached` is never a retention.

Characterize unchanged public behavior before authorized source edits. Preserve
ordering, nil/empty distinctions, aliasing, pointer/diagnostic identity, callbacks,
panic boundaries and existing unusual contracts. No universal synchronized
container, replacement-handle repair or required positive refactoring follows
from this aid. Full tests or fewer findings alone do not establish preservation
or ownership benefit.

## Separate optional priority checkpoint contract

Use [the reusable JSON template](../../scripts/cooperating-method-inspection-v1/checkpoint-template.json)
and `checkpoint.py` with the same bound inputs plus `--checkpoint` and
`--evidence-root`. This optional validator is independent of collectors and stage
gates. It requires the preferred candidate, strongest known whole-owner
competitor, exact source/evidence witnesses, competitor rationale, comparison,
selection, characterization state, disposition and next step.

A planned or blocked checkpoint supplies a next action and makes no completed
characterization claim. Attempted/passed/failed characterization additionally
requires existing hashed oracle and execution-receipt artifacts, actual argv,
terminal exit, compatible original source, oracle bytes present in execution
source manifests and unchanged execution source. Added oracle files are allowed
only as explicit evidence context; original checkpoint source must still match.
A missing oracle is rejected even if an unrelated suite exited zero. Each attempt
uses `oracle` and `receipt` objects containing evidence-relative `path` and
`sha256`, plus an `interpretation` string and the characterization's stable
`obligation_id`. Retain every ordered attempt, including failures. Only the latest
completed attempt establishes current status. A changed oracle or argv requires
`replaces: {oracle_sha256, command_sha256}` matching the preceding attempt plus a
`replacement_reason`; command hashes use compact JSON argv encoding. Each attempt
must independently bind the same unchanged original source. Unrelated later
passes cannot silently replace the target obligation. The result preserves all
attempt outcomes and sums reported active/queue costs only when every value is
available. An explicit replacement records a reviewer decision, not proof that
the new oracle/command tests the same semantics.

Structural/binding success does not prove the oracle was selected by the command,
its assertions are adequate, the competitor genuinely is strongest, the chosen
work is highest-value, or behavior was preserved. Those remain source/reviewer
judgments. Shell success cannot substitute for inspecting the actual command,
selected tests and result. No priority scores are generated.

## Evidence boundary and maintenance cost

The motivating exploratory pair found a bounded existing-owner gain: ten
attachment sites were expressed through two attachment variants, plus a detach
operation, with real list/role API coupling and bypass costs. It did not establish
one universal invariant owner. Only 16 of 68 seed receivers were fully reached;
52 remained unreached. Unusual negative contracts and unresolved imported,
dynamic and transitive relationships remain. Coordinator supervision, finite
oracles, unreplicated sequential worker/reviewer variation, unequal work and
shared-cache/order costs prevent efficacy and saved-time claims.

Mechanical HCL/K9s/MinIO checks exercise portability/accounting only, not new
refactoring trials or cross-corpus benefit. This candidate authorizes no PocketBase
continuation. Maintenance includes the explicit Go export surface, JSON binding
contract, read-only SQLite reader and optional checkpoint validator, their
fixtures, and future loader/schema compatibility review. All added production Go
must remain visible in self-check and fresh whole-source review. Do not suppress
new findings to make this aid look cheap.
