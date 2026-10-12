# Optional source inspection

Run from the Go module root with a fresh output path outside that root:

```sh
columbo --inspect-source --no-history --output /absolute/evidence/report.sqlite ./...
sqlite3 -readonly /absolute/evidence/report.sqlite < docs/sqlite/source-navigation.sql
```

`--inspect-source` adds source navigation to the same SQLite snapshot as ordinary
findings. Both use one package load, configuration and transaction. It also works
with `--staged`; the stored ordinary verdicts and stage selection are unchanged.
Default scans do not collect this optional inventory. Exit codes, read-only
readers and atomic no-clobber publication follow the ordinary CLI contract.

## Read the evidence

- `source_receiver_seeds` links unsuppressed, non-prose findings to their primary
  and supporting instance methods inside the configured ordinary target scope.
  Roles and case IDs are retained; repeated primary links are deduplicated.
- `source_receiver_siblings` lists every selected-runtime source-defined method
  of those receivers, including short methods and generated/excluded supporting
  context. Aliases and generic receivers resolve to their underlying named type;
  promoted methods are not invented.
- `source_storage_paths` follows declared local embedding, preserving distinct
  paths through a diamond and stopping at cycles. Held fields are terminal.
  Missing targets and builtin, compound or imported types stay explicit.
  `source_field_names` supplies each field's declared names; embedded fields use
  their stored type spelling.
- `source_one_hop_calls` lists calls from sibling bodies with static local method
  targets where available, including target ranges and hashes. Unresolved calls
  remain visible. Receiver and lexical-binding ranges can be joined through
  `source_ranges`. Target bodies are navigation context, not recursive expansion.

The [published navigation queries](sqlite/source-navigation.sql) run against
saved snapshots without reading current source or re-running analysis. Their
empty result on an ordinary snapshot means inspection was not collected; check
`source_scan` to distinguish that from a collected scan with no eligible seeds.
No queue or separate review-status file is needed to discover these relationships.

## Identity and limits

`source_scan` records the absolute root, running Columbo Go version and source
identity. `source_patterns`, `source_environment` and the typed `source_config*`
tables retain package patterns, inherited loader environment and effective
configuration, including staging, history, exclusions and threshold values.
The configuration path must remain inside the root; its hash comes from the same
read that was decoded. A missing implicit `.columbo.yml` uses defaults and a
null file/hash. An explicitly missing or invalid configuration fails.

`source_files` stores SHA-256 and size for every regular file below the root,
excluding `.git` directories, with selected-runtime, ordinary-target and generated
scope distinguished. Tests and nonselected build/package/module variants never
become runtime declarations. The complete path/hash inventory is compared before
loading and after analysis; durable source/configuration drift fails without
publishing. Symlinks and nonregular inputs are refused. Keep build outputs and
mutable caches outside the source root. Earlier snapshots are never overwritten.

The source identity is SHA-256 over sorted inventory entries encoded as
`<UTF-8 path byte length>:<path>:<lowercase file SHA-256>\n`. Declaration, type,
field, call, receiver and binding ranges use physical, half-open byte offsets and
SHA-256 of the exact captured range. Declaration locations reuse the ordinary
`declarations` rows. Compare stored file hashes with current bytes before using
a saved snapshot to edit source.

These are in-process observations, not signatures or an attestation of external
module/toolchain caches. They do not prove alias origin, interface dispatch,
mutation, ownership, lifetime, behavioral equivalence or better architecture.
Read source and test any proposed refactoring. Inspection does not add a gate,
priority score or completion claim.
