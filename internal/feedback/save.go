package feedback

import (
	"context"
	"encoding/json"
	"strconv"
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
	for _, write := range record.upserts() {
		if err := write(s); err != nil {
			return err
		}
	}
	return record.pruneCurrent(s)
}

func jsonKeys(keys []string) string {
	data, _ := json.Marshal(append([]string{}, keys...))
	return string(data)
}

func (r PullRequestRecord) pruneCurrent(s saving) error {
	var comments, reactions, commits, files, suppressions []string
	for _, c := range r.Comments {
		comments = append(comments, string(c.Kind)+":"+strconv.FormatInt(c.ID, 10))
	}
	for _, r := range r.Reactions {
		reactions = append(reactions, strconv.FormatInt(r.ID, 10))
	}
	for _, c := range r.Commits {
		commits = append(commits, c.SHA)
		for _, f := range c.Files {
			files = append(files, c.SHA+":"+f.Path)
		}
	}
	for _, u := range r.Suppressions {
		suppressions = append(suppressions, u.Path+":"+strconv.FormatInt(u.Line, 10))
	}
	steps := []func() error{
		func() error {
			return s.q.PruneReactions(s.ctx, feedbackdb.PruneReactionsParams{PullRequestID: s.pullRequestID, Keys: jsonKeys(reactions)})
		},
		func() error {
			return s.q.PruneComments(s.ctx, feedbackdb.PruneCommentsParams{PullRequestID: s.pullRequestID, Keys: jsonKeys(comments)})
		},
		func() error {
			return s.q.PruneCommitFiles(s.ctx, feedbackdb.PruneCommitFilesParams{PullRequestID: s.pullRequestID, Keys: jsonKeys(files)})
		},
		func() error {
			return s.q.PruneCommits(s.ctx, feedbackdb.PruneCommitsParams{PullRequestID: s.pullRequestID, Keys: jsonKeys(commits)})
		},
		func() error {
			return s.q.PruneSuppressions(s.ctx, feedbackdb.PruneSuppressionsParams{PullRequestID: s.pullRequestID, Keys: jsonKeys(suppressions)})
		},
	}
	for _, step := range steps {
		if err := step(); err != nil {
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
