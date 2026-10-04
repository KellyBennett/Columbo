-- :case_id NULL (or unset in sqlite3) examines every case.
-- Only explicit clue_receipts links establish support; shared case ownership does not.
SELECT c.id AS case_id, q.ordinal AS clue_ordinal, q.kind, q.subject,
       q.numeric_value AS stored_metric, COUNT(r.id) AS contribution_receipts,
       COALESCE(SUM(r.value), 0) AS contribution_sum,
       q.numeric_value = COALESCE(SUM(r.value), 0) AS reconciles
FROM clues q
JOIN cases c ON c.id = q.case_id
LEFT JOIN clue_receipts cr ON cr.clue_id = q.id AND cr.case_id = q.case_id
LEFT JOIN source_receipts r ON r.id = cr.receipt_id AND r.kind = 'metric-contribution'
WHERE q.kind IN ('function-lines', 'expanded-lines', 'cognitive-complexity', 'expanded-complexity')
  AND (:case_id IS NULL OR q.case_id = :case_id)
GROUP BY q.id
ORDER BY c.ordinal, q.ordinal;
