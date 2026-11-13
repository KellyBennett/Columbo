#!/usr/bin/env bash
# Usage: compare.sh <smell-filter> <db>...
# For each sweep database: baseline (self-check) case counts for smells matching the filter,
# summed over corpora, then outcome counts per operator. Run one db per configuration.
set -euo pipefail
filter="$1"; shift
for db in "$@"; do
  echo "== $(basename "$db" .sqlite)"
  sqlite3 -column -header "$db" "SELECT r.corpus_module AS corpus, b.smell, count(*) AS cases FROM baseline_findings b JOIN runs r ON r.id = b.run_id WHERE b.smell LIKE '$filter' AND b.verdict = 'FAIL' GROUP BY 1, 2 ORDER BY 2, 1"
  sqlite3 -column -header "$db" "SELECT operator, sum(status='killed') AS killed, sum(status='survived') AS survived, sum(status='equivalent') AS equivalent, sum(status='conflict') AS conflict, sum(status='invalid') AS invalid FROM mutants GROUP BY operator ORDER BY operator"
  sqlite3 -column -header "$db" "SELECT m.operator, f.smell AS conflict_smell, count(*) AS n FROM mutants m JOIN mutant_findings f ON f.mutant_id = m.id AND f.side = 'added' WHERE m.status = 'conflict' GROUP BY 1, 2 ORDER BY 1, 3 DESC"
done
