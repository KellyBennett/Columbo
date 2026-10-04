-- Preserve stored verdicts plus applied/unused suppression and policy explanations.
SELECT c.id AS case_id, c.smell, d.symbol, c.verdict, c.suppressed,
       f.path AS directive_file, s.line AS directive_line, s.justification,
       p.ordinal AS policy_ordinal, pr.id AS policy_id, pr.note, pr.review_prompt
FROM cases c
JOIN declarations d ON d.id = c.primary_declaration_id
LEFT JOIN suppressions s ON s.case_id = c.id AND s.applied = 1
LEFT JOIN files f ON f.id = s.file_id
LEFT JOIN case_policy_reviews p ON p.case_id = c.id
LEFT JOIN policy_reviews pr ON pr.id = p.policy_id
WHERE :case_id IS NULL OR c.id = :case_id
ORDER BY c.ordinal, s.ordinal, p.ordinal;
SELECT s.smell, s.symbol, f.path, s.line, s.justification, s.applied, s.case_id
FROM suppressions s JOIN files f ON f.id = s.file_id
WHERE s.applied = 0
ORDER BY s.ordinal;
SELECT w.code, COALESCE(f.path, '') AS path, w.line, w.message
FROM warnings w LEFT JOIN files f ON f.id = w.file_id
ORDER BY w.ordinal;
