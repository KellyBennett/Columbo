SELECT d.id AS declaration_id, d.symbol, f.path, d.start_line, d.end_line,
       MAX(CASE WHEN l.kind = 'cognitive-complexity' THEN l.numeric_value END) AS cognitive_complexity,
       MAX(CASE WHEN l.kind = 'function-lines' THEN l.numeric_value END) AS function_lines
FROM cases c
JOIN declarations d ON d.id = c.primary_declaration_id
JOIN files f ON f.id = d.file_id
JOIN clues l ON l.case_id = c.id
WHERE c.smell IN ('long-function', 'high-cognitive-complexity')
  AND c.suppressed = 0 AND c.verdict <> 'PASS'
GROUP BY d.id
HAVING COUNT(DISTINCT c.smell) = 2
ORDER BY cognitive_complexity DESC, function_lines DESC, f.path, d.symbol;
