package feedback

import (
	"context"
	"time"

	"github.com/KellyBennett/Columbo/internal/feedbackdb"
)

type saving struct {
	ctx           context.Context
	q             feedbackdb.Querier
	pullRequestID int64
	run           string
}

func saveRecord(ctx context.Context, q feedbackdb.Querier, record PullRequestRecord, run string) error {
	s := saving{ctx: ctx, q: q, pullRequestID: record.PullRequest.ID, run: run}
	for _, write := range append(record.upserts(), saving.deleteUnseenConversation, saving.deleteUnseenCode) {
		if err := write(s); err != nil {
			return err
		}
	}
	return nil
}

func (r PullRequestRecord) upserts() []func(saving) error {
	upserts := []func(saving) error{r.PullRequest.save}
	for _, comment := range r.Comments {
		upserts = append(upserts, comment.save)
	}
	for _, reaction := range r.Reactions {
		upserts = append(upserts, reaction.save)
	}
	for _, commit := range r.Commits {
		upserts = append(upserts, commit.save)
	}
	for _, suppression := range r.Suppressions {
		upserts = append(upserts, suppression.save)
	}
	return upserts
}

func (p PullRequest) save(s saving) error {
	return s.q.UpsertPullRequest(s.ctx, feedbackdb.UpsertPullRequestParams{
		ID: p.ID, Repository: p.Repository, Number: p.Number, Title: p.Title, Author: p.Author,
		State: p.State, Merged: boolInt(p.Merged), CreatedAt: stamp(p.CreatedAt), ClosedAt: optionalStamp(p.ClosedAt), Seen: s.run,
	})
}

func (c Comment) save(s saving) error {
	params := feedbackdb.UpsertCommentParams{
		Kind: string(c.Kind), ID: c.ID, PullRequestID: s.pullRequestID, Author: c.Author, Body: c.Body,
		Path: c.Path, Line: c.Line, CommitID: c.CommitID, InReplyTo: c.InReplyTo,
		CreatedAt: stamp(c.CreatedAt), UpdatedAt: stamp(c.UpdatedAt), Seen: s.run,
	}
	if c.Marker != nil {
		params.CaseID, params.Role, params.State, params.Rule = c.Marker.columns()
	}
	return s.q.UpsertComment(s.ctx, params)
}

func (m Marker) columns() (caseID, role, state, rule *string) {
	if m.Rule != "" {
		rule = &m.Rule
	}
	return &m.CaseID, &m.Role, &m.State, rule
}

func (r Reaction) save(s saving) error {
	return s.q.UpsertReaction(s.ctx, feedbackdb.UpsertReactionParams{
		ID: r.ID, CommentKind: string(r.CommentKind), CommentID: r.CommentID,
		Author: r.Author, Content: r.Content, CreatedAt: stamp(r.CreatedAt), Seen: s.run,
	})
}

func (c Commit) save(s saving) error {
	if err := s.q.UpsertCommit(s.ctx, feedbackdb.UpsertCommitParams{
		PullRequestID: s.pullRequestID, Sha: c.SHA, Author: c.Author, Message: c.Message, CommittedAt: stamp(c.CommittedAt), Seen: s.run,
	}); err != nil {
		return err
	}
	for _, file := range c.Files {
		if err := s.q.UpsertCommitFile(s.ctx, feedbackdb.UpsertCommitFileParams{
			PullRequestID: s.pullRequestID, Sha: c.SHA, Path: file.Path, Patch: file.Patch, Seen: s.run,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (u Suppression) save(s saving) error {
	return s.q.UpsertSuppression(s.ctx, feedbackdb.UpsertSuppressionParams{
		PullRequestID: s.pullRequestID, Path: u.Path, Line: u.Line, Rule: u.Rule, Justification: u.Justification, Seen: s.run,
	})
}

func (s saving) deleteUnseenConversation() error {
	if err := s.q.DeleteReactionsNotSeenSince(s.ctx, feedbackdb.DeleteReactionsNotSeenSinceParams{PullRequestID: s.pullRequestID, RunTime: s.run}); err != nil {
		return err
	}
	return s.q.DeleteCommentsNotSeenSince(s.ctx, feedbackdb.DeleteCommentsNotSeenSinceParams{PullRequestID: s.pullRequestID, RunTime: s.run})
}

func (s saving) deleteUnseenCode() error {
	if err := s.q.DeleteCommitFilesNotSeenSince(s.ctx, feedbackdb.DeleteCommitFilesNotSeenSinceParams{PullRequestID: s.pullRequestID, RunTime: s.run}); err != nil {
		return err
	}
	if err := s.q.DeleteCommitsNotSeenSince(s.ctx, feedbackdb.DeleteCommitsNotSeenSinceParams{PullRequestID: s.pullRequestID, RunTime: s.run}); err != nil {
		return err
	}
	return s.q.DeleteSuppressionsNotSeenSince(s.ctx, feedbackdb.DeleteSuppressionsNotSeenSinceParams{PullRequestID: s.pullRequestID, RunTime: s.run})
}

func boolInt(value bool) int64 {
	if value {
		return 1
	}
	return 0
}

func optionalStamp(t *time.Time) *string {
	if t == nil {
		return nil
	}
	value := stamp(*t)
	return &value
}
