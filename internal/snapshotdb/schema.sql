
CREATE TABLE report (
 id INTEGER PRIMARY KEY CHECK (id = 1),
 schema_version INTEGER NOT NULL CHECK (schema_version = 3),
 columbo_version TEXT NOT NULL
) STRICT;
CREATE TABLE files (
 id INTEGER PRIMARY KEY,
 path TEXT NOT NULL UNIQUE CHECK (length(path) > 0 AND substr(path, 1, 1) <> '/' AND path <> '..' AND path NOT LIKE '../%' AND instr(path, '/../') = 0)
) STRICT;
CREATE TABLE declarations (
 id INTEGER PRIMARY KEY,
 file_id INTEGER NOT NULL REFERENCES files(id),
 symbol TEXT NOT NULL CHECK (length(symbol) > 0),
 start_line INTEGER NOT NULL CHECK (start_line > 0),
 end_line INTEGER NOT NULL CHECK (end_line >= start_line),
 start_offset INTEGER NOT NULL CHECK (start_offset >= 0),
 end_offset INTEGER NOT NULL CHECK (end_offset >= start_offset),
 UNIQUE (file_id, symbol),
 UNIQUE (id, file_id)
) STRICT;
CREATE INDEX declarations_symbol ON declarations(symbol);
CREATE TABLE cases (
 id TEXT PRIMARY KEY,
 ordinal INTEGER NOT NULL UNIQUE CHECK (ordinal >= 0),
 primary_declaration_id INTEGER NOT NULL REFERENCES declarations(id),
 smell TEXT NOT NULL CHECK (smell IN ('long-function','long-parameter-list','high-cognitive-complexity','excessive-dependencies','feature-envy','data-clump','cosmetic-extraction','duplicate-code','repeated-variant-decision','selection-use-coupling')),
 verdict TEXT NOT NULL CHECK (verdict IN ('FAIL','WARN')),
 suppressed INTEGER NOT NULL CHECK (suppressed IN (0, 1)),
 start_line INTEGER NOT NULL CHECK (start_line > 0),
 end_line INTEGER NOT NULL CHECK (end_line >= start_line),
 why TEXT NOT NULL,
 diagnosis TEXT NOT NULL
) STRICT;
CREATE INDEX cases_declaration ON cases(primary_declaration_id);
CREATE INDEX cases_outcomes ON cases(smell, verdict, suppressed);
CREATE TABLE case_declarations (
 case_id TEXT NOT NULL REFERENCES cases(id),
 declaration_id INTEGER NOT NULL REFERENCES declarations(id),
 role TEXT NOT NULL CHECK (length(role) > 0),
 PRIMARY KEY (case_id, declaration_id, role)
) STRICT;
CREATE INDEX case_declarations_declaration ON case_declarations(declaration_id);
CREATE TABLE case_guidance (
 case_id TEXT NOT NULL REFERENCES cases(id),
 kind TEXT NOT NULL CHECK (kind IN ('lead', 'avoid')),
 ordinal INTEGER NOT NULL CHECK (ordinal >= 0),
 item TEXT NOT NULL,
 PRIMARY KEY (case_id, kind, ordinal)
) STRICT;
CREATE TABLE dependencies (
 id INTEGER PRIMARY KEY,
 identity TEXT NOT NULL UNIQUE CHECK (length(identity) > 0)
) STRICT;
CREATE TABLE declaration_dependencies (
 declaration_id INTEGER NOT NULL REFERENCES declarations(id),
 dependency_id INTEGER NOT NULL REFERENCES dependencies(id),
 scored INTEGER NOT NULL CHECK (scored IN (0, 1)),
 PRIMARY KEY (declaration_id, dependency_id)
) STRICT;
CREATE INDEX declaration_dependencies_dependency ON declaration_dependencies(dependency_id, scored);
CREATE TABLE clusters (
 id INTEGER PRIMARY KEY,
 case_id TEXT NOT NULL REFERENCES cases(id),
 cluster_key TEXT NOT NULL,
 owner_declaration_id INTEGER NOT NULL,
 file_id INTEGER NOT NULL REFERENCES files(id),
 ordinal INTEGER NOT NULL CHECK (ordinal >= 0),
 FOREIGN KEY (owner_declaration_id, file_id) REFERENCES declarations(id, file_id),
 UNIQUE (case_id, cluster_key),
 UNIQUE (case_id, ordinal),
 UNIQUE (id, case_id)
) STRICT;
CREATE INDEX clusters_owner ON clusters(owner_declaration_id, file_id);
CREATE INDEX clusters_file ON clusters(file_id);
CREATE TABLE cluster_members (
 id INTEGER PRIMARY KEY,
 cluster_id INTEGER NOT NULL,
 case_id TEXT NOT NULL REFERENCES cases(id),
 helper_declaration_id INTEGER NOT NULL REFERENCES declarations(id),
 ordinal INTEGER NOT NULL CHECK (ordinal >= 0),
 call_offset INTEGER NOT NULL CHECK (call_offset >= 0),
 FOREIGN KEY (cluster_id, case_id) REFERENCES clusters(id, case_id),
 UNIQUE (cluster_id, ordinal),
 UNIQUE (cluster_id, helper_declaration_id, call_offset),
 UNIQUE (id, cluster_id, case_id)
) STRICT;
CREATE INDEX cluster_members_case ON cluster_members(case_id);
CREATE INDEX cluster_members_helper ON cluster_members(helper_declaration_id);
CREATE TABLE clues (
 id INTEGER PRIMARY KEY,
 case_id TEXT NOT NULL REFERENCES cases(id),
 ordinal INTEGER NOT NULL CHECK (ordinal >= 0),
 kind TEXT NOT NULL,
 subject TEXT NOT NULL,
 value_type TEXT NOT NULL CHECK (value_type IN ('integer', 'real', 'list')),
 numeric_value ANY,
 limit_type TEXT CHECK (limit_type IN ('integer', 'real')),
 limit_value ANY,
 operator TEXT CHECK (operator IN ('>', '>=', '<', '<=', '=', '!=')),
 declaration_id INTEGER REFERENCES declarations(id),
 cluster_id INTEGER,
 member_id INTEGER,
 pair_left_member_id INTEGER,
 pair_right_member_id INTEGER,
 CHECK ((value_type = 'integer' AND typeof(numeric_value) = 'integer') OR (value_type = 'real' AND typeof(numeric_value) = 'real') OR (value_type = 'list' AND numeric_value IS NULL)),
 CHECK ((limit_type IS NULL AND limit_value IS NULL AND operator IS NULL) OR (limit_type IS NOT NULL AND limit_value IS NOT NULL AND operator IS NOT NULL AND ((limit_type = 'integer' AND typeof(limit_value) = 'integer') OR (limit_type = 'real' AND typeof(limit_value) = 'real')))),
 CHECK ((pair_left_member_id IS NULL) = (pair_right_member_id IS NULL)),
 CHECK (member_id IS NULL OR (cluster_id IS NOT NULL AND pair_left_member_id IS NULL)),
 CHECK (pair_left_member_id IS NULL OR (cluster_id IS NOT NULL AND pair_left_member_id <> pair_right_member_id)),
 FOREIGN KEY (cluster_id, case_id) REFERENCES clusters(id, case_id),
 FOREIGN KEY (member_id, cluster_id, case_id) REFERENCES cluster_members(id, cluster_id, case_id),
 FOREIGN KEY (pair_left_member_id, cluster_id, case_id) REFERENCES cluster_members(id, cluster_id, case_id),
 FOREIGN KEY (pair_right_member_id, cluster_id, case_id) REFERENCES cluster_members(id, cluster_id, case_id),
 UNIQUE (case_id, ordinal),
 UNIQUE (id, case_id)
) STRICT;
CREATE INDEX clues_kind ON clues(kind);
CREATE INDEX clues_declaration ON clues(declaration_id);
CREATE INDEX clues_cluster ON clues(cluster_id, case_id);
CREATE INDEX clues_member ON clues(member_id, cluster_id, case_id);
CREATE INDEX clues_pair_left ON clues(pair_left_member_id, cluster_id, case_id);
CREATE INDEX clues_pair_right ON clues(pair_right_member_id, cluster_id, case_id);
CREATE TABLE clue_values (
 clue_id INTEGER NOT NULL REFERENCES clues(id),
 ordinal INTEGER NOT NULL CHECK (ordinal >= 0),
 value TEXT NOT NULL,
 PRIMARY KEY (clue_id, ordinal)
) STRICT;
CREATE TRIGGER clue_values_list BEFORE INSERT ON clue_values WHEN (SELECT value_type FROM clues WHERE id = NEW.clue_id) <> 'list' BEGIN SELECT RAISE(ABORT, 'clue_values requires a list clue'); END;
CREATE TRIGGER clue_values_list_update BEFORE UPDATE ON clue_values WHEN (SELECT value_type FROM clues WHERE id = NEW.clue_id) <> 'list' BEGIN SELECT RAISE(ABORT, 'clue_values requires a list clue'); END;
CREATE TRIGGER clues_list_update BEFORE UPDATE OF value_type ON clues WHEN NEW.value_type <> 'list' AND EXISTS (SELECT 1 FROM clue_values WHERE clue_id = OLD.id) BEGIN SELECT RAISE(ABORT, 'a clue with list items must remain a list'); END;
CREATE TABLE source_receipts (
 id INTEGER PRIMARY KEY,
 case_id TEXT NOT NULL REFERENCES cases(id),
 ordinal INTEGER NOT NULL CHECK (ordinal >= 0),
 kind TEXT NOT NULL,
 file_id INTEGER NOT NULL REFERENCES files(id),
 start_line INTEGER NOT NULL CHECK (start_line > 0),
 end_line INTEGER NOT NULL CHECK (end_line >= start_line),
 start_offset INTEGER NOT NULL CHECK (start_offset >= 0),
 end_offset INTEGER NOT NULL CHECK (end_offset >= start_offset),
 subject TEXT NOT NULL,
 value_type TEXT CHECK (value_type IN ('integer', 'real')),
 value ANY,
 nesting INTEGER CHECK (nesting >= 0),
 source_declaration_id INTEGER,
 spelling TEXT NOT NULL DEFAULT '',
 CHECK ((value_type IS NULL AND value IS NULL) OR (value_type IS NOT NULL AND ((value_type = 'integer' AND typeof(value) = 'integer') OR (value_type = 'real' AND typeof(value) = 'real')))),
 FOREIGN KEY (source_declaration_id, file_id) REFERENCES declarations(id, file_id),
 UNIQUE (case_id, ordinal),
 UNIQUE (id, case_id),
 UNIQUE (id, source_declaration_id)
) STRICT;
CREATE INDEX source_receipts_locations ON source_receipts(file_id, start_line, end_line, start_offset, end_offset, kind);
CREATE INDEX source_receipts_declaration ON source_receipts(source_declaration_id, file_id);
CREATE TABLE dependency_receipts (
 declaration_id INTEGER NOT NULL,
 dependency_id INTEGER NOT NULL,
 receipt_id INTEGER NOT NULL,
 case_id TEXT NOT NULL,
 FOREIGN KEY (declaration_id, dependency_id) REFERENCES declaration_dependencies(declaration_id, dependency_id),
 FOREIGN KEY (receipt_id, case_id) REFERENCES source_receipts(id, case_id),
 FOREIGN KEY (receipt_id, declaration_id) REFERENCES source_receipts(id, source_declaration_id),
 PRIMARY KEY (declaration_id, dependency_id, receipt_id)
) STRICT;
CREATE INDEX dependency_receipts_dependency ON dependency_receipts(dependency_id);
CREATE INDEX dependency_receipts_source ON dependency_receipts(receipt_id, case_id);
CREATE TABLE receipt_expansion_declarations (
 receipt_id INTEGER NOT NULL REFERENCES source_receipts(id),
 ordinal INTEGER NOT NULL CHECK (ordinal >= 0),
 declaration_id INTEGER NOT NULL REFERENCES declarations(id),
 symbol TEXT NOT NULL,
 PRIMARY KEY (receipt_id, ordinal)
) STRICT;
CREATE INDEX receipt_expansion_declarations_declaration ON receipt_expansion_declarations(declaration_id);
CREATE TABLE receipt_expansion_sites (
 receipt_id INTEGER NOT NULL REFERENCES source_receipts(id),
 ordinal INTEGER NOT NULL CHECK (ordinal >= 0),
 file_id INTEGER NOT NULL REFERENCES files(id),
 call_offset INTEGER NOT NULL CHECK (call_offset >= 0),
 owner_declaration_id INTEGER,
 FOREIGN KEY (owner_declaration_id, file_id) REFERENCES declarations(id, file_id),
 PRIMARY KEY (receipt_id, ordinal)
) STRICT;
CREATE INDEX receipt_expansion_sites_file ON receipt_expansion_sites(file_id, call_offset);
CREATE INDEX receipt_expansion_sites_owner ON receipt_expansion_sites(owner_declaration_id, file_id);
CREATE TABLE clue_receipts (
 case_id TEXT NOT NULL REFERENCES cases(id),
 clue_id INTEGER NOT NULL,
 receipt_id INTEGER NOT NULL,
 FOREIGN KEY (clue_id, case_id) REFERENCES clues(id, case_id),
 FOREIGN KEY (receipt_id, case_id) REFERENCES source_receipts(id, case_id),
 PRIMARY KEY (clue_id, receipt_id)
) STRICT;
CREATE INDEX clue_receipts_case ON clue_receipts(case_id);
CREATE INDEX clue_receipts_receipt ON clue_receipts(receipt_id, case_id);
CREATE TABLE commits (
 hash TEXT PRIMARY KEY CHECK (length(hash) > 0),
 committed_at INTEGER NOT NULL
) STRICT;
CREATE TABLE case_history (
 case_id TEXT NOT NULL REFERENCES cases(id),
 ordinal INTEGER NOT NULL CHECK (ordinal >= 0),
 commit_hash TEXT NOT NULL REFERENCES commits(hash),
 kind TEXT NOT NULL CHECK (kind = 'history'),
 PRIMARY KEY (case_id, ordinal),
 UNIQUE (case_id, commit_hash)
) STRICT;
CREATE INDEX case_history_commit ON case_history(commit_hash);
CREATE TABLE case_history_files (
 case_id TEXT NOT NULL,
 history_ordinal INTEGER NOT NULL,
 ordinal INTEGER NOT NULL CHECK (ordinal >= 0),
 file_id INTEGER NOT NULL REFERENCES files(id),
 FOREIGN KEY (case_id, history_ordinal) REFERENCES case_history(case_id, ordinal),
 PRIMARY KEY (case_id, history_ordinal, ordinal),
 UNIQUE (case_id, history_ordinal, file_id)
) STRICT;
CREATE INDEX case_history_files_file ON case_history_files(file_id);
CREATE TABLE suppressions (
 id INTEGER PRIMARY KEY,
 ordinal INTEGER NOT NULL UNIQUE CHECK (ordinal >= 0),
 smell TEXT NOT NULL,
 symbol TEXT NOT NULL,
 file_id INTEGER NOT NULL REFERENCES files(id),
 line INTEGER NOT NULL CHECK (line > 0),
 justification TEXT NOT NULL,
 applied INTEGER NOT NULL CHECK (applied IN (0, 1)),
 case_id TEXT REFERENCES cases(id),
 CHECK ((applied = 1 AND case_id IS NOT NULL) OR (applied = 0 AND case_id IS NULL)),
 UNIQUE (file_id, line, smell)
) STRICT;
CREATE INDEX suppressions_case ON suppressions(case_id);
CREATE TABLE warnings (
 id INTEGER PRIMARY KEY,
 ordinal INTEGER NOT NULL UNIQUE CHECK (ordinal >= 0),
 code TEXT NOT NULL,
 file_id INTEGER REFERENCES files(id),
 line INTEGER NOT NULL,
 message TEXT NOT NULL,
 CHECK ((file_id IS NULL AND line = 0) OR (file_id IS NOT NULL AND line > 0))
) STRICT;
CREATE INDEX warnings_file ON warnings(file_id);
CREATE TABLE policy_reviews (
 id TEXT PRIMARY KEY,
 note TEXT NOT NULL,
 review_prompt TEXT NOT NULL
) STRICT;
CREATE TABLE case_policy_reviews (
 case_id TEXT NOT NULL REFERENCES cases(id),
 ordinal INTEGER NOT NULL CHECK (ordinal >= 0),
 policy_id TEXT NOT NULL REFERENCES policy_reviews(id),
 PRIMARY KEY (case_id, ordinal),
 UNIQUE (case_id, policy_id)
) STRICT;
CREATE INDEX case_policy_reviews_policy ON case_policy_reviews(policy_id);
CREATE VIEW summary AS SELECT
 COALESCE(SUM(CASE WHEN verdict = 'FAIL' AND suppressed = 0 THEN 1 ELSE 0 END), 0) AS failed,
 COALESCE(SUM(CASE WHEN verdict = 'WARN' AND suppressed = 0 THEN 1 ELSE 0 END), 0) AS warned,
 COALESCE(SUM(CASE WHEN suppressed = 1 THEN 1 ELSE 0 END), 0) AS suppressed
 FROM cases;

