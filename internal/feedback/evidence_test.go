package feedback

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KellyBennett/Columbo/internal/feedbackdb"
	"github.com/stretchr/testify/require"
)

func newHistory(t *testing.T) (dir, before, after string) {
	t.Helper()
	dir = filepath.Join(t.TempDir(), "source with spaces")
	_, err := git(context.Background(), "", "init", "--quiet", dir)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "f.go"), []byte("package p\nfunc Sum(a, b int) int { return a + b }\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.invalid/pilot\n\ngo 1.25.1\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "f_test.go"), []byte("package p\nimport \"testing\"\nfunc TestSum(t *testing.T) { if Sum(2,3) != 5 { t.Fatal(\"sum changed\") } }\n"), 0o600))
	before = commitFiles(t, dir)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "f.go"), []byte("package p\nfunc Sum(a, b int) int { return b + a }\n"), 0o600))
	after = commitFiles(t, dir)
	return
}

func commitFiles(t *testing.T, dir string) string {
	t.Helper()
	_, err := git(context.Background(), dir, "add", ".")
	require.NoError(t, err)
	_, err = git(context.Background(), dir, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "--quiet", "-m", "fixture")
	require.NoError(t, err)
	sha, err := git(context.Background(), dir, "rev-parse", "HEAD")
	require.NoError(t, err)
	return strings.TrimSpace(sha)
}

func TestRetainedHistorySurvivesSourceDeletionAndGarbageCollection(t *testing.T) {
	source, before, after := newHistory(t)
	client, err := newGitHubClient("https://api.github.com", "")
	require.NoError(t, err)
	c := collector{repository: "o/r", github: client, options: CollectOptions{Checkout: source, EvidencePath: filepath.Join(t.TempDir(), "evidence")}}
	o := Observation{ID: "observation-1", Revisions: []Revision{{SHA: before, Roles: []string{"original-review-anchor"}}, {SHA: after, Roles: []string{"pr-head"}}}}
	archive, err := c.retainHistory(context.Background(), &o)
	require.NoError(t, err)
	require.True(t, o.Revisions[0].Present)
	require.Equal(t, []string{before}, o.Revisions[1].Parents)
	require.NoFileExists(t, filepath.Join(archive, "objects", "info", "alternates"))
	require.NoError(t, os.RemoveAll(source))
	_, err = git(context.Background(), archive, "gc", "--prune=now")
	require.NoError(t, err)
	checkout := filepath.Join(t.TempDir(), "trial")
	require.NoError(t, checkoutRevision(context.Background(), archive, before, checkout))
	data, err := os.ReadFile(filepath.Join(checkout, "f.go"))
	require.NoError(t, err)
	require.Contains(t, string(data), "return a + b")
	_, err = git(context.Background(), checkout, "cat-file", "-e", after+"^{commit}")
	require.Error(t, err, "trial checkout must not receive later history")
}

func TestFeedbackV1StoreMigratesWithoutLosingRows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v1.sqlite")
	schema, err := os.ReadFile("../feedbackdb/schema.sql")
	require.NoError(t, err)
	schema = []byte(strings.Split(string(schema), "CREATE TABLE observations")[0])
	command := exec.Command("python3", "-c", `import sqlite3,sys
c=sqlite3.connect(sys.argv[1]); c.executescript(sys.stdin.read())
c.execute("PRAGMA application_id="+sys.argv[2]); c.execute("PRAGMA user_version=1")
c.execute("INSERT INTO repositories(name,first_seen,last_seen) VALUES ('o/r','2026-01-01T00:00:00Z','2026-01-01T00:00:00Z')")
c.commit()`, path, fmt.Sprint(feedbackdb.SQLiteApplicationID))
	command.Stdin = bytes.NewReader(schema)
	output, err := command.CombinedOutput()
	require.NoError(t, err, string(output))
	store := openStore(t, path)
	rows, err := store.Queries().RowTimes(context.Background())
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.NoError(t, store.Update(context.Background(), func(q feedbackdb.Querier) error {
		return q.InsertObservation(context.Background(), feedbackdb.InsertObservationParams{ID: "one", Repository: "o/r", Number: 1, ObservedAt: "2026-01-02T00:00:00Z", Payload: `{"version":1}`})
	}))
	require.NoError(t, store.Close())
	rows2, err := openStore(t, path).Queries().Observations(context.Background(), feedbackdb.ObservationsParams{Repository: "o/r"})
	require.NoError(t, err)
	require.Len(t, rows2, 1)
}

func TestReviewThreadsUseReadOnlyQueryAndPaginateOnEnterprise(t *testing.T) {
	var cursors []any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/graphql", r.URL.Path)
		require.Equal(t, http.MethodPost, r.Method)
		var request struct {
			Query     string
			Variables map[string]any
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		require.True(t, strings.HasPrefix(request.Query, "query "))
		require.NotContains(t, request.Query, "mutation")
		cursors = append(cursors, request.Variables["after"])
		next := len(cursors) == 1
		fmt.Fprintf(w, `{"data":{"repository":{"pullRequest":{"reviewThreads":{"nodes":[],"pageInfo":{"hasNextPage":%t,"endCursor":"next"}}}}}}`, next)
	}))
	defer server.Close()
	client, err := newGitHubClient(server.URL+"/api/v3", "token")
	require.NoError(t, err)
	client.captured = map[string]json.RawMessage{}
	require.NoError(t, client.threads(context.Background(), "o/r", 1))
	require.Equal(t, []any{nil, "next"}, cursors)
	require.Len(t, client.captured, 2)
}

func TestArtifactCapturePreservesArchiveAndStructuredProvenance(t *testing.T) {
	var data bytes.Buffer
	archive := zip.NewWriter(&data)
	entry, err := archive.Create("feedback-provenance.json")
	require.NoError(t, err)
	_, err = entry.Write([]byte(`{"analyzed_commit":"abc","head_commit":"def","clean":true}`))
	require.NoError(t, err)
	require.NoError(t, archive.Close())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(data.Bytes())
	}))
	defer server.Close()
	client, err := newGitHubClient(server.URL, "token")
	require.NoError(t, err)
	c := collector{repository: "o/r", github: client, options: CollectOptions{EvidencePath: t.TempDir()}}
	a := CIArtifact{ID: 12}
	require.NoError(t, c.downloadArtifact(context.Background(), &a))
	got, err := os.ReadFile(a.Path)
	require.NoError(t, err)
	require.Equal(t, data.Bytes(), got)
	require.Len(t, a.SHA256, 64)
	require.JSONEq(t, `{"analyzed_commit":"abc","head_commit":"def","clean":true}`, string(a.Provenance))
}
