-- Evidence-bearing declarations only; inventory-only identities never count.
WITH shared AS (
  SELECT dependency_id, COUNT(DISTINCT declaration_id) AS users
  FROM declaration_dependencies
  WHERE scored = 1
  GROUP BY dependency_id
  HAVING COUNT(DISTINCT declaration_id) > 1
)
SELECT dep.identity, s.users, d.symbol, f.path, d.start_line
FROM shared s
JOIN dependencies dep ON dep.id = s.dependency_id
JOIN declaration_dependencies dd ON dd.dependency_id = s.dependency_id AND dd.scored = 1
JOIN declarations d ON d.id = dd.declaration_id
JOIN files f ON f.id = d.file_id
ORDER BY s.users DESC, dep.identity, f.path, d.start_offset, d.symbol;