-- Common-role inference is evidence. None of these tables contributes to summary.
CREATE TABLE role_candidates (
 id TEXT PRIMARY KEY,
 canonical_interface TEXT NOT NULL,
 confidence TEXT NOT NULL CHECK (confidence IN ('strong','supporting')),
 classification TEXT NOT NULL CHECK (classification IN ('inferred role','existing role'))
) STRICT;
CREATE TABLE role_candidate_implementations (
 candidate_id TEXT NOT NULL REFERENCES role_candidates(id),
 identity TEXT NOT NULL,
 PRIMARY KEY (candidate_id,identity)
) STRICT;
CREATE TABLE role_candidate_messages (
 candidate_id TEXT NOT NULL REFERENCES role_candidates(id),
 identity TEXT NOT NULL,
 PRIMARY KEY (candidate_id,identity)
) STRICT;
CREATE TABLE role_candidate_interfaces (
 candidate_id TEXT NOT NULL REFERENCES role_candidates(id),
 identity TEXT NOT NULL,
 relationship TEXT NOT NULL CHECK (relationship IN ('exact','compatible','used')),
 PRIMARY KEY (candidate_id,identity,relationship)
) STRICT;
CREATE TABLE role_candidate_receipts (
 candidate_id TEXT NOT NULL REFERENCES role_candidates(id),
 ordinal INTEGER NOT NULL CHECK (ordinal >= 0),
 declaration_id INTEGER NOT NULL REFERENCES declarations(id),
 kind TEXT NOT NULL,
 subject TEXT NOT NULL,
 message TEXT NOT NULL,
 start_line INTEGER NOT NULL CHECK (start_line > 0),
 end_line INTEGER NOT NULL CHECK (end_line >= start_line),
 start_offset INTEGER NOT NULL CHECK (start_offset >= 0),
 end_offset INTEGER NOT NULL CHECK (end_offset >= start_offset),
 PRIMARY KEY (candidate_id,ordinal)
) STRICT;
CREATE TABLE role_candidate_case_links (
 candidate_id TEXT NOT NULL REFERENCES role_candidates(id),
 case_id TEXT NOT NULL REFERENCES cases(id),
 PRIMARY KEY (candidate_id,case_id)
) STRICT;

