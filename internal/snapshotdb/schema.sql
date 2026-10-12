
CREATE TABLE report (
 id INTEGER PRIMARY KEY CHECK (id = 1),
 schema_version INTEGER NOT NULL CHECK (schema_version = 9),
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
 smell TEXT NOT NULL CHECK (smell IN ('long-function','long-parameter-list','high-cognitive-complexity','excessive-dependencies','feature-envy','data-clump','cosmetic-extraction','duplicate-code','repeated-variant-decision','selection-use-coupling','prose-comment')),
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

CREATE TABLE advisory_collectors (
 kind TEXT PRIMARY KEY CHECK (kind IN ('choice-set','nested-field-decision','variant-coordination','category-selected-behavior','repeated-guarded-update','witnessed-conditional-overwrite','category-split-updates')),
 files_analyzed INTEGER NOT NULL CHECK (files_analyzed >= 0),
 declarations_analyzed INTEGER NOT NULL CHECK (declarations_analyzed >= 0)
) STRICT;
CREATE TABLE advisory_groups (
 id TEXT PRIMARY KEY,
 kind TEXT NOT NULL REFERENCES advisory_collectors(kind),
 subject TEXT NOT NULL,
 lead TEXT NOT NULL,
 limits TEXT NOT NULL
) STRICT;
CREATE TABLE advisory_values (
 group_id TEXT NOT NULL REFERENCES advisory_groups(id),
 kind TEXT NOT NULL,
 ordinal INTEGER NOT NULL CHECK (ordinal >= 0),
 identity TEXT NOT NULL,
 value TEXT NOT NULL,
 PRIMARY KEY (group_id,kind,ordinal)
) STRICT;
CREATE TABLE advisory_sites (
 group_id TEXT NOT NULL REFERENCES advisory_groups(id),
 ordinal INTEGER NOT NULL CHECK (ordinal >= 0),
 declaration_id INTEGER NOT NULL REFERENCES declarations(id),
 representation TEXT NOT NULL,
 input_expression TEXT NOT NULL,
 PRIMARY KEY (group_id,ordinal)
) STRICT;
CREATE TABLE advisory_site_values (
 group_id TEXT NOT NULL,
 site_ordinal INTEGER NOT NULL,
 ordinal INTEGER NOT NULL CHECK (ordinal >= 0),
 identity TEXT NOT NULL,
 value TEXT NOT NULL,
 PRIMARY KEY (group_id,site_ordinal,ordinal),
 FOREIGN KEY (group_id,site_ordinal) REFERENCES advisory_sites(group_id,ordinal)
) STRICT;
CREATE TABLE advisory_receipts (
 group_id TEXT NOT NULL,
 site_ordinal INTEGER NOT NULL,
 ordinal INTEGER NOT NULL CHECK (ordinal >= 0),
 kind TEXT NOT NULL,
 subject TEXT NOT NULL,
 start_line INTEGER NOT NULL CHECK (start_line > 0),
 end_line INTEGER NOT NULL CHECK (end_line >= start_line),
 start_offset INTEGER NOT NULL CHECK (start_offset >= 0),
 end_offset INTEGER NOT NULL CHECK (end_offset >= start_offset),
 spelling TEXT NOT NULL,
 PRIMARY KEY (group_id,site_ordinal,ordinal),
 FOREIGN KEY (group_id,site_ordinal) REFERENCES advisory_sites(group_id,ordinal)
) STRICT;

CREATE TABLE refactoring_stages (
 id TEXT PRIMARY KEY,
 ordinal INTEGER NOT NULL UNIQUE CHECK (ordinal >= 0),
 name TEXT NOT NULL,
 task TEXT NOT NULL,
 state TEXT NOT NULL CHECK (state IN ('cleared','active','locked')),
 pending_definition INTEGER NOT NULL CHECK (pending_definition IN (0,1)),
 issue_count INTEGER NOT NULL CHECK (issue_count >= 0),
 CHECK (state <> 'cleared' OR (pending_definition = 0 AND issue_count = 0))
) STRICT;
CREATE UNIQUE INDEX one_active_stage ON refactoring_stages(state) WHERE state='active';
CREATE TABLE stage_collectors (
 stage_id TEXT NOT NULL REFERENCES refactoring_stages(id),
 ordinal INTEGER NOT NULL CHECK (ordinal >= 0),
 collector TEXT NOT NULL REFERENCES advisory_collectors(kind),
 issue_count INTEGER NOT NULL CHECK (issue_count >= 0),
 PRIMARY KEY (stage_id, collector),
 UNIQUE (stage_id, ordinal)
) STRICT;
CREATE VIEW active_stage_issues AS
 SELECT g.* FROM advisory_groups g
 JOIN stage_collectors m ON m.collector=g.kind
 JOIN refactoring_stages s ON s.id=m.stage_id WHERE s.state='active';

CREATE TABLE source_scan (
 id INTEGER PRIMARY KEY CHECK (id = 1) REFERENCES report(id),
 root TEXT NOT NULL CHECK (length(root) > 0),
 go_version TEXT NOT NULL CHECK (length(go_version) > 0),
 source_sha256 TEXT NOT NULL CHECK (length(source_sha256) = 64 AND source_sha256 NOT GLOB '*[^0-9a-f]*')
) STRICT;
CREATE TABLE source_environment (
 scan_id INTEGER NOT NULL DEFAULT 1 REFERENCES source_scan(id),
 name TEXT NOT NULL CHECK (length(name) > 0),
 value TEXT NOT NULL,
 PRIMARY KEY (scan_id, name)
) STRICT;
CREATE TABLE source_patterns (
 scan_id INTEGER NOT NULL DEFAULT 1 REFERENCES source_scan(id),
 ordinal INTEGER NOT NULL CHECK (ordinal >= 0),
 pattern TEXT NOT NULL,
 PRIMARY KEY (scan_id, ordinal)
) STRICT;
CREATE TABLE source_files (
 file_id INTEGER PRIMARY KEY REFERENCES files(id),
 scan_id INTEGER NOT NULL DEFAULT 1 REFERENCES source_scan(id),
 sha256 TEXT NOT NULL CHECK (length(sha256) = 64 AND sha256 NOT GLOB '*[^0-9a-f]*'),
 size INTEGER NOT NULL CHECK (size >= 0),
 package TEXT NOT NULL,
 selected_runtime INTEGER NOT NULL CHECK (selected_runtime IN (0, 1)),
 ordinary_target INTEGER NOT NULL CHECK (ordinary_target IN (0, 1)),
 generated INTEGER NOT NULL CHECK (generated IN (0, 1)),
 scope TEXT NOT NULL CHECK (length(scope) > 0),
 CHECK (ordinary_target <= selected_runtime),
 CHECK (selected_runtime = 0 OR length(package) > 0),
 UNIQUE (file_id, sha256)
) STRICT;
CREATE INDEX source_files_scope ON source_files(selected_runtime, ordinary_target);
CREATE TABLE source_config (
 scan_id INTEGER PRIMARY KEY DEFAULT 1 REFERENCES source_scan(id),
 config_path TEXT NOT NULL CHECK (length(config_path) > 0),
 config_file_id INTEGER,
 config_sha256 TEXT,
 staged INTEGER NOT NULL CHECK (staged IN (0, 1)),
 history INTEGER NOT NULL CHECK (history IN (0, 1)),
 max_commits INTEGER NOT NULL CHECK (max_commits > 0),
 CHECK ((config_file_id IS NULL) = (config_sha256 IS NULL)),
 FOREIGN KEY (config_file_id, config_sha256) REFERENCES source_files(file_id, sha256)
) STRICT;
CREATE TABLE source_config_severity (
 scan_id INTEGER NOT NULL DEFAULT 1 REFERENCES source_config(scan_id),
 smell TEXT NOT NULL,
 severity TEXT NOT NULL CHECK (severity IN ('off', 'warn', 'fail')),
 PRIMARY KEY (scan_id, smell)
) STRICT;
CREATE TABLE source_config_counts (
 scan_id INTEGER NOT NULL DEFAULT 1 REFERENCES source_config(scan_id),
 name TEXT NOT NULL,
 value INTEGER NOT NULL CHECK (value > 0),
 PRIMARY KEY (scan_id, name)
) STRICT;
CREATE TABLE source_config_ratios (
 scan_id INTEGER NOT NULL DEFAULT 1 REFERENCES source_config(scan_id),
 name TEXT NOT NULL,
 value REAL NOT NULL CHECK (value >= 0),
 PRIMARY KEY (scan_id, name)
) STRICT;
CREATE TABLE source_config_exclusions (
 scan_id INTEGER NOT NULL DEFAULT 1 REFERENCES source_config(scan_id),
 ordinal INTEGER NOT NULL CHECK (ordinal >= 0),
 pattern TEXT NOT NULL,
 PRIMARY KEY (scan_id, ordinal)
) STRICT;
CREATE TABLE source_ranges (
 id INTEGER PRIMARY KEY,
 file_id INTEGER NOT NULL REFERENCES source_files(file_id),
 start_line INTEGER NOT NULL CHECK (start_line > 0),
 end_line INTEGER NOT NULL CHECK (end_line >= start_line),
 start_offset INTEGER NOT NULL CHECK (start_offset >= 0),
 end_offset INTEGER NOT NULL CHECK (end_offset >= start_offset),
 sha256 TEXT NOT NULL CHECK (length(sha256) = 64 AND sha256 NOT GLOB '*[^0-9a-f]*'),
 UNIQUE (file_id, start_offset, end_offset),
 UNIQUE (id, file_id)
) STRICT;
CREATE TRIGGER source_ranges_file_bounds BEFORE INSERT ON source_ranges WHEN NEW.end_offset > (SELECT size FROM source_files WHERE file_id = NEW.file_id) BEGIN SELECT RAISE(ABORT, 'source range exceeds file size'); END;
CREATE TRIGGER source_ranges_file_bounds_update BEFORE UPDATE ON source_ranges WHEN NEW.end_offset > (SELECT size FROM source_files WHERE file_id = NEW.file_id) BEGIN SELECT RAISE(ABORT, 'source range exceeds file size'); END;
CREATE TRIGGER source_files_range_bounds_update BEFORE UPDATE OF size ON source_files WHEN EXISTS (SELECT 1 FROM source_ranges WHERE file_id = NEW.file_id AND end_offset > NEW.size) OR EXISTS (SELECT 1 FROM source_declarations sd JOIN declarations d ON d.id = sd.declaration_id WHERE sd.file_id = NEW.file_id AND d.end_offset > NEW.size) BEGIN SELECT RAISE(ABORT, 'source file size excludes stored range'); END;
CREATE TABLE source_types (
 id INTEGER PRIMARY KEY,
 type_key TEXT NOT NULL UNIQUE CHECK (length(type_key) > 0),
 resolved_key TEXT NOT NULL,
 alias INTEGER NOT NULL CHECK (alias IN (0, 1)),
 file_id INTEGER NOT NULL REFERENCES source_files(file_id),
 range_id INTEGER NOT NULL,
 FOREIGN KEY (range_id, file_id) REFERENCES source_ranges(id, file_id),
 UNIQUE (id, file_id),
 UNIQUE (id, type_key)
) STRICT;
CREATE TABLE source_fields (
 id INTEGER PRIMARY KEY,
 type_id INTEGER NOT NULL,
 file_id INTEGER NOT NULL REFERENCES source_files(file_id),
 ordinal INTEGER NOT NULL CHECK (ordinal >= 0),
 range_id INTEGER NOT NULL,
 embedded INTEGER NOT NULL CHECK (embedded IN (0, 1)),
 type_text TEXT NOT NULL,
 type_key TEXT,
 target_type_id INTEGER,
 FOREIGN KEY (type_id, file_id) REFERENCES source_types(id, file_id),
 FOREIGN KEY (range_id, file_id) REFERENCES source_ranges(id, file_id),
 FOREIGN KEY (target_type_id, type_key) REFERENCES source_types(id, type_key),
 CHECK (target_type_id IS NULL OR type_key IS NOT NULL),
 UNIQUE (type_id, ordinal)
) STRICT;
CREATE INDEX source_fields_target ON source_fields(target_type_id);
CREATE TABLE source_field_names (
 field_id INTEGER NOT NULL REFERENCES source_fields(id),
 ordinal INTEGER NOT NULL CHECK (ordinal >= 0),
 name TEXT NOT NULL CHECK (length(name) > 0),
 PRIMARY KEY (field_id, ordinal)
) STRICT;
CREATE TABLE source_declarations (
 declaration_id INTEGER PRIMARY KEY,
 file_id INTEGER NOT NULL REFERENCES source_files(file_id),
 kind TEXT NOT NULL CHECK (kind IN ('function', 'method')),
 receiver_type_id INTEGER REFERENCES source_types(id),
 sha256 TEXT NOT NULL CHECK (length(sha256) = 64 AND sha256 NOT GLOB '*[^0-9a-f]*'),
 FOREIGN KEY (declaration_id, file_id) REFERENCES declarations(id, file_id),
 CHECK ((kind = 'method') = (receiver_type_id IS NOT NULL)),
 UNIQUE (declaration_id, file_id)
) STRICT;
CREATE INDEX source_declarations_receiver ON source_declarations(receiver_type_id);
CREATE TRIGGER source_declarations_file_bounds BEFORE INSERT ON source_declarations WHEN (SELECT end_offset FROM declarations WHERE id = NEW.declaration_id) > (SELECT size FROM source_files WHERE file_id = NEW.file_id) BEGIN SELECT RAISE(ABORT, 'source declaration exceeds file size'); END;
CREATE TRIGGER source_declarations_file_bounds_update BEFORE UPDATE ON source_declarations WHEN (SELECT end_offset FROM declarations WHERE id = NEW.declaration_id) > (SELECT size FROM source_files WHERE file_id = NEW.file_id) BEGIN SELECT RAISE(ABORT, 'source declaration exceeds file size'); END;
CREATE TRIGGER declarations_source_bounds_update BEFORE UPDATE OF end_offset ON declarations WHEN EXISTS (SELECT 1 FROM source_declarations sd JOIN source_files sf ON sf.file_id = sd.file_id WHERE sd.declaration_id = OLD.id AND NEW.end_offset > sf.size) BEGIN SELECT RAISE(ABORT, 'source declaration exceeds file size'); END;
CREATE TABLE source_calls (
 id INTEGER PRIMARY KEY,
 declaration_id INTEGER NOT NULL,
 file_id INTEGER NOT NULL REFERENCES source_files(file_id),
 ordinal INTEGER NOT NULL CHECK (ordinal >= 0),
 range_id INTEGER NOT NULL,
 expression TEXT NOT NULL,
 resolution TEXT NOT NULL CHECK (length(resolution) > 0),
 target_declaration_id INTEGER REFERENCES source_declarations(declaration_id),
 receiver_form TEXT NOT NULL CHECK (receiver_form IN ('', 'method-value-or-invocation', 'method-expression')),
 receiver_range_id INTEGER,
 binding_range_id INTEGER REFERENCES source_ranges(id),
 binding_limit TEXT NOT NULL,
 FOREIGN KEY (declaration_id, file_id) REFERENCES source_declarations(declaration_id, file_id),
 FOREIGN KEY (range_id, file_id) REFERENCES source_ranges(id, file_id),
 FOREIGN KEY (receiver_range_id, file_id) REFERENCES source_ranges(id, file_id),
 CHECK (binding_range_id IS NULL OR receiver_range_id IS NOT NULL),
 CHECK ((receiver_range_id IS NULL) = (receiver_form = '')),
 UNIQUE (declaration_id, ordinal)
) STRICT;
CREATE INDEX source_calls_target ON source_calls(target_declaration_id);
CREATE VIEW source_receiver_seeds AS
 WITH case_links AS (
  SELECT id AS case_id, primary_declaration_id AS declaration_id, 'primary' AS role FROM cases
  UNION SELECT case_id, declaration_id, role FROM case_declarations
 )
 SELECT c.id AS case_id, c.smell, c.verdict, l.role, d.id AS declaration_id,
        sd.receiver_type_id, t.type_key, d.symbol, f.path,
        d.start_line, d.end_line, d.start_offset, d.end_offset, sd.sha256,
        sf.sha256 AS file_sha256, sf.ordinary_target, sf.scope
 FROM cases c JOIN case_links l ON l.case_id = c.id
 JOIN declarations d ON d.id = l.declaration_id
 JOIN source_declarations sd ON sd.declaration_id = d.id
 JOIN source_types t ON t.id = sd.receiver_type_id
 JOIN files f ON f.id = d.file_id JOIN source_files sf ON sf.file_id = f.id
 WHERE c.suppressed = 0 AND c.smell <> 'prose-comment' AND sd.kind = 'method' AND sf.ordinary_target = 1;
CREATE VIEW source_receiver_siblings AS
 SELECT sd.receiver_type_id, t.type_key, d.id AS declaration_id, d.symbol, f.path,
        d.start_line, d.end_line, d.start_offset, d.end_offset, sd.sha256,
        sf.sha256 AS file_sha256, sf.ordinary_target, sf.generated, sf.scope
 FROM source_declarations sd JOIN declarations d ON d.id = sd.declaration_id
 JOIN source_types t ON t.id = sd.receiver_type_id
 JOIN files f ON f.id = d.file_id JOIN source_files sf ON sf.file_id = f.id
 WHERE sd.kind = 'method' AND sd.receiver_type_id IN (SELECT receiver_type_id FROM source_receiver_seeds);
CREATE VIEW source_storage_paths AS
 WITH RECURSIVE paths(receiver_type_id, owner_type_id, field_id, target_type_id, depth, field_path, type_path, relation) AS (
  SELECT roots.receiver_type_id, f.type_id AS owner_type_id, f.id AS field_id, f.target_type_id, 1 AS depth,
         '/' || f.id || '/' AS field_path, '/' || f.type_id || '/' || COALESCE(f.target_type_id || '/', '') AS type_path,
         CASE WHEN f.embedded = 0 THEN 'held-field'
              WHEN f.target_type_id IS NULL THEN 'unresolved-embedding'
              WHEN f.target_type_id = f.type_id THEN 'cycle-boundary'
              ELSE 'declared-embedding' END AS relation
  FROM (SELECT DISTINCT receiver_type_id FROM source_receiver_seeds) roots
  JOIN source_fields f ON f.type_id = roots.receiver_type_id
  UNION ALL
  SELECT p.receiver_type_id, f.type_id, f.id, f.target_type_id, p.depth + 1,
         p.field_path || f.id || '/', p.type_path || COALESCE(f.target_type_id || '/', ''),
         CASE WHEN f.embedded = 0 THEN 'held-field'
              WHEN f.target_type_id IS NULL THEN 'unresolved-embedding'
              WHEN instr(p.type_path, '/' || f.target_type_id || '/') > 0 THEN 'cycle-boundary'
              ELSE 'declared-embedding' END
  FROM paths p JOIN source_fields f ON f.type_id = p.target_type_id
  WHERE p.relation = 'declared-embedding'
 )
 SELECT p.receiver_type_id, p.owner_type_id, p.field_id, p.target_type_id, p.depth,
        CAST(p.field_path AS TEXT) AS field_path, p.relation, t.type_key AS owner_type, f.ordinal, f.embedded,
        f.type_text, f.type_key AS field_type, fp.path, r.start_line, r.end_line,
        r.start_offset, r.end_offset, r.sha256, sf.sha256 AS file_sha256
 FROM paths p JOIN source_fields f ON f.id = p.field_id
 JOIN source_types t ON t.id = p.owner_type_id JOIN source_ranges r ON r.id = f.range_id
 JOIN files fp ON fp.id = r.file_id JOIN source_files sf ON sf.file_id = fp.id;
CREATE VIEW source_one_hop_calls AS
 SELECT caller.receiver_type_id, c.id AS call_id, c.declaration_id, caller.symbol AS caller_symbol,
        c.ordinal, c.expression, c.resolution,
        CASE WHEN c.target_declaration_id IS NULL THEN 'unresolved' ELSE 'local-one-hop' END AS relation,
        f.path, r.start_line, r.end_line, r.start_offset, r.end_offset, r.sha256,
        sf.sha256 AS file_sha256, c.target_declaration_id, target.symbol AS target_symbol,
        tf.path AS target_path, target.start_line AS target_start_line, target.end_line AS target_end_line,
        target.start_offset AS target_start_offset, target.end_offset AS target_end_offset,
        td.sha256 AS target_sha256, tsf.sha256 AS target_file_sha256,
        c.receiver_form, c.receiver_range_id, c.binding_range_id, c.binding_limit
 FROM source_calls c JOIN source_receiver_siblings caller ON caller.declaration_id = c.declaration_id
 JOIN source_ranges r ON r.id = c.range_id JOIN files f ON f.id = r.file_id
 JOIN source_files sf ON sf.file_id = f.id
 LEFT JOIN declarations target ON target.id = c.target_declaration_id
 LEFT JOIN source_declarations td ON td.declaration_id = target.id
 LEFT JOIN files tf ON tf.id = target.file_id LEFT JOIN source_files tsf ON tsf.file_id = tf.id;
