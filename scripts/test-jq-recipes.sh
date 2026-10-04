#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$root"
for tool in go jq git; do
  command -v "$tool" >/dev/null || { echo "Required tool not found: $tool" >&2; exit 1; }
done

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
recipes="$root/docs/jq"
go build -o "$tmp/columbo" ./cmd/columbo
mkdir "$tmp/fixture" "$tmp/warn" "$tmp/empty"
cp internal/columbo/testdata/all/{all.go,go.mod} "$tmp/fixture/"
cp internal/columbo/testdata/all/{all.go,go.mod} "$tmp/warn/"
cp internal/columbo/testdata/all/go.mod "$tmp/empty/"
printf 'package fixture\n' > "$tmp/empty/empty.go"

capture() {
  local expected=$1 dir=$2 output=$3 status=0
  shift 3
  (cd "$dir" && "$tmp/columbo" --format json "$@" ./...) > "$output" 2> "$output.stderr" || status=$?
  if [[ "$status" != "$expected" ]]; then
    cat "$output.stderr" >&2
    echo "Columbo exit $status, expected $expected ($output)" >&2
    exit 1
  fi
}

capture 1 "$tmp/fixture" "$tmp/all.json" --no-history
jq -e '.version == 1 and .summary == {failed: 8, warned: 0, suppressed: 0}
  and ([.cases[].smell] | unique | length) == 7' "$tmp/all.json" >/dev/null

# Include suppressed FAIL, unsuppressed WARN, and an unused-suppression warning.
# The unused Long Function directive attaches to the short Parent declaration.
awk '/^func Parent\(/ {
  print "// columbo:ignore cosmetic-extraction -- reviewed helper boundary for this fixture"
  print "// columbo:ignore long-function -- intentionally unused suppression fixture"
} {print}' "$tmp/fixture/all.go" > "$tmp/fixture/source.tmp"
mv "$tmp/fixture/source.tmp" "$tmp/fixture/all.go"
printf 'severity:\n  long-parameter-list: warn\n' > "$tmp/fixture/.columbo.yml"
git -C "$tmp/fixture" init -q
git -C "$tmp/fixture" add .
GIT_AUTHOR_DATE='2020-01-01T00:00:00Z' GIT_COMMITTER_DATE='2020-01-01T00:00:00Z' \
  git -C "$tmp/fixture" -c user.name='Recipe Test' -c user.email='recipes@example.invalid' \
  -c commit.gpgsign=false commit -qm 'Create recipe fixture'
capture 1 "$tmp/fixture" "$tmp/mixed.json"
jq -e '.summary == {failed: 5, warned: 2, suppressed: 1}
  and (.warnings | length) == 1 and .warnings[0].code == "unused-suppression"
  and any(.cases[]; .verdict == "FAIL" and .suppressed)
  and any(.cases[].receipts[]; .kind == "history")' "$tmp/mixed.json" >/dev/null
# A policy review must also survive an unsuppressed WARN verdict.
printf 'severity:\n  cosmetic-extraction: warn\n' > "$tmp/warn/.columbo.yml"
capture 1 "$tmp/warn" "$tmp/warn.json" --no-history
jq -e '.summary == {failed: 7, warned: 1, suppressed: 0}
  and any(.cases[]; .verdict == "WARN" and .suppressed == false and .policy_reviews[0].id == "CE-001")' "$tmp/warn.json" >/dev/null
capture 0 "$tmp/empty" "$tmp/empty.json" --no-history

for report in "$tmp/all.json" "$tmp/mixed.json" "$tmp/warn.json" "$tmp/empty.json"; do
  jq -f "$recipes/failures.jq" "$report" |
    jq -e --slurpfile original "$report" '
      .version == $original[0].version and .summary == $original[0].summary
      and .warnings == $original[0].warnings
      and (.cases | length) == .summary.failed
      and [.cases[].id] == [$original[0].cases[] | select(.verdict == "FAIL" and .suppressed == false) | .id]
      and all(.cases[]; . as $case | $original[0].cases[] | select(.id == $case.id)
        | {id, smell, verdict, symbol, file, start_line, end_line, clues, policy_reviews} == $case)
    ' >/dev/null
  jq -f "$recipes/policy-reviews.jq" "$report" |
    jq -e --slurpfile original "$report" \
      '. == [$original[0].cases[] | select(.policy_reviews != [])]' >/dev/null
  jq --arg symbol 'fixture.Dependencies' -f "$recipes/symbol.jq" "$report" |
    jq -e --slurpfile original "$report" \
      '. == [$original[0].cases[] | select(.symbol == "fixture.Dependencies")]' >/dev/null
  jq --arg symbol 'fixture.NoFindings" | error("injected")' -f "$recipes/symbol.jq" "$report" |
    jq -e '. == []' >/dev/null
  for recipe in case receipts; do
    if jq --arg id 'C-missing' --arg kind declaration -f "$recipes/$recipe.jq" "$report" \
        > "$tmp/missing.out" 2> "$tmp/missing.err"; then
      echo "$recipe.jq accepted an unknown case ID" >&2
      exit 1
    fi
    test ! -s "$tmp/missing.out"
    grep -q 'Unknown case id: C-missing' "$tmp/missing.err"
  done
  # Exercise exact-case and receipt selectors on every case and every emitted kind.
  while IFS= read -r id; do
    jq --arg id "$id" -f "$recipes/case.jq" "$report" |
      jq -e --arg id "$id" --slurpfile original "$report" \
        '. == ($original[0].cases[] | select(.id == $id))' >/dev/null
    while IFS= read -r kind; do
      jq --arg id "$id" --arg kind "$kind" -f "$recipes/receipts.jq" "$report" |
        jq -e --arg id "$id" --arg kind "$kind" --slurpfile original "$report" '
          . as $view | ($original[0].cases[] | select(.id == $id)) as $case
          | ($view | del(.receipts)) == ($case | {id, smell, verdict, suppressed, symbol, clues, clusters, policy_reviews})
          and $view.receipts == [$case.receipts[] | select(.kind == $kind)]
        ' >/dev/null
    done < <(jq -r --arg id "$id" '.cases[] | select(.id == $id) | [.receipts[].kind, "not-a-receipt-kind"] | unique[]' "$report")
  done < <(jq -r '.cases[].id' "$report")
done

# Guards ensure these runs actually exercise copied evidence and review notes.
jq -e 'any(.cases[].receipts[]; .kind == "metric-contribution" and (.detail.expansion_sites | length) > 0)
  and any(.cases[]; (.clusters | length) > 0 and .policy_reviews[0].id == "CE-001")' "$tmp/all.json" >/dev/null
jq -e 'any(.cases[]; .suppressed and .policy_reviews[0].id == "CE-001")' "$tmp/mixed.json" >/dev/null
echo 'jq recipes: all checks passed'
