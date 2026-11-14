#!/usr/bin/env bash
# Usage: agent-trials.sh <db> [leads|trials|rescore|report|all]
# Plan phase 8, outside CI. Lead checks run no model. Trials run the claude CLI and cost money.
# WORKERS sets agents at once (default 4); OUT keeps each trial's prompt, transcript, and patches.
# rescore replays the fixes stored in the database FROM names, with this checkout's Columbo and scorer, into <db>.
# Columbo is built without the checkout path or commit, so its hash, which every trial key carries, changes only with its source.
set -euo pipefail
here="$(cd "$(dirname "$0")" && pwd)"
root="$(cd "$here/.." && pwd)"
db="$(cd "$(dirname "$1")" && pwd)/$(basename "$1")"
step="${2:-all}"
bin="$(mktemp -d)"
trap 'rm -rf "$bin"' EXIT
(cd "$root" && go build -trimpath -buildvcs=false -o "$bin/columbo" ./cmd/columbo)
(cd "$here" && go build -o "$bin/calibrate" ./cmd/calibrate)
cd "$here"
if [ "$step" = leads ] || [ "$step" = all ]; then
  fixtures=()
  for f in fixtures/*/; do fixtures+=(--corpus "${f%/}"); done
  "$bin/calibrate" trials leads --db "$db" --columbo "$bin/columbo" --validity test "${fixtures[@]}"
  pinned=()
  while read -r module version _license; do
    [ -n "$module" ] && pinned+=(--corpus "$module@$version")
  done < <(grep -v '^#' corpus.txt)
  "$bin/calibrate" trials leads --db "$db" --columbo "$bin/columbo" --validity build "${pinned[@]}"
fi
if [ "$step" = trials ] || [ "$step" = all ]; then
  "$bin/calibrate" trials run --plan trials.tsv --db "$db" --columbo "$bin/columbo" --workers "${WORKERS:-4}" ${OUT:+--out "$OUT"}
fi
if [ "$step" = rescore ]; then
  "$bin/calibrate" trials rescore --from "${FROM:?set FROM to the database holding the stored trials}" --plan trials.tsv --db "$db" --columbo "$bin/columbo" --workers "${WORKERS:-4}"
fi
if [ "$step" = report ] || [ "$step" = all ]; then
  "$bin/calibrate" trials report --db "$db" > reports/agent-trials.md
  "$bin/calibrate" trials export --db "$db" --table trials --format tsv > reports/agent-trials.tsv
  "$bin/calibrate" trials export --db "$db" --table trials --format jsonl > reports/agent-trials.jsonl
  "$bin/calibrate" trials export --db "$db" --table lead_checks --format tsv > reports/lead-checks.tsv
fi
