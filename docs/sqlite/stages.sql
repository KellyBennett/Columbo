-- Empty on a legacy-mode run. Active pending-definition is not completion.
SELECT ordinal,name,state,pending_definition,issue_count,task
FROM refactoring_stages ORDER BY ordinal;
SELECT s.name,m.collector,m.issue_count
FROM stage_collectors m JOIN refactoring_stages s ON s.id=m.stage_id
ORDER BY s.ordinal,m.ordinal;
SELECT id,kind,subject,lead,limits FROM active_stage_issues ORDER BY kind,id;
