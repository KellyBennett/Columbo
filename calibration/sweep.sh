#!/usr/bin/env bash
# Usage: sweep.sh <outdir> <operators> <max-sites> <config.yml>...
# Measures the same operators under each Columbo configuration into <outdir>/<config>.sqlite,
# so a threshold change can be compared row by row against defaults.yml.
set -euo pipefail
here="$(cd "$(dirname "$0")" && pwd)"
out="$1"; ops="$2"; max="$3"; shift 3
mkdir -p "$out"
for config in "$@"; do
  name="$(basename "$config" .yml)"
  rm -f "$out/$name.sqlite"
  "$here/calibrate.sh" "$out/$name.sqlite" "$ops" "$max" "$config" > "$out/$name.log" 2>&1
done
