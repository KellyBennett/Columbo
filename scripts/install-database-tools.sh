#!/usr/bin/env bash
# Install official pinned guard tooling, without editing application go.mod/go.sum.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
source "$root/scripts/database-boundary/tools.env"
bin="${COLUMBO_TOOLS_BIN:-$root/.tools/bin}"
mkdir -p "$bin"
bin="$(cd "$bin" && pwd)"
go="${GO:-go}"
if [[ "$("$go" env GOVERSION)" != "go$DATABASE_GUARD_GO_VERSION" ]]; then
  echo "Database guard tools require Go $DATABASE_GUARD_GO_VERSION (set GO or PATH)" >&2
  exit 2
fi
# Go's official module proxy and checksum database verify the pinned module and
# its transitive versions; build with the application's Go to support its syntax.
GOBIN="$bin" GOTOOLCHAIN=local GOSUMDB=sum.golang.org GOMAXPROCS=2 \
  "$go" install "$GOLANGCI_LINT_MODULE@v$GOLANGCI_LINT_VERSION"
"$go" version -m "$bin/golangci-lint" | grep -F "$GOLANGCI_LINT_MODULE_SUM" >/dev/null
"$bin/golangci-lint" version
# Use the schema shipped in the checksum-verified pinned module, so validation
# never depends on a mutable website or an extra network request during lint.
cp "$("$go" env GOMODCACHE)/github.com/golangci/golangci-lint/v2@v$GOLANGCI_LINT_VERSION/jsonschema/golangci.next.jsonschema.json" \
  "$bin/golangci.database.schema.json"
os="$("$go" env GOHOSTOS)"
arch="$("$go" env GOHOSTARCH)"
key="SQLC_SHA256_${os}_${arch}"
sha="${!key:-}"
if [[ -z "$sha" ]]; then
  echo "No checked sqlc release digest for $os/$arch" >&2
  exit 2
fi
temporary="$(mktemp -d)"
trap 'rm -rf "$temporary"' EXIT
archive="sqlc_${SQLC_VERSION}_${os}_${arch}.tar.gz"
curl --fail --location --retry 3 --output "$temporary/$archive" \
  "https://downloads.sqlc.dev/$archive"
python3 - "$temporary/$archive" "$sha" <<'PY'
import hashlib, pathlib, sys
actual = hashlib.sha256(pathlib.Path(sys.argv[1]).read_bytes()).hexdigest()
if actual != sys.argv[2]:
    raise SystemExit("Official sqlc release checksum mismatch")
PY
# Extract only the verified binary, never archive-supplied paths or scripts.
tar -xzf "$temporary/$archive" -C "$temporary" sqlc
install -m 0755 "$temporary/sqlc" "$bin/sqlc"
[[ "$("$bin/sqlc" version)" == "v$SQLC_VERSION" ]]
"$bin/sqlc" version
