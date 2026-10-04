-- Stored outcomes drive enforcement. Never recompute a verdict from rounded clues.
SELECT c.id AS case_id, c.smell, d.symbol, f.path, c.start_line, c.end_line,
       c.verdict, c.why, c.diagnosis
FROM cases c
JOIN declarations d ON d.id = c.primary_declaration_id
JOIN files f ON f.id = d.file_id
WHERE c.verdict = 'FAIL' AND c.suppressed = 0
ORDER BY c.ordinal;
