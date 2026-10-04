-- Typed numeric/list clues, support links, and pair/member relationships.
-- LEFT JOIN keeps an empty list visible: value_type=list and value_ordinal=NULL.
SELECT c.id AS case_id, q.ordinal AS clue_ordinal, q.kind, q.subject,
       q.value_type, q.numeric_value, v.ordinal AS value_ordinal, v.value AS list_value,
       q.limit_type, q.limit_value, q.operator,
       d.symbol AS declaration, cl.cluster_key,
       member.symbol AS member, left_member.symbol AS pair_left,
       right_member.symbol AS pair_right
FROM clues q
JOIN cases c ON c.id = q.case_id
LEFT JOIN clue_values v ON v.clue_id = q.id
LEFT JOIN declarations d ON d.id = q.declaration_id
LEFT JOIN clusters cl ON cl.id = q.cluster_id AND cl.case_id = q.case_id
LEFT JOIN cluster_members m ON m.id = q.member_id
LEFT JOIN declarations member ON member.id = m.helper_declaration_id
LEFT JOIN cluster_members ml ON ml.id = q.pair_left_member_id
LEFT JOIN declarations left_member ON left_member.id = ml.helper_declaration_id
LEFT JOIN cluster_members mr ON mr.id = q.pair_right_member_id
LEFT JOIN declarations right_member ON right_member.id = mr.helper_declaration_id
WHERE :case_id IS NULL OR q.case_id = :case_id
ORDER BY c.ordinal, q.ordinal, v.ordinal;
SELECT c.id AS case_id, q.ordinal AS clue_ordinal, q.kind,
       r.id AS receipt_id, r.ordinal AS receipt_ordinal, r.kind AS receipt_kind,
       f.path, r.start_line, r.end_line, r.start_offset, r.end_offset,
       r.subject, r.value, r.nesting
FROM clue_receipts cr
JOIN clues q ON q.id = cr.clue_id AND q.case_id = cr.case_id
JOIN cases c ON c.id = cr.case_id
JOIN source_receipts r ON r.id = cr.receipt_id AND r.case_id = cr.case_id
JOIN files f ON f.id = r.file_id
WHERE :case_id IS NULL OR cr.case_id = :case_id
ORDER BY c.ordinal, q.ordinal, r.ordinal;
