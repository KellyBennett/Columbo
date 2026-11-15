package feedback

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/KellyBennett/Columbo/internal/feedbackdb"
	"github.com/KellyBennett/Columbo/internal/snapshotdb"
	"github.com/stretchr/testify/require"
)

func openStore(t *testing.T, path string) *feedbackdb.Store {
	t.Helper()
	store, err := feedbackdb.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })
	return store
}

func TestOpenCreatesAPrivateStoreThatReopens(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "feedback.sqlite")
	store := openStore(t, path)
	ctx := context.Background()
	require.NoError(t, store.Update(ctx, func(q feedbackdb.Querier) error {
		return q.UpsertRepository(ctx, feedbackdb.UpsertRepositoryParams{Name: "o/r", Seen: "2026-01-01T00:00:00Z"})
	}))
	require.NoError(t, store.Close())

	info, err := os.Stat(filepath.Dir(path))
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o700), info.Mode().Perm())
	rows, err := openStore(t, path).Queries().RowTimes(ctx)
	require.NoError(t, err)
	require.Equal(t, []feedbackdb.RowTimesRow{{Tbl: "repository", Record: "o/r", FirstSeen: "2026-01-01T00:00:00Z", LastSeen: "2026-01-01T00:00:00Z"}}, rows)
}

func TestOpenAcceptsARelativePath(t *testing.T) {
	directory := t.TempDir()
	t.Chdir(directory)
	store := openStore(t, "feedback.sqlite")
	ctx := context.Background()
	require.NoError(t, store.Update(ctx, func(q feedbackdb.Querier) error {
		return q.UpsertRepository(ctx, feedbackdb.UpsertRepositoryParams{Name: "o/r", Seen: "2026-01-01T00:00:00Z"})
	}))
	require.FileExists(t, filepath.Join(directory, "feedback.sqlite"))
}

func TestOpenRefusesAnotherApplicationsFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "snapshot.sqlite")
	writer, err := snapshotdb.Create(path)
	require.NoError(t, err)
	require.NoError(t, writer.Queries().InsertReport(context.Background(), snapshotdb.InsertReportParams{SchemaVersion: snapshotdb.SchemaVersion, ColumboVersion: "test"}))
	require.NoError(t, writer.Complete())
	require.NoError(t, writer.Close())

	_, err = feedbackdb.Open(path)
	require.ErrorContains(t, err, "not a Columbo feedback store")
}

func TestUpdateRollsBackWhenTheCallbackFails(t *testing.T) {
	store := openStore(t, filepath.Join(t.TempDir(), "feedback.sqlite"))
	ctx := context.Background()
	failure := errors.New("stop")
	err := store.Update(ctx, func(q feedbackdb.Querier) error {
		require.NoError(t, q.UpsertRepository(ctx, feedbackdb.UpsertRepositoryParams{Name: "o/r", Seen: "2026-01-01T00:00:00Z"}))
		return failure
	})
	require.ErrorIs(t, err, failure)
	rows, err := store.Queries().RowTimes(ctx)
	require.NoError(t, err)
	require.Empty(t, rows)
}

func TestUpsertKeepsFirstSeenAndForeignKeysHold(t *testing.T) {
	store := openStore(t, filepath.Join(t.TempDir(), "feedback.sqlite"))
	ctx := context.Background()
	save := func(seen string) {
		require.NoError(t, store.Update(ctx, func(q feedbackdb.Querier) error {
			require.NoError(t, q.UpsertRepository(ctx, feedbackdb.UpsertRepositoryParams{Name: "o/r", Seen: seen}))
			require.NoError(t, q.UpsertPullRequest(ctx, feedbackdb.UpsertPullRequestParams{ID: 7, Repository: "o/r", Number: 1, Title: "t", Author: "a", State: "open", CreatedAt: seen, Seen: seen}))
			return q.UpsertComment(ctx, feedbackdb.UpsertCommentParams{Kind: "issue", ID: 9, PullRequestID: 7, Author: "a", Body: "b", CreatedAt: seen, UpdatedAt: seen, Seen: seen})
		}))
	}
	save("2026-01-01T00:00:00Z")
	save("2026-01-02T00:00:00Z")
	rows, err := store.Queries().RowTimes(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 3)
	for _, row := range rows {
		require.Equal(t, "2026-01-01T00:00:00Z", row.FirstSeen, row.Tbl)
		require.Equal(t, "2026-01-02T00:00:00Z", row.LastSeen, row.Tbl)
	}

	require.NoError(t, store.Update(ctx, func(q feedbackdb.Querier) error {
		return q.DeleteCommentsNotSeenSince(ctx, feedbackdb.DeleteCommentsNotSeenSinceParams{PullRequestID: 7, RunTime: "2026-01-03T00:00:00Z"})
	}))
	rows, err = store.Queries().RowTimes(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 2)

	err = store.Update(ctx, func(q feedbackdb.Querier) error {
		return q.UpsertComment(ctx, feedbackdb.UpsertCommentParams{Kind: "issue", ID: 10, PullRequestID: 99, Author: "a", Body: "b", CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-01T00:00:00Z", Seen: "2026-01-01T00:00:00Z"})
	})
	require.ErrorContains(t, err, "FOREIGN KEY")
}
