-- Report identity, outcome totals, tables/views, columns, indexes, and declared keys.
PRAGMA application_id;
PRAGMA user_version;
SELECT id, schema_version, columbo_version FROM report ORDER BY id;
SELECT failed, warned, suppressed FROM summary;
SELECT type, name, sql FROM sqlite_schema
WHERE name NOT LIKE 'sqlite_%' ORDER BY type, name;
SELECT s.name AS table_name, p.cid, p.name AS column_name, p.type,
       p."notnull", p.dflt_value, p.pk
FROM sqlite_schema s JOIN pragma_table_info(s.name) p
WHERE s.type IN ('table', 'view') AND s.name NOT LIKE 'sqlite_%'
ORDER BY s.name, p.cid;
SELECT s.name AS table_name, fk.id AS foreign_key_id, fk.seq,
       fk."from" AS source_column, fk."table" AS target_table,
       fk."to" AS target_column
FROM sqlite_schema s JOIN pragma_foreign_key_list(s.name) fk
WHERE s.type = 'table' AND s.name NOT LIKE 'sqlite_%'
ORDER BY s.name, fk.id, fk.seq;
PRAGMA integrity_check;
PRAGMA foreign_key_check;
