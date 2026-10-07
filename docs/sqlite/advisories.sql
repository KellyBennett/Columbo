SELECT kind, files_analyzed, declarations_analyzed FROM advisory_collectors ORDER BY kind;
SELECT id, kind, subject, lead, limits FROM advisory_groups ORDER BY kind,id;
SELECT group_id, kind, ordinal, identity, value FROM advisory_values ORDER BY group_id,kind,ordinal;
SELECT s.group_id, s.ordinal, d.symbol, f.path, s.representation, s.input_expression
FROM advisory_sites s JOIN declarations d ON d.id=s.declaration_id
JOIN files f ON f.id=d.file_id ORDER BY s.group_id,s.ordinal;
SELECT group_id,site_ordinal,ordinal,identity,value FROM advisory_site_values ORDER BY group_id,site_ordinal,ordinal;
SELECT r.group_id,r.site_ordinal,r.kind,f.path,r.start_line,r.end_line,r.start_offset,r.end_offset,r.subject,r.spelling
FROM advisory_receipts r JOIN advisory_sites s ON s.group_id=r.group_id AND s.ordinal=r.site_ordinal
JOIN declarations d ON d.id=s.declaration_id JOIN files f ON f.id=d.file_id
ORDER BY r.group_id,r.site_ordinal,r.ordinal;
