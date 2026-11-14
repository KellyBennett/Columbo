-- name: InsertLedgerSecret :exec
INSERT INTO ledger_secret (id, secret) VALUES (1, ?) ON CONFLICT (id) DO NOTHING;

-- name: LedgerSecret :one
SELECT secret FROM ledger_secret WHERE id = 1;

-- name: RecordLedgerCase :exec
INSERT INTO ledger_cases (repo, case_key, smell, verdict, suppressed, excess) VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT (repo, case_key) DO UPDATE SET smell = excluded.smell, verdict = excluded.verdict,
 suppressed = excluded.suppressed, excess = excluded.excess;

-- name: LedgerBuckets :many
SELECT smell, verdict, suppressed, excess, cases, repos FROM ledger_buckets
WHERE cases >= sqlc.arg(min_cases) AND repos >= sqlc.arg(min_repos)
ORDER BY smell, verdict, suppressed, excess;
