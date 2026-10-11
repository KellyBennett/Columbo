# Optional source inspection

`columbo-inspect` is a provisional, opt-in source-navigation command. It emits
JSON facts to stdout without running smell analysis or editing source. It is not
a default analysis gate, an architectural diagnosis or evidence of refactoring
efficacy. The Go loader can use normal build/module caches.

## Command

Build from the Columbo checkout with Go 1.25.1. Keep output outside the source
root and choose a new destination so shell redirection does not replace evidence:

```sh
CGO_ENABLED=0 go build -o /absolute/evidence/columbo-inspect ./cmd/columbo-inspect
(set -C; /absolute/evidence/columbo-inspect --root /absolute/source ./... > /absolute/evidence/facts.json)
```

`--root` defaults to the working directory. `--config` is relative to that root
unless absolute, and must remain inside it. Symlink source roots are refused.
An absent implicit `.columbo.yml` uses defaults; an explicitly missing file,
read error or invalid configuration fails. Configuration and its hash are
captured from the same read. Later filesystem changes do not update that captured
value; the binding checks below must still reject source drift. Errors exit 2.

Facts include file identities, physical source ranges and hashes, declarations,
receiver types, fields and calls. Selected runtime definitions remain separate
from tests, generated files and nonselected build/package/module variants.
One-hop local method targets and embedded storage are navigation context;
unresolved imported/dynamic/transitive relationships remain explicit. Facts do
not prove value aliasing, ownership, lifetime, mutation or behavior preservation.

## Existing support scripts

The versioned directory name is an existing script interface. Run its scripts
from this Columbo checkout; no Gauntlet runner is required for these commands.
The optional Python binder reads an ordinary SQLite report without changing it:

```sh
python3 scripts/cooperating-method-inspection-v1/inspect.py \
  --database /absolute/evidence/ordinary.sqlite \
  --facts /absolute/evidence/facts.json \
  --manifest /absolute/evidence/source-manifest.json \
  --binding /absolute/evidence/binding.json \
  --scan-receipt /absolute/evidence/ordinary-receipt.json \
  --output /absolute/evidence/queue.json
```

The manifest contains the complete source-tree `files` array of `{path, sha256}`
and its compact, sorted-object-key JSON SHA-256. It excludes `.git` directories,
refuses symlinks and duplicate/unsafe paths, and must match every current file.
The scan receipt must have terminal `child_exited` status, actual exit 0 or 1,
exact argv/cwd/environment, and identical `source_before`/`source_after` manifests.
Staged scans and unrecognized argument forms are refused. Match the exact Go
version, package patterns, configuration and loader environment used for facts.

The binding records `database_sha256`, `facts_sha256`, `manifest_sha256`,
`scan_receipt_sha256`, `source_identity`, `database_path_at_scan`,
`database_hash_provenance`, `patterns`, `configuration` and `scope_provenance`.
Use `scan-time` provenance only with preserved scan-time evidence; otherwise use
`current-observation`. Schema 8 has no source-content hashes, so a current report
hash is not a scan-time attestation or independent proof of authorship.

The binder validates hashes, source membership, physical ranges, scan scope and
configuration before creating a new queue outside the source root. It refuses
an existing queue destination. The queue starts receiver seeds as `not-reached`;
its existence does not certify semantic review or justify a refactoring.

`checkpoint.py` is a separate optional structure/binding validator. It accepts
the same `--database`, `--facts`, `--manifest`, `--binding`, `--scan-receipt`
arguments plus `--checkpoint` and `--evidence-root`, and prints JSON to stdout.
The [input template](../scripts/cooperating-method-inspection-v1/checkpoint-template.json)
and [implementation](../scripts/cooperating-method-inspection-v1/checkpoint.py)
define the accepted fields, ordered attempt/replacement records and hashed
oracle/receipt requirements. Passing it does not adjudicate semantic priority,
oracle adequacy or behavior preservation. It generates no priority score.

## Maintenance checks

```sh
CGO_ENABLED=0 go test ./...
CGO_ENABLED=0 go vet ./...
CGO_ENABLED=0 go build ./cmd/columbo-inspect
python3 -m unittest discover -s scripts/cooperating-method-inspection-v1 -p 'test_*.py' -v
```

Keep the Go facts representation, Python validation, adjacent SQL/template and
regression fixtures compatible. Existing ordinary CLI, Action/CI, collector,
threshold, suppression, schema and stage behavior are independent of this aid.
