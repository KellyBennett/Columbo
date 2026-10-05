-- Both kinds of dependency evidence; scored=0 never consumes collaborator budget.
SELECT c.id AS case_id, d.symbol, dep.identity, dd.scored,
       r.kind, f.path, r.start_line, r.start_offset, r.end_offset, r.ordinal,
       (SELECT group_concat(origin, ", ") FROM (
          SELECT DISTINCT q.kind AS origin FROM clues q
          JOIN clue_receipts cr ON cr.clue_id=q.id AND cr.case_id=q.case_id
          JOIN clue_values cv ON cv.clue_id=q.id AND cv.value=dep.identity
          WHERE cr.receipt_id=r.id AND q.case_id=c.id
            AND q.kind LIKE 'dependency-use-%' ORDER BY q.kind
       )) AS use_origins
FROM dependency_receipts dr
JOIN declaration_dependencies dd ON dd.declaration_id = dr.declaration_id
 AND dd.dependency_id = dr.dependency_id
JOIN declarations d ON d.id = dr.declaration_id
JOIN dependencies dep ON dep.id = dr.dependency_id
JOIN source_receipts r ON r.id = dr.receipt_id AND r.case_id = dr.case_id
JOIN files f ON f.id = r.file_id
JOIN cases c ON c.id = dr.case_id
WHERE :case_id IS NULL OR dr.case_id = :case_id
ORDER BY c.ordinal, d.symbol, dep.identity, r.ordinal;
