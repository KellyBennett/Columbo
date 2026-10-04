-- The same commit can have different exact matching-file subsets for each case.
SELECT c.id AS case_id, h.ordinal AS history_ordinal, h.commit_hash,
       cm.committed_at, hf.ordinal AS file_ordinal, f.path
FROM cases c
JOIN case_history h ON h.case_id = c.id
JOIN commits cm ON cm.hash = h.commit_hash
JOIN case_history_files hf ON hf.case_id = h.case_id AND hf.history_ordinal = h.ordinal
JOIN files f ON f.id = hf.file_id
WHERE :case_id IS NULL OR c.id = :case_id
ORDER BY c.ordinal, h.ordinal, hf.ordinal;
