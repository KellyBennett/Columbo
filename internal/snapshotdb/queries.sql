-- name: InsertReport :exec
INSERT INTO report(id,schema_version,columbo_version) VALUES (1, ?, ?);

-- name: InsertFile :execlastid
INSERT INTO files(path) VALUES (?);

-- name: InsertDependency :execlastid
INSERT INTO dependencies(identity) VALUES (?);

-- name: InsertDeclaration :execlastid
INSERT INTO declarations(file_id,symbol,start_line,end_line,start_offset,end_offset) VALUES (?,?,?,?,?,?);

-- name: InsertDeclarationDependency :exec
INSERT INTO declaration_dependencies(declaration_id,dependency_id,scored) VALUES (?,?,?);

-- name: InsertCase :exec
INSERT INTO cases(id,ordinal,primary_declaration_id,smell,verdict,suppressed,start_line,end_line,why,diagnosis) VALUES (?,?,?,?,?,?,?,?,?,?);

-- name: LinkCaseDeclaration :exec
INSERT OR IGNORE INTO case_declarations(case_id,declaration_id,role) VALUES (?,?,?);

-- name: InsertGuidance :exec
INSERT INTO case_guidance(case_id,kind,ordinal,item) VALUES (?,?,?,?);

-- name: InsertCluster :execlastid
INSERT INTO clusters(case_id,cluster_key,owner_declaration_id,file_id,ordinal) VALUES (?,?,?,?,?);

-- name: InsertClusterMember :execlastid
INSERT INTO cluster_members(cluster_id,case_id,helper_declaration_id,ordinal,call_offset) VALUES (?,?,?,?,?);

-- name: InsertSourceReceipt :execlastid
INSERT INTO source_receipts(case_id,ordinal,kind,file_id,start_line,end_line,start_offset,end_offset,subject,value_type,value,nesting,source_declaration_id) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?);

-- name: InsertDependencyReceipt :exec
INSERT INTO dependency_receipts(declaration_id,dependency_id,receipt_id,case_id) VALUES (?,?,?,?);

-- name: InsertExpansionDeclaration :exec
INSERT INTO receipt_expansion_declarations(receipt_id,ordinal,declaration_id,symbol) VALUES (?,?,?,?);

-- name: InsertExpansionSite :exec
INSERT INTO receipt_expansion_sites(receipt_id,ordinal,file_id,call_offset,owner_declaration_id) VALUES (?,?,?,?,?);

-- name: InsertClue :execlastid
INSERT INTO clues(case_id,ordinal,kind,subject,value_type,numeric_value,limit_type,limit_value,operator,declaration_id,cluster_id,member_id,pair_left_member_id,pair_right_member_id) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?);

-- name: InsertClueValue :exec
INSERT INTO clue_values(clue_id,ordinal,value) VALUES (?,?,?);

-- name: LinkClueReceipt :exec
INSERT INTO clue_receipts(case_id,clue_id,receipt_id) VALUES (?,?,?);

-- name: InsertHistory :exec
INSERT INTO case_history(case_id,ordinal,commit_hash,kind) VALUES (?,?,?,?);

-- name: InsertHistoryFile :exec
INSERT INTO case_history_files(case_id,history_ordinal,ordinal,file_id) VALUES (?,?,?,?);

-- name: InsertCommit :exec
INSERT INTO commits(hash,committed_at) VALUES (?,?);

-- name: LinkPolicy :exec
INSERT INTO case_policy_reviews(case_id,ordinal,policy_id) VALUES (?,?,?);

-- name: InsertPolicy :exec
INSERT INTO policy_reviews(id,note,review_prompt) VALUES (?,?,?);

-- name: InsertSuppression :exec
INSERT INTO suppressions(ordinal,smell,symbol,file_id,line,justification,applied,case_id) VALUES (?,?,?,?,?,?,?,?);

-- name: InsertWarning :exec
INSERT INTO warnings(ordinal,code,file_id,line,message) VALUES (?,?,?,?,?);

-- name: DependencyScored :one
SELECT scored FROM declaration_dependencies WHERE declaration_id=? AND dependency_id=?;

-- name: SummaryCases :many
SELECT c.id,c.smell,d.symbol,f.path,c.start_line,c.end_line,c.verdict,c.suppressed
FROM cases c JOIN declarations d ON d.id=c.primary_declaration_id
JOIN files f ON f.id=d.file_id ORDER BY c.ordinal;

-- name: SummarySuppressions :many
SELECT f.path,s.line,s.justification FROM suppressions s
JOIN files f ON f.id=s.file_id WHERE s.case_id=? ORDER BY s.ordinal;

-- name: SummaryMetrics :many
SELECT kind,subject,numeric_value,limit_value,operator FROM clues
WHERE case_id=? AND numeric_value IS NOT NULL ORDER BY ordinal;

-- name: SummaryPolicy :many
SELECT p.id,p.note,p.review_prompt FROM case_policy_reviews cp
JOIN policy_reviews p ON p.id=cp.policy_id WHERE cp.case_id=? ORDER BY cp.ordinal;

-- name: SummaryWarnings :many
SELECT code,COALESCE(f.path,''),w.line,w.message FROM warnings w
LEFT JOIN files f ON f.id=w.file_id ORDER BY w.ordinal;

-- name: SummaryTotals :one
SELECT CAST(failed AS INTEGER) AS failed,CAST(warned AS INTEGER) AS warned,CAST(suppressed AS INTEGER) AS suppressed FROM summary;

-- name: WarningMessages :many
SELECT message FROM warnings ORDER BY ordinal;

-- name: ReportIdentity :one
SELECT COUNT(*) AS report_count,CAST(COALESCE(MAX(schema_version),0) AS INTEGER) AS schema_version FROM report;

-- name: SummaryDuplicateFragments :many
SELECT f.path, r.start_line, r.end_line, r.start_offset, r.end_offset
FROM source_receipts r JOIN files f ON f.id = r.file_id
WHERE r.case_id = sqlc.arg(case_id) AND r.kind = 'duplicate-fragment'
ORDER BY f.path, r.start_offset, r.end_offset;
