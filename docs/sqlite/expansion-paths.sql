-- Both ordered paths for every copied receipt. Do not cross-product the child tables.
-- receipt_id distinguishes copies even when source range/declaration chains coincide.
WITH path_steps AS (
  SELECT r.id AS receipt_id, r.case_id, r.ordinal AS receipt_ordinal,
         'declaration' AS path_kind, x.ordinal AS step_ordinal,
         x.symbol, f.path, NULL AS call_offset
  FROM source_receipts r
  JOIN receipt_expansion_declarations x ON x.receipt_id = r.id
  JOIN declarations d ON d.id = x.declaration_id
  JOIN files f ON f.id = d.file_id
  UNION ALL
  SELECT r.id, r.case_id, r.ordinal, 'call-site', x.ordinal,
         d.symbol, f.path, x.call_offset
  FROM source_receipts r
  JOIN receipt_expansion_sites x ON x.receipt_id = r.id
  JOIN files f ON f.id = x.file_id
  LEFT JOIN declarations d ON d.id = x.owner_declaration_id
)
SELECT c.id AS case_id, p.receipt_id, p.receipt_ordinal, r.kind,
       rf.path AS receipt_file, r.start_offset, r.end_offset, r.value, r.nesting,
       p.path_kind, p.step_ordinal, p.symbol, p.path, p.call_offset
FROM path_steps p
JOIN cases c ON c.id = p.case_id
JOIN source_receipts r ON r.id = p.receipt_id
JOIN files rf ON rf.id = r.file_id
WHERE :case_id IS NULL OR p.case_id = :case_id
ORDER BY c.ordinal, p.receipt_ordinal,
         CASE p.path_kind WHEN 'declaration' THEN 0 ELSE 1 END, p.step_ordinal;
