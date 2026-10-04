-- Exact physical ranges shared by different cases. Keep separate expansion copies.
WITH shared AS (
  SELECT file_id, start_offset, end_offset, kind, COUNT(DISTINCT case_id) AS cases_using
  FROM source_receipts
  GROUP BY file_id, start_offset, end_offset, kind
  HAVING COUNT(DISTINCT case_id) > 1
)
SELECT f.path, r.start_line, r.end_line, r.start_offset, r.end_offset,
       r.kind, s.cases_using, c.id AS case_id, c.smell,
       r.id AS receipt_id, r.ordinal AS receipt_ordinal, r.subject, r.value, r.nesting
FROM shared s
JOIN source_receipts r ON r.file_id = s.file_id
 AND r.start_offset = s.start_offset AND r.end_offset = s.end_offset AND r.kind = s.kind
JOIN files f ON f.id = r.file_id
JOIN cases c ON c.id = r.case_id
ORDER BY f.path, r.start_offset, r.end_offset, r.kind, c.ordinal, r.ordinal;