-- Correlations explain existing evidence and never contribute to verdict totals.
CREATE TABLE correlations (
 id TEXT PRIMARY KEY,
 kind TEXT NOT NULL CHECK(kind = 'missing-polymorphic-role'),
 confidence TEXT NOT NULL CHECK(confidence IN ('strong','partial')),
 variant_domain TEXT NOT NULL,
 diagnosis TEXT NOT NULL,
 policy_id TEXT NOT NULL,
 policy_status TEXT NOT NULL CHECK(policy_status = 'provisional'),
 policy_note TEXT NOT NULL,
 review_prompt TEXT NOT NULL
) STRICT;
CREATE TABLE correlation_cases (
 correlation_id TEXT NOT NULL REFERENCES correlations(id),
 case_id TEXT NOT NULL REFERENCES cases(id),
 PRIMARY KEY(correlation_id,case_id)
) STRICT;
CREATE TABLE correlation_role_candidates (
 correlation_id TEXT NOT NULL REFERENCES correlations(id),
 candidate_id TEXT NOT NULL REFERENCES role_candidates(id),
 PRIMARY KEY(correlation_id,candidate_id)
) STRICT;
CREATE TABLE correlation_evidence (
 correlation_id TEXT NOT NULL,
 case_id TEXT NOT NULL,
 receipt_id INTEGER NOT NULL,
 FOREIGN KEY(correlation_id,case_id) REFERENCES correlation_cases(correlation_id,case_id),
 FOREIGN KEY(receipt_id,case_id) REFERENCES source_receipts(id,case_id),
 PRIMARY KEY(correlation_id,case_id,receipt_id)
) STRICT;
CREATE TABLE correlation_guidance (
 correlation_id TEXT NOT NULL REFERENCES correlations(id),
 kind TEXT NOT NULL CHECK(kind IN ('lead','avoid')),
 ordinal INTEGER NOT NULL CHECK(ordinal>=0),
 text TEXT NOT NULL,
 PRIMARY KEY(correlation_id,kind,ordinal)
) STRICT;
