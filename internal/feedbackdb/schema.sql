CREATE TABLE repositories (
 name TEXT PRIMARY KEY CHECK (name GLOB '?*/?*'),
 scanned_through TEXT CHECK (scanned_through GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9]Z'),
 first_seen TEXT NOT NULL CHECK (first_seen GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9]Z'),
 last_seen TEXT NOT NULL CHECK (last_seen GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9]Z')
) STRICT;
CREATE TABLE pull_requests (
 id INTEGER PRIMARY KEY CHECK (id > 0),
 repository TEXT NOT NULL REFERENCES repositories(name) ON DELETE CASCADE,
 number INTEGER NOT NULL CHECK (number > 0),
 title TEXT NOT NULL,
 author TEXT NOT NULL CHECK (length(author) > 0),
 state TEXT NOT NULL CHECK (state IN ('open', 'closed')),
 merged INTEGER NOT NULL CHECK (merged IN (0, 1) AND (merged = 0 OR state = 'closed')),
 created_at TEXT NOT NULL CHECK (created_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9]Z'),
 closed_at TEXT CHECK (closed_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9]Z'),
 first_seen TEXT NOT NULL CHECK (first_seen GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9]Z'),
 last_seen TEXT NOT NULL CHECK (last_seen GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9]Z'),
 UNIQUE (repository, number)
) STRICT;
CREATE TABLE comments (
 kind TEXT NOT NULL CHECK (kind IN ('review', 'issue')),
 id INTEGER NOT NULL CHECK (id > 0),
 pull_request_id INTEGER NOT NULL REFERENCES pull_requests(id) ON DELETE CASCADE,
 author TEXT NOT NULL CHECK (length(author) > 0),
 body TEXT NOT NULL,
 path TEXT,
 line INTEGER CHECK (line > 0),
 commit_id TEXT,
 in_reply_to INTEGER CHECK (in_reply_to > 0),
 created_at TEXT NOT NULL CHECK (created_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9]Z'),
 updated_at TEXT NOT NULL CHECK (updated_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9]Z'),
 case_id TEXT CHECK (length(case_id) > 0),
 role TEXT CHECK (length(role) > 0),
 state TEXT CHECK (state IN ('open', 'cleared')),
 rule TEXT CHECK (length(rule) > 0),
 first_seen TEXT NOT NULL CHECK (first_seen GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9]Z'),
 last_seen TEXT NOT NULL CHECK (last_seen GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9]Z'),
 PRIMARY KEY (kind, id),
 CHECK ((case_id IS NULL) = (role IS NULL) AND (role IS NULL) = (state IS NULL)),
 CHECK (rule IS NULL OR case_id IS NOT NULL),
 CHECK (kind = 'review' OR (path IS NULL AND line IS NULL AND commit_id IS NULL AND in_reply_to IS NULL))
) STRICT;
CREATE INDEX comments_pull_request ON comments(pull_request_id);
CREATE INDEX comments_rule ON comments(role, rule);
CREATE TABLE reactions (
 id INTEGER PRIMARY KEY CHECK (id > 0),
 comment_kind TEXT NOT NULL,
 comment_id INTEGER NOT NULL,
 author TEXT NOT NULL CHECK (length(author) > 0),
 content TEXT NOT NULL CHECK (length(content) > 0),
 created_at TEXT NOT NULL CHECK (created_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9]Z'),
 first_seen TEXT NOT NULL CHECK (first_seen GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9]Z'),
 last_seen TEXT NOT NULL CHECK (last_seen GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9]Z'),
 FOREIGN KEY (comment_kind, comment_id) REFERENCES comments(kind, id) ON DELETE CASCADE
) STRICT;
CREATE INDEX reactions_comment ON reactions(comment_kind, comment_id);
CREATE TABLE commits (
 pull_request_id INTEGER NOT NULL REFERENCES pull_requests(id) ON DELETE CASCADE,
 sha TEXT NOT NULL CHECK (length(sha) > 0),
 author TEXT CHECK (length(author) > 0),
 message TEXT NOT NULL,
 committed_at TEXT NOT NULL CHECK (committed_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9]Z'),
 first_seen TEXT NOT NULL CHECK (first_seen GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9]Z'),
 last_seen TEXT NOT NULL CHECK (last_seen GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9]Z'),
 PRIMARY KEY (pull_request_id, sha)
) STRICT;
CREATE TABLE commit_files (
 pull_request_id INTEGER NOT NULL,
 sha TEXT NOT NULL,
 path TEXT NOT NULL CHECK (length(path) > 0),
 patch TEXT,
 first_seen TEXT NOT NULL CHECK (first_seen GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9]Z'),
 last_seen TEXT NOT NULL CHECK (last_seen GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9]Z'),
 PRIMARY KEY (pull_request_id, sha, path),
 FOREIGN KEY (pull_request_id, sha) REFERENCES commits(pull_request_id, sha) ON DELETE CASCADE
) STRICT;
CREATE TABLE suppressions (
 pull_request_id INTEGER NOT NULL REFERENCES pull_requests(id) ON DELETE CASCADE,
 path TEXT NOT NULL CHECK (length(path) > 0),
 line INTEGER NOT NULL CHECK (line > 0),
 rule TEXT NOT NULL CHECK (length(rule) > 0),
 justification TEXT NOT NULL CHECK (length(justification) > 0),
 first_seen TEXT NOT NULL CHECK (first_seen GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9]Z'),
 last_seen TEXT NOT NULL CHECK (last_seen GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9]Z'),
 PRIMARY KEY (pull_request_id, path, line)
) STRICT;
