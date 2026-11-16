#!/usr/bin/env bash
set -euo pipefail

evidence=$(mktemp -d "$RUNNER_TEMP/columbo-evidence.XXXXXX")
printf 'evidence=%s\n' "$evidence" >> "$GITHUB_OUTPUT"
# Kept outside the evidence directory so the binary is never uploaded.
analyzer="$(mktemp -d "$RUNNER_TEMP/columbo-analyzer.XXXXXX")/columbo"
status=2
{
  printf 'Analyzed commit: %s\n' "$GITHUB_SHA"
  go version
} > "$evidence/provenance.txt"
if CGO_ENABLED=0 go -C "$GITHUB_ACTION_PATH" build -o "$analyzer" ./cmd/columbo \
    > "$evidence/build.txt" 2>&1; then
  printf 'analyzer=%s\n' "$analyzer" >> "$GITHUB_OUTPUT"
  go version -m "$analyzer" >> "$evidence/provenance.txt"
  sha256sum "$analyzer" >> "$evidence/provenance.txt"
  cd "$GITHUB_WORKSPACE/$COLUMBO_WORKING_DIRECTORY"
  args=(--output "$evidence/report.sqlite")
  if [[ -n "$COLUMBO_CONFIG" ]]; then args+=(--config "$COLUMBO_CONFIG"); fi
  case "${COLUMBO_STAGED:-true}" in
    true|false) args+=("--staged=${COLUMBO_STAGED:-true}") ;;
    *) printf 'staged must be true or false\n' >&2; exit 2 ;;
  esac
  status=0
  "$analyzer" "${args[@]}" ./... 2>&1 | tee "$evidence/report.txt" || status=$?
else
  cat "$evidence/build.txt"
fi
printf '%s\n' "$status" > "$evidence/exit-status.txt"
printf 'exit-code=%s\n' "$status" >> "$GITHUB_OUTPUT"
# Findings are reported after publishing and artifact upload finish.
