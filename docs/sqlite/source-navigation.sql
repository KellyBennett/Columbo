SELECT s.root, s.go_version, s.source_sha256, c.config_path, c.config_sha256,
       c.staged, c.history, c.max_commits
FROM source_scan s JOIN source_config c ON c.scan_id = s.id;

SELECT f.path, s.sha256, s.size, s.package, s.selected_runtime,
       s.ordinary_target, s.generated, s.scope
FROM source_files s JOIN files f ON f.id = s.file_id
ORDER BY f.path;

SELECT case_id, smell, verdict, role, type_key, declaration_id, symbol, path,
       start_line, end_line, start_offset, end_offset, sha256, file_sha256
FROM source_receiver_seeds
ORDER BY type_key, case_id, path, start_offset, role;

SELECT type_key, declaration_id, symbol, path, start_line, end_line,
       start_offset, end_offset, sha256, file_sha256, ordinary_target, generated, scope
FROM source_receiver_siblings
ORDER BY type_key, path, start_offset, symbol;

SELECT t.type_key AS receiver_type, p.field_path, p.depth, p.owner_type,
       p.field_id, n.ordinal AS name_ordinal, n.name, p.embedded, p.type_text,
       p.field_type, p.target_type_id, p.relation, p.path,
       p.start_line, p.end_line, p.start_offset, p.end_offset, p.sha256, p.file_sha256
FROM source_storage_paths p JOIN source_types t ON t.id = p.receiver_type_id
LEFT JOIN source_field_names n ON n.field_id = p.field_id
ORDER BY t.type_key, p.field_path, n.ordinal;

SELECT t.type_key AS receiver_type, c.caller_symbol, c.expression, c.relation,
       c.resolution, c.path, c.start_line, c.end_line, c.start_offset, c.end_offset,
       c.sha256, c.file_sha256, c.target_declaration_id, c.target_symbol,
       c.target_path, c.target_start_line, c.target_end_line,
       c.target_start_offset, c.target_end_offset, c.target_sha256, c.target_file_sha256,
       c.receiver_form, c.receiver_range_id, c.binding_range_id, c.binding_limit
FROM source_one_hop_calls c JOIN source_types t ON t.id = c.receiver_type_id
ORDER BY t.type_key, c.path, c.start_offset, c.ordinal;

SELECT c.call_id, 'receiver' AS relation, f.path, sf.sha256 AS file_sha256,
       r.start_line, r.end_line, r.start_offset, r.end_offset, r.sha256
FROM source_one_hop_calls c JOIN source_ranges r ON r.id = c.receiver_range_id
JOIN files f ON f.id = r.file_id JOIN source_files sf ON sf.file_id = f.id
UNION ALL
SELECT c.call_id, 'lexical-binding' AS relation, f.path, sf.sha256 AS file_sha256,
       r.start_line, r.end_line, r.start_offset, r.end_offset, r.sha256
FROM source_one_hop_calls c JOIN source_ranges r ON r.id = c.binding_range_id
JOIN files f ON f.id = r.file_id JOIN source_files sf ON sf.file_id = f.id
ORDER BY call_id, relation;
