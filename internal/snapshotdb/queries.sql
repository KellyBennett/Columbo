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
INSERT INTO source_receipts(case_id,ordinal,kind,file_id,start_line,end_line,start_offset,end_offset,subject,value_type,value,nesting,source_declaration_id,spelling) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?);

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

-- name: SummaryClueSets :many
SELECT c.kind,c.subject,v.value,v.ordinal,c.ordinal AS clue_ordinal
FROM clues c JOIN clue_values v ON v.clue_id=c.id
WHERE c.case_id=sqlc.arg(case_id) AND c.kind IN ('variant-set','repeated-variant-set','variant-shared-write','variant-write-read','selected-role','selected-implementation-set','selected-message-set')
ORDER BY c.ordinal,v.ordinal;

-- name: SummaryVariantDecisions :many
SELECT f.path,r.start_line,r.end_line,r.start_offset,r.end_offset,d.symbol
FROM source_receipts r JOIN files f ON f.id=r.file_id
JOIN declarations d ON d.id=r.source_declaration_id
WHERE r.case_id=sqlc.arg(case_id) AND r.kind='variant-decision'
ORDER BY f.path,r.start_offset;

-- name: SummarySelectionReceipts :many
SELECT r.kind,r.subject,f.path,r.start_line,r.end_line,r.start_offset,r.end_offset
FROM source_receipts r JOIN files f ON f.id=r.file_id
WHERE r.case_id=sqlc.arg(case_id) AND r.kind IN ('selection-decision','selection-origin','selection-flow','selected-message')
ORDER BY f.path,r.start_offset,r.kind;

-- name: InsertRoleCandidate :exec
INSERT INTO role_candidates (id,canonical_interface,confidence,classification) VALUES (?,?,?,?);
-- name: InsertRoleImplementation :exec
INSERT INTO role_candidate_implementations (candidate_id,identity) VALUES (?,?);
-- name: InsertRoleMessage :exec
INSERT INTO role_candidate_messages (candidate_id,identity) VALUES (?,?);
-- name: InsertRoleInterface :exec
INSERT INTO role_candidate_interfaces (candidate_id,identity,relationship) VALUES (?,?,?);
-- name: InsertRoleReceipt :exec
INSERT INTO role_candidate_receipts (candidate_id,ordinal,declaration_id,kind,subject,message,start_line,end_line,start_offset,end_offset) VALUES (?,?,?,?,?,?,?,?,?,?);
-- name: LinkRoleCase :exec
INSERT INTO role_candidate_case_links (candidate_id,case_id) VALUES (?,?);
-- name: SummaryRoles :many
SELECT id,canonical_interface,confidence,classification FROM role_candidates ORDER BY id;
-- name: SummaryRoleInterfaces :many
SELECT identity,relationship FROM role_candidate_interfaces WHERE candidate_id=sqlc.arg(candidate_id) ORDER BY relationship,identity;

-- name: InsertCorrelation :exec
INSERT INTO correlations (id,kind,confidence,variant_domain,diagnosis,policy_id,policy_status,policy_note,review_prompt) VALUES (?,?,?,?,?,?,?,?,?);
-- name: LinkCorrelationCase :exec
INSERT INTO correlation_cases (correlation_id,case_id) VALUES (?,?);
-- name: LinkCorrelationRole :exec
INSERT INTO correlation_role_candidates (correlation_id,candidate_id) VALUES (?,?);
-- name: LinkCorrelationEvidence :exec
INSERT INTO correlation_evidence (correlation_id,case_id,receipt_id) VALUES (?,?,?);
-- name: InsertCorrelationGuidance :exec
INSERT INTO correlation_guidance (correlation_id,kind,ordinal,text) VALUES (?,?,?,?);
-- name: SummaryCorrelations :many
SELECT * FROM correlations ORDER BY id;
-- name: SummaryCorrelationCases :many
SELECT c.id,c.suppressed,c.verdict FROM correlation_cases cc JOIN cases c ON c.id=cc.case_id WHERE cc.correlation_id=sqlc.arg(correlation_id) ORDER BY c.id;
-- name: SummaryCorrelationRoles :many
SELECT r.id,r.canonical_interface FROM correlation_role_candidates cr JOIN role_candidates r ON r.id=cr.candidate_id WHERE cr.correlation_id=sqlc.arg(correlation_id) ORDER BY r.id;
-- name: SummaryCorrelationPlayers :many
SELECT DISTINCT p.identity FROM correlation_role_candidates cr JOIN role_candidate_implementations p ON p.candidate_id=cr.candidate_id WHERE cr.correlation_id=sqlc.arg(correlation_id) ORDER BY p.identity;
-- name: SummaryCorrelationMappings :many
SELECT DISTINCT r.subject AS variant,r.spelling AS implementation FROM correlation_evidence ce JOIN source_receipts r ON r.id=ce.receipt_id AND r.case_id=ce.case_id WHERE ce.correlation_id=sqlc.arg(correlation_id) AND r.kind='selection-variant-mapping' ORDER BY r.subject,r.spelling;
-- name: SummaryCorrelationGuidance :many
SELECT kind,text FROM correlation_guidance WHERE correlation_id=sqlc.arg(correlation_id) ORDER BY kind,ordinal;

-- name: InsertAdvisoryCollector :exec
INSERT INTO advisory_collectors(kind,files_analyzed,declarations_analyzed) VALUES (?,?,?);
-- name: InsertAdvisoryGroup :exec
INSERT INTO advisory_groups(id,kind,subject,lead,limits) VALUES (?,?,?,?,?);
-- name: InsertAdvisoryValue :exec
INSERT INTO advisory_values(group_id,kind,ordinal,identity,value) VALUES (?,?,?,?,?);
-- name: InsertAdvisorySite :exec
INSERT INTO advisory_sites(group_id,ordinal,declaration_id,representation,input_expression) VALUES (?,?,?,?,?);
-- name: InsertAdvisorySiteValue :exec
INSERT INTO advisory_site_values(group_id,site_ordinal,ordinal,identity,value) VALUES (?,?,?,?,?);
-- name: InsertAdvisoryReceipt :exec
INSERT INTO advisory_receipts(group_id,site_ordinal,ordinal,kind,subject,start_line,end_line,start_offset,end_offset,spelling) VALUES (?,?,?,?,?,?,?,?,?,?);
-- name: SummaryAdvisories :many
SELECT id,kind,subject,lead,limits FROM advisory_groups ORDER BY kind,id;
-- name: AdvisoryValues :many
SELECT kind,identity,value FROM advisory_values WHERE group_id=? ORDER BY kind,ordinal;
-- name: AdvisorySites :many
SELECT s.ordinal,s.representation,s.input_expression,d.symbol,f.path FROM advisory_sites s JOIN declarations d ON d.id=s.declaration_id JOIN files f ON f.id=d.file_id WHERE s.group_id=? ORDER BY s.ordinal;
-- name: AdvisoryReceipts :many
SELECT kind,subject,start_line,end_line,start_offset,end_offset,spelling FROM advisory_receipts WHERE group_id=? AND site_ordinal=? ORDER BY ordinal;
-- name: AdvisoryCoverage :many
SELECT kind,files_analyzed,declarations_analyzed FROM advisory_collectors ORDER BY kind;
