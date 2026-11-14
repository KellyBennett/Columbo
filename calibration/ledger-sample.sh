#!/usr/bin/env bash
# Usage: ledger-sample.sh <export.json>
# Records Columbo's own module and every public module in corpus.txt into a throwaway
# case ledger under a temporary HOME, then writes `columbo calibrate --export` to
# <export.json>. It never reads or writes the real ~/.columbo.
set -euo pipefail
here="$(cd "$(dirname "$0")" && pwd)"
root="$(cd "$here/.." && pwd)"
out="$(cd "$(dirname "$1")" && pwd)/$(basename "$1")"
work="$(cd "$(mktemp -d)" && pwd -P)"
trap 'chmod -R u+w "$work"; rm -rf "$work"' EXIT
GOMODCACHE="$(go env GOMODCACHE)" GOCACHE="$(go env GOCACHE)" GOPATH="$(go env GOPATH)"
export GOMODCACHE GOCACHE GOPATH TMPDIR="$work"
(cd "$root" && go build -o "$work/columbo" ./cmd/columbo)
mkdir -p "$work/home" "$work/probe" "$work/corpus"
record() {
  local status=0
  (cd "$1" && HOME="$work/home" "$work/columbo" --ledger --no-history --output "$work/$2.sqlite" ./... >/dev/null) || status=$?
  [ "$status" -le 1 ] || { echo "columbo exited $status on $1" >&2; exit "$status"; }
}
record "$root" self
(cd "$work/probe" && go mod init ledger.sample.probe >/dev/null 2>&1)
grep -v '^#' "$here/corpus.txt" | while read -r module version _license; do
  [ -n "$module" ] || continue
  dir="$(cd "$work/probe" && go mod download -json "$module@$version" | python3 -c 'import json,sys; print(json.load(sys.stdin)["Dir"])')"
  name="$(echo "$module" | tr / _)"
  cp -R "$dir" "$work/corpus/$name"
  chmod -R u+w "$work/corpus/$name"
  record "$work/corpus/$name" "$name"
done
HOME="$work/home" "$work/columbo" calibrate --export > "$out"
