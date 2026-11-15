-- name: UpsertRepository :exec
INSERT INTO repositories(name, first_seen, last_seen) VALUES (@name, @seen, @seen)
ON CONFLICT(name) DO UPDATE SET last_seen = excluded.last_seen;

-- name: RepositoryScannedThrough :one
SELECT scanned_through FROM repositories WHERE name = ?;

-- name: SetRepositoryScannedThrough :exec
UPDATE repositories SET scanned_through = @scanned_through WHERE name = @name;

-- name: PullRequestNumbers :many
SELECT number FROM pull_requests WHERE repository = ? ORDER BY number;

-- name: UpsertPullRequest :exec
INSERT INTO pull_requests(id, repository, number, title, author, state, merged, created_at, closed_at, first_seen, last_seen)
VALUES (@id, @repository, @number, @title, @author, @state, @merged, @created_at, @closed_at, @seen, @seen)
ON CONFLICT(id) DO UPDATE SET
 repository = excluded.repository, number = excluded.number, title = excluded.title, author = excluded.author,
 state = excluded.state, merged = excluded.merged, created_at = excluded.created_at, closed_at = excluded.closed_at,
 last_seen = excluded.last_seen;

-- name: UpsertComment :exec
INSERT INTO comments(kind, id, pull_request_id, author, body, path, line, commit_id, in_reply_to, created_at, updated_at,
 case_id, role, state, rule, first_seen, last_seen)
VALUES (@kind, @id, @pull_request_id, @author, @body, @path, @line, @commit_id, @in_reply_to, @created_at, @updated_at,
 @case_id, @role, @state, @rule, @seen, @seen)
ON CONFLICT(kind, id) DO UPDATE SET
 pull_request_id = excluded.pull_request_id, author = excluded.author, body = excluded.body, path = excluded.path,
 line = excluded.line, commit_id = excluded.commit_id, in_reply_to = excluded.in_reply_to, created_at = excluded.created_at,
 updated_at = excluded.updated_at, case_id = excluded.case_id, role = excluded.role, state = excluded.state,
 rule = excluded.rule, last_seen = excluded.last_seen;

-- name: UpsertReaction :exec
INSERT INTO reactions(id, comment_kind, comment_id, author, content, created_at, first_seen, last_seen)
VALUES (@id, @comment_kind, @comment_id, @author, @content, @created_at, @seen, @seen)
ON CONFLICT(id) DO UPDATE SET
 comment_kind = excluded.comment_kind, comment_id = excluded.comment_id, author = excluded.author,
 content = excluded.content, created_at = excluded.created_at, last_seen = excluded.last_seen;

-- name: UpsertCommit :exec
INSERT INTO commits(pull_request_id, sha, author, message, committed_at, first_seen, last_seen)
VALUES (@pull_request_id, @sha, @author, @message, @committed_at, @seen, @seen)
ON CONFLICT(pull_request_id, sha) DO UPDATE SET
 author = excluded.author, message = excluded.message, committed_at = excluded.committed_at, last_seen = excluded.last_seen;

-- name: UpsertCommitFile :exec
INSERT INTO commit_files(pull_request_id, sha, path, patch, first_seen, last_seen)
VALUES (@pull_request_id, @sha, @path, @patch, @seen, @seen)
ON CONFLICT(pull_request_id, sha, path) DO UPDATE SET patch = excluded.patch, last_seen = excluded.last_seen;

-- name: UpsertSuppression :exec
INSERT INTO suppressions(pull_request_id, path, line, rule, justification, first_seen, last_seen)
VALUES (@pull_request_id, @path, @line, @rule, @justification, @seen, @seen)
ON CONFLICT(pull_request_id, path, line) DO UPDATE SET
 rule = excluded.rule, justification = excluded.justification, last_seen = excluded.last_seen;

-- A run writes last_seen = its own start time on every row it saw, so any
-- other value means the row was not seen this run, even within one second.

-- name: DeleteCommentsNotSeenSince :exec
DELETE FROM comments WHERE pull_request_id = @pull_request_id AND last_seen <> @run_time;

-- name: DeleteReactionsNotSeenSince :exec
DELETE FROM reactions WHERE reactions.last_seen <> @run_time AND EXISTS
 (SELECT 1 FROM comments c WHERE c.kind = reactions.comment_kind AND c.id = reactions.comment_id AND c.pull_request_id = @pull_request_id);

-- name: DeleteCommitsNotSeenSince :exec
DELETE FROM commits WHERE pull_request_id = @pull_request_id AND last_seen <> @run_time;

-- name: DeleteCommitFilesNotSeenSince :exec
DELETE FROM commit_files WHERE pull_request_id = @pull_request_id AND last_seen <> @run_time;

-- name: DeleteSuppressionsNotSeenSince :exec
DELETE FROM suppressions WHERE pull_request_id = @pull_request_id AND last_seen <> @run_time;

-- name: FindingReactions :many
SELECT c.rule, r.content, count(*) AS reactions
FROM reactions r JOIN comments c ON c.kind = r.comment_kind AND c.id = r.comment_id
WHERE c.role = 'finding' AND c.rule IS NOT NULL
GROUP BY c.rule, r.content ORDER BY c.rule, r.content;

-- name: FindingRules :many
SELECT DISTINCT rule FROM comments WHERE role = 'finding' AND rule IS NOT NULL ORDER BY rule;

-- LIKE is ASCII case-insensitive; the caller decides which candidates are misses.
-- name: MissCandidates :many
SELECT body FROM comments WHERE case_id IS NULL AND body LIKE '%columbo missed:%' ORDER BY kind, id;

-- One row per stored record, so a caller can see what a run added, kept or removed.
-- name: RowTimes :many
SELECT 'repository' AS tbl, name AS record, first_seen, last_seen FROM repositories
UNION ALL SELECT 'pull_request', CAST(id AS TEXT), first_seen, last_seen FROM pull_requests
UNION ALL SELECT 'comment', kind || ':' || id, first_seen, last_seen FROM comments
UNION ALL SELECT 'reaction', CAST(id AS TEXT), first_seen, last_seen FROM reactions
UNION ALL SELECT 'commit', pull_request_id || ':' || sha, first_seen, last_seen FROM commits
UNION ALL SELECT 'commit_file', pull_request_id || ':' || sha || ':' || path, first_seen, last_seen FROM commit_files
UNION ALL SELECT 'suppression', pull_request_id || ':' || path || ':' || line, first_seen, last_seen FROM suppressions
ORDER BY 1, 2;
