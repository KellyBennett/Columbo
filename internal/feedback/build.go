package feedback

import (
	"cmp"
	"encoding/json"
	"slices"
)

type commentKey struct {
	kind CommentKind
	id   int64
}

type fetchedPullRequest struct {
	observationID  string
	resources      map[string]json.RawMessage
	repository     string
	pull           ghPull
	reviewComments []ghComment
	issueComments  []ghComment
	reactions      map[commentKey][]ghReaction
	files          []ghFile
	commits        []ghPullCommit
	commitFiles    map[string][]ghFile
}

func (f fetchedPullRequest) record() PullRequestRecord {
	return PullRequestRecord{
		PullRequest: f.pull.record(f.repository), Comments: f.comments(), Reactions: f.reactionRecords(),
		Commits: f.commitRecords(), Suppressions: f.suppressions(),
	}
}

func (f fetchedPullRequest) comments() []Comment {
	var comments []Comment
	for _, comment := range f.reviewComments {
		comments = append(comments, comment.review())
	}
	for _, comment := range f.issueComments {
		comments = append(comments, comment.issue())
	}
	return comments
}

func (f fetchedPullRequest) reactionRecords() []Reaction {
	var records []Reaction
	for key, reactions := range f.reactions {
		for _, reaction := range reactions {
			records = append(records, reaction.record(key))
		}
	}
	slices.SortFunc(records, func(a, b Reaction) int { return cmp.Compare(a.ID, b.ID) })
	return records
}

func (f fetchedPullRequest) commitRecords() []Commit {
	var commits []Commit
	for _, commit := range f.commits {
		commits = append(commits, commit.record(f.commitFiles[commit.SHA]))
	}
	return commits
}

func (f fetchedPullRequest) suppressions() []Suppression {
	var suppressions []Suppression
	for _, file := range f.files {
		suppressions = append(suppressions, file.suppressions()...)
	}
	return suppressions
}

func (p ghPull) record(repository string) PullRequest {
	return PullRequest{
		ID: p.ID, Repository: repository, Number: p.Number, Title: p.Title, Author: p.User.login(),
		State: p.State, Merged: p.Merged, CreatedAt: p.CreatedAt, ClosedAt: p.ClosedAt,
	}
}

func (c ghComment) review() Comment {
	comment := c.issue()
	comment.Kind, comment.Path, comment.CommitID, comment.InReplyTo = ReviewComment, c.Path, c.CommitID, c.InReplyToID
	comment.OriginalCommitID, comment.OriginalLine = c.OriginalCommitID, c.OriginalLine
	comment.Line = c.anchorLine()
	return comment
}

func (c ghComment) anchorLine() *int64 {
	if c.Line != nil {
		return c.Line
	}
	return c.OriginalLine
}

func (c ghComment) issue() Comment {
	return Comment{
		Kind: IssueComment, ID: c.ID, Author: c.User.login(), Body: c.Body,
		CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt, Marker: parseMarker(c.Body),
	}
}

func (r ghReaction) record(comment commentKey) Reaction {
	return Reaction{
		ID: r.ID, CommentKind: comment.kind, CommentID: comment.id,
		Author: r.User.login(), Content: r.Content, CreatedAt: r.CreatedAt,
	}
}

func (c ghPullCommit) record(files []ghFile) Commit {
	commit := Commit{SHA: c.SHA, Message: c.Commit.Message, CommittedAt: c.Commit.Committer.Date}
	if c.Author != nil && c.Author.Login != "" {
		commit.Author = &c.Author.Login
	}
	for _, file := range files {
		commit.Files = append(commit.Files, CommitFile{Path: file.Filename, Patch: file.Patch})
	}
	return commit
}

func (f ghFile) suppressions() []Suppression {
	if f.Patch == nil {
		return nil
	}
	return parseSuppressions(f.Filename, *f.Patch)
}
