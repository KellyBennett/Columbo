-- A physical cluster is case-scoped; use cluster/member ordinals, not display subjects.
SELECT c.id AS case_id, cl.id AS cluster_id, cl.ordinal AS cluster_ordinal,
       cl.cluster_key, owner.symbol AS owner, f.path,
       m.id AS member_id, m.ordinal AS member_ordinal,
       helper.symbol AS helper, m.call_offset
FROM cases c
JOIN clusters cl ON cl.case_id = c.id
JOIN declarations owner ON owner.id = cl.owner_declaration_id
JOIN files f ON f.id = cl.file_id
JOIN cluster_members m ON m.cluster_id = cl.id AND m.case_id = cl.case_id
JOIN declarations helper ON helper.id = m.helper_declaration_id
WHERE :case_id IS NULL OR c.id = :case_id
ORDER BY c.ordinal, cl.ordinal, m.ordinal;
