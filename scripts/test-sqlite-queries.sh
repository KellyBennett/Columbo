#!/usr/bin/env bash
# Run the published SQL exactly as documented, without sqlite3 CLI dependencies.
set -euo pipefail
if [ "$#" -ne 1 ]; then
  printf 'usage: %s SNAPSHOT.sqlite\n' "$0" >&2
  exit 2
fi
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
python3 - "$root" "$1" <<'PY'
import math
import sqlite3
import sys
from pathlib import Path

root, supplied = map(Path, sys.argv[1:])
path = supplied.resolve(strict=True)
connection = sqlite3.connect(path.as_uri() + '?mode=ro', uri=True)
connection.execute('PRAGMA query_only = ON')
connection.execute('PRAGMA foreign_keys = ON')


def require(condition, message):
    if not condition:
        raise AssertionError(message)


def statements(content):
    pending = ''
    for line in content.splitlines(keepends=True):
        pending += line
        if sqlite3.complete_statement(pending):
            yield pending
            pending = ''
    require(not pending.strip(), 'incomplete SQL statement')


require(connection.execute('PRAGMA application_id').fetchone()[0] == 0x434C4D42,
        'not a Columbo snapshot')
version = connection.execute('PRAGMA user_version').fetchone()[0]
require(version == 6, 'unsupported schema version')
report = connection.execute('SELECT id, schema_version, columbo_version FROM report').fetchall()
require(len(report) == 1 and report[0][:2] == (1, version), 'invalid report metadata')
require(connection.execute('PRAGMA integrity_check').fetchall() == [('ok',)], 'integrity check failed')
require(not connection.execute('PRAGMA foreign_key_check').fetchall(), 'foreign key check failed')

queries = sorted((root / 'docs/sqlite').glob('*.sql'))
require(bool(queries), 'no published SQL queries found')
results, count = {}, 0
for query in queries:
    outputs = []
    for sql in statements(query.read_text()):
        rows = connection.execute(sql, {'case_id': None}).fetchall()
        outputs.append(rows)
        count += 1
    results[query.name] = outputs
    print(f'{query.name}: {len(outputs)} statements, {sum(map(len, outputs))} rows')

for row in results['metric-to-receipt-reconciliation.sql'][0]:
    require(row[-1] == 1, f'contributions do not reconcile: {row!r}')
summary = connection.execute('SELECT failed, warned, suppressed FROM summary').fetchone()
expected = connection.execute('''
SELECT COUNT(CASE WHEN verdict='FAIL' AND suppressed=0 THEN 1 END),
       COUNT(CASE WHEN verdict='WARN' AND suppressed=0 THEN 1 END),
       COUNT(CASE WHEN suppressed=1 THEN 1 END)
FROM cases''').fetchone()
require(summary == expected, 'summary diverges from stored verdicts')
require(len(results['unsuppressed-failures.sql'][0]) == summary[0], 'failure query count diverges')

for case_id, kind, metric, scored in connection.execute('''
SELECT q.case_id, q.kind, q.numeric_value,
       (SELECT COUNT(*) FROM declaration_dependencies dd
        WHERE dd.declaration_id=q.declaration_id AND dd.scored=1)
FROM clues q WHERE q.kind='dependencies' '''):
    require(metric == scored, f'scored dependency count diverges for {case_id}: {metric} != {scored}')

require(not connection.execute('''
SELECT 1 FROM dependency_receipts dr
JOIN declaration_dependencies dd USING(declaration_id, dependency_id)
JOIN source_receipts r ON r.id=dr.receipt_id
WHERE (r.kind='dependency' AND dd.scored<>1)
   OR (r.kind='dependency-inventory' AND dd.scored<>0)
   OR r.kind NOT IN ('dependency','dependency-inventory')''').fetchall(),
        'dependency inventory/scored receipt distinction diverges')

require(not connection.execute('''
SELECT 1 FROM clues q JOIN clue_values v ON v.clue_id=q.id
WHERE q.value_type<>'list' ''').fetchall(), 'numeric clue has list children')
for value, in connection.execute('SELECT numeric_value FROM clues WHERE value_type<>\'list\''):
    require(math.isfinite(value), 'nonfinite stored numeric clue')
for value, in connection.execute('SELECT limit_value FROM clues WHERE limit_type IS NOT NULL'):
    require(math.isfinite(value), 'nonfinite stored comparison limit')

for receipt_id, declarations, sites in connection.execute('''
SELECT r.id,
       (SELECT COUNT(*) FROM receipt_expansion_declarations x WHERE x.receipt_id=r.id),
       (SELECT COUNT(*) FROM receipt_expansion_sites x WHERE x.receipt_id=r.id)
FROM source_receipts r'''):
    require((declarations == 0 and sites == 0) or declarations == sites + 1,
            f'expansion path lengths diverge for receipt {receipt_id}')

require(not connection.execute('''
SELECT 1 FROM case_history_files hf
WHERE NOT EXISTS (SELECT 1 FROM source_receipts r
                  WHERE r.case_id=hf.case_id AND r.file_id=hf.file_id)''').fetchall(),
        'case history contains a file outside its source receipt subset')

# Exercise parameter binding on each saved case, too. SQL must stay read-only.
case_ids = [row[0] for row in connection.execute('SELECT id FROM cases ORDER BY ordinal')]
for case_id in case_ids:
    for query in queries:
        for sql in statements(query.read_text()):
            if ':case_id' in sql:
                connection.execute(sql, {'case_id': case_id}).fetchall()
connection.close()
print(f'OK: {len(queries)} published queries ({count} statements), {len(case_ids)} case filters')
PY
