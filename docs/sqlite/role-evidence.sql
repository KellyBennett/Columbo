-- Minimal behavioral surfaces, including observations without an enforceable case.
SELECT id, confidence, classification, canonical_interface
FROM role_candidates ORDER BY id;

-- Players and their common observed messages are independent ordered relations.
SELECT candidate_id, identity FROM role_candidate_implementations
ORDER BY candidate_id, identity;
SELECT candidate_id, identity FROM role_candidate_messages
ORDER BY candidate_id, identity;

-- Preserve exact/superset and already-used distinctions.
SELECT candidate_id, relationship, identity FROM role_candidate_interfaces
ORDER BY candidate_id, relationship, identity;

-- Show the source proof without relying on presentation strings for ownership.
SELECT r.candidate_id, f.path, d.symbol, r.kind, r.subject, r.message,
       r.start_line, r.end_line, r.start_offset, r.end_offset
FROM role_candidate_receipts r
JOIN declarations d ON d.id = r.declaration_id
JOIN files f ON f.id = d.file_id
ORDER BY r.candidate_id, r.ordinal;

-- Only existing cases supply verdicts.
SELECT l.candidate_id, c.id AS case_id, c.smell, c.verdict, c.suppressed
FROM role_candidate_case_links l JOIN cases c ON c.id = l.case_id
ORDER BY l.candidate_id, c.id;
