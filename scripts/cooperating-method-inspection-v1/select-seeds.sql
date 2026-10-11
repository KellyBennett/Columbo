WITH eligible_cases AS (
  SELECT id, ordinal, smell, verdict, primary_declaration_id
  FROM cases
  WHERE suppressed = 0 AND verdict <> 'PASS' AND smell <> 'prose-comment'
), associations AS (
  SELECT id AS case_id, ordinal AS case_ordinal, smell, verdict,
         primary_declaration_id AS declaration_id, 'primary' AS source_role
  FROM eligible_cases
  UNION
  SELECT c.id, c.ordinal, c.smell, c.verdict, p.declaration_id,
         'peer:' || p.role
  FROM eligible_cases c JOIN case_declarations p ON p.case_id = c.id
)
SELECT a.*, d.symbol, f.path, d.start_line, d.end_line, d.start_offset, d.end_offset
FROM associations a JOIN declarations d ON d.id = a.declaration_id
JOIN files f ON f.id = d.file_id
ORDER BY f.path, d.start_offset, d.id, a.case_ordinal, a.case_id, a.source_role;
