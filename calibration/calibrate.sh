#!/usr/bin/env bash
# Usage: calibrate.sh <db> <operators> [max-sites] [columbo-config]
# Measures Columbo's own module and every module in corpus.txt into one calibration.sqlite.
# External corpora use build validity (go vet) so no downloaded code is executed.
# COLUMBO_BIN measures a prebuilt columbo, such as one from a proposal branch.
set -euo pipefail
here="$(cd "$(dirname "$0")" && pwd)"
root="$(cd "$here/.." && pwd)"
db="$(cd "$(dirname "$1")" && pwd)/$(basename "$1")"
ops="$2"
max="${3:-40}"
config=()
if [ -n "${4:-}" ]; then config=(--config "$(cd "$(dirname "$4")" && pwd)/$(basename "$4")"); fi
bin="$(mktemp -d)"
trap 'rm -rf "$bin"' EXIT
if [ -n "${COLUMBO_BIN:-}" ]; then cp "$COLUMBO_BIN" "$bin/columbo"; else (cd "$root" && go build -o "$bin/columbo" ./cmd/columbo); fi
(cd "$here" && go build -o "$bin/calibrate" ./cmd/calibrate)
version="$(git -C "$root" rev-parse --short HEAD)"
"$bin/calibrate" run --corpus "$root" --module github.com/KellyBennett/Columbo --version "$version" \
  --columbo "$bin/columbo" --db "$db" --operators "$ops" --validity build --max-sites "$max" --workers "${WORKERS:-8}" ${config[@]+"${config[@]}"} ${REDTEAM:+--redteam "$here/redteam"}
probe="$(mktemp -d)"
(cd "$probe" && go mod init calibration.probe >/dev/null 2>&1)
grep -v '^#' "$here/corpus.txt" | while read -r module modversion _license; do
  [ -n "$module" ] || continue
  dir="$(cd "$probe" && go mod download -json "$module@$modversion" | python3 -c 'import json,sys; print(json.load(sys.stdin)["Dir"])')"
  "$bin/calibrate" run --corpus "$dir" --module "$module" --version "$modversion" \
    --columbo "$bin/columbo" --db "$db" --operators "$ops" --validity build --max-sites "$max" --workers "${WORKERS:-8}" ${config[@]+"${config[@]}"} ${REDTEAM:+--redteam "$here/redteam"}
done
rm -rf "$probe"
