#!/usr/bin/env bash
# Run only the DB boundary checks and retain every migration site, even on failure.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"
source scripts/database-boundary/tools.env
export GOTOOLCHAIN=local GOTELEMETRY=off GOWORK=off
export GOFLAGS="${GOFLAGS:-} -buildvcs=false"
report="${BOUNDARY_REPORT_DIR:-$(mktemp -d)}"
mkdir -p "$report"
tools="${COLUMBO_TOOLS_BIN:-$root/.tools/bin}"
lint="${GOLANGCI_LINT:-$tools/golangci-lint}"
if [[ ! -x "$lint" ]]; then
  echo "Pinned golangci-lint missing; run ./scripts/install-database-tools.sh" >&2
  exit 2
fi
"$lint" version
if ! "$lint" version | grep -F "version $GOLANGCI_LINT_VERSION built with go$DATABASE_GUARD_GO_VERSION " >/dev/null; then
  echo "Unexpected linter version/build; run ./scripts/install-database-tools.sh" >&2
  exit 2
fi
schema="${GOLANGCI_LINT_SCHEMA:-$(dirname "$lint")/golangci.database.schema.json}"
if [[ ! -f "$schema" ]]; then
  echo "Pinned linter schema missing; run ./scripts/install-database-tools.sh" >&2
  exit 2
fi
schema_url="$(python3 -c 'from pathlib import Path; import sys; print(Path(sys.argv[1]).resolve().as_uri())' "$schema")"
"$lint" config verify --config .golangci.database.yml --schema "$schema_url"
status=0
python3 scripts/database-boundary/check-manifest.py --json "$report/manifest.json" || status=1
python3 scripts/database-boundary/check-package-inventory.py --json "$report/packages.json" || status=1
# Force our dedicated config and scan all packages; do not inherit a user's config,
# skip generated headers, or require unrelated full-project linters to be clean.
"$lint" run --config .golangci.database.yml --output.text.path stdout --output.json.path "$report/lint.json" ./... || status=1
python3 scripts/database-boundary/check-generated.py || status=1
printf 'Database boundary diagnostics: %s/{manifest,packages,lint}.json\n' "$report"
exit "$status"
