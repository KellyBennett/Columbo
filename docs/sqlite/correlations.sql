-- Non-enforcing diagnoses with member status; one row per member.
SELECT co.id, co.kind, co.confidence, co.variant_domain, co.diagnosis,
       c.id AS case_id, c.smell, c.verdict, c.suppressed,
       co.policy_id, co.policy_status, co.policy_note, co.review_prompt
FROM correlations co
JOIN correlation_cases cc ON cc.correlation_id = co.id
JOIN cases c ON c.id = cc.case_id
ORDER BY co.id, c.id;

-- Exact variant-to-player mappings, retaining originating case and source.
SELECT ce.correlation_id, ce.case_id, r.subject AS variant,
       r.spelling AS implementation, f.path, r.start_line
FROM correlation_evidence ce
JOIN source_receipts r ON r.id = ce.receipt_id AND r.case_id = ce.case_id
JOIN files f ON f.id = r.file_id
WHERE r.kind = 'selection-variant-mapping'
ORDER BY ce.correlation_id, r.subject, r.spelling, f.path, r.start_line;
