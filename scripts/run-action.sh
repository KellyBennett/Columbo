#!/usr/bin/env bash
set -euo pipefail

evidence=$(mktemp -d "$RUNNER_TEMP/columbo-evidence.XXXXXX")
printf 'evidence=%s\n' "$evidence" >> "$GITHUB_OUTPUT"
status=2
{
  printf 'Analyzed commit: %s\n' "$GITHUB_SHA"
  go version
} > "$evidence/provenance.txt"
if CGO_ENABLED=0 go -C "$GITHUB_ACTION_PATH" build -o "$evidence/columbo" ./cmd/columbo \
    > "$evidence/build.txt" 2>&1; then
  go version -m "$evidence/columbo" >> "$evidence/provenance.txt"
  sha256sum "$evidence/columbo" >> "$evidence/provenance.txt"
  cd "$GITHUB_WORKSPACE/$COLUMBO_WORKING_DIRECTORY"
  args=(--output "$evidence/report.sqlite")
  if [[ -n "$COLUMBO_CONFIG" ]]; then args+=(--config "$COLUMBO_CONFIG"); fi
  status=0
  "$evidence/columbo" "${args[@]}" ./... 2>&1 | tee "$evidence/report.txt" || status=$?
else
  cat "$evidence/build.txt"
fi
printf '%s\n' "$status" > "$evidence/exit-status.txt"
printf 'exit-code=%s\n' "$status" >> "$GITHUB_OUTPUT"
# Findings are reported after publishing and artifact upload finish.
