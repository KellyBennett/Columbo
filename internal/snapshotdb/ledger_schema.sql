CREATE TABLE IF NOT EXISTS ledger_secret (
 id INTEGER PRIMARY KEY CHECK (id = 1),
 secret BLOB NOT NULL CHECK (length(secret) = 32)
) STRICT;
CREATE TABLE IF NOT EXISTS ledger_cases (
 repo BLOB NOT NULL CHECK (length(repo) = 32),
 case_key BLOB NOT NULL CHECK (length(case_key) = 32),
 smell TEXT NOT NULL CHECK (length(smell) > 0),
 verdict TEXT NOT NULL CHECK (verdict IN ('FAIL', 'WARN')),
 suppressed INTEGER NOT NULL CHECK (suppressed IN (0, 1)),
 excess INTEGER NOT NULL CHECK (excess BETWEEN 0 AND 4),
 PRIMARY KEY (repo, case_key)
) STRICT;
CREATE VIEW IF NOT EXISTS ledger_buckets AS
 SELECT smell, verdict, suppressed, excess, count(*) AS cases, count(DISTINCT repo) AS repos
 FROM ledger_cases GROUP BY smell, verdict, suppressed, excess;
