package feedback

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/KellyBennett/Columbo/internal/feedbackdb"
	"github.com/stretchr/testify/require"
)

const findingBody = `<!-- columbo-feedback case=C-1 role=finding state=cleared -->\n**Columbo: long-parameter-list** in ` + "`o/r.F`" + ` (new)`

type fakeGitHub struct {
	t         *testing.T
	mu        sync.Mutex
	responses map[string]string
	requests  []*http.Request
}

func newFakeGitHub(t *testing.T) (*fakeGitHub, *httptest.Server) {
	fake := &fakeGitHub{t: t, responses: map[string]string{
		"/repos/o/r/pulls/comments": `[
			{"id": 11, "body": "` + findingBody + `", "pull_request_url": "https://api.example/repos/o/r/pulls/1", "created_at": "2026-01-02T00:00:00Z", "updated_at": "2026-01-02T00:00:00Z"},
			{"id": 12, "body": "Agreed", "pull_request_url": "https://api.example/repos/o/r/pulls/1", "created_at": "2026-01-02T01:00:00Z", "updated_at": "2026-01-02T01:00:00Z"}]`,
		"/repos/o/r/issues/comments": `[
			{"id": 21, "body": " columbo MISSED: feature-envy in G", "issue_url": "https://api.example/repos/o/r/issues/1", "created_at": "2026-01-02T02:00:00Z", "updated_at": "2026-01-02T02:00:00Z"}]`,
		"/repos/o/r/pulls/1": `{"id": 1001, "number": 1, "title": "Add F", "user": {"login": "dev"}, "state": "closed", "merged": true,
			"created_at": "2026-01-01T00:00:00Z", "closed_at": "2026-01-03T00:00:00Z"}`,
		"/repos/o/r/pulls/1/comments?page=1": `[
			{"id": 11, "user": {"login": "github-actions[bot]"}, "body": "` + findingBody + `", "path": "f.go", "line": null, "original_line": 3,
			 "commit_id": "bbb", "created_at": "2026-01-02T00:00:00Z", "updated_at": "2026-01-02T00:00:00Z"}]`,
		"/repos/o/r/pulls/1/comments?page=2": `[
			{"id": 12, "user": {"login": "dev"}, "body": "Agreed", "path": "f.go", "line": 3, "commit_id": "bbb", "in_reply_to_id": 11,
			 "created_at": "2026-01-02T01:00:00Z", "updated_at": "2026-01-02T01:00:00Z"}]`,
		"/repos/o/r/issues/1/comments": `[
			{"id": 21, "user": null, "body": " columbo MISSED: feature-envy in G", "created_at": "2026-01-02T02:00:00Z", "updated_at": "2026-01-02T02:00:00Z"}]`,
		"/repos/o/r/pulls/comments/11/reactions": `[
			{"id": 501, "user": {"login": "alice"}, "content": "+1", "created_at": "2026-01-02T03:00:00Z"},
			{"id": 502, "user": {"login": "bob"}, "content": "-1", "created_at": "2026-01-02T04:00:00Z"}]`,
		"/repos/o/r/pulls/1/files": `[
			{"filename": "f.go", "patch": "@@ -1,2 +1,3 @@\n package f\n+// columbo:ignore long-function -- generated dispatch table\n func F() {}"},
			{"filename": "logo.png"}]`,
		"/repos/o/r/pulls/1/commits": `[
			{"sha": "aaa", "author": {"login": "dev"}, "commit": {"message": "Add F", "committer": {"date": "2026-01-01T12:00:00Z"}}},
			{"sha": "bbb", "author": null, "commit": {"message": "Shorten F", "committer": {"date": "2026-01-02T12:00:00Z"}}}]`,
		"/repos/o/r/commits/bbb":       `{"files": [{"filename": "f.go", "patch": "@@ -1 +1 @@\n-func F(a, b, c, d, e int) {}\n+func F(o Options) {}"}]}`,
		"/repos/o/r/commits/aaa":       `{"parents":[],"files": [{"filename":"f.go","patch":"@@ -0,0 +1 @@\n+package f"}]}`,
		"/repos/o/r/pulls/1/reviews":   `[{"id":91,"body":"Keep this ownership boundary","state":"COMMENTED","commit_id":"aaa"}]`,
		"/repos/o/r/issues/1/timeline": `[]`,
		"/repos/o/r/actions/runs":      `{"workflow_runs":[]}`,
		"/graphql":                     `{"data":{"repository":{"pullRequest":{"reviewThreads":{"nodes":[{"id":"thread-1","isResolved":true,"comments":{"nodes":[{"databaseId":11}]}}],"pageInfo":{"hasNextPage":false}}}}}}`,
	}}
	server := httptest.NewServer(http.HandlerFunc(fake.serve))
	t.Cleanup(server.Close)
	return fake, server
}

func (f *fakeGitHub) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, r)
	key := r.URL.Path
	if r.URL.Path == "/repos/o/r/pulls/1/comments" {
		page := r.FormValue("page")
		if page == "" {
			page = "1"
			w.Header().Set("Link", fmt.Sprintf(`<http://%s/repos/o/r/pulls/1/comments?per_page=100&page=2>; rel="next"`, r.Host))
		}
		key += "?page=" + page
	}
	body, ok := f.responses[key]
	if !ok {
		f.t.Errorf("unexpected request %s", r.URL)
		http.NotFound(w, r)
		return
	}
	fmt.Fprint(w, body)
}

func (f *fakeGitHub) set(path, body string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.responses[path] = body
}

func collectAt(t *testing.T, server *httptest.Server, store string, now string) string {
	t.Helper()
	at, err := time.Parse(time.RFC3339, now)
	require.NoError(t, err)
	var out bytes.Buffer
	err = Collect(context.Background(), CollectOptions{
		Repositories: []string{"o/r"}, StorePath: store, APIURL: server.URL, Token: "secret-token",
		Now: func() time.Time { return at },
	}, &out)
	require.NoError(t, err)
	return out.String()
}

func storedRows(t *testing.T, path string) []feedbackdb.RowTimesRow {
	t.Helper()
	rows, err := openStore(t, path).Queries().RowTimes(context.Background())
	require.NoError(t, err)
	return rows
}

func tableCounts(rows []feedbackdb.RowTimesRow) map[string]int {
	counts := map[string]int{}
	for _, row := range rows {
		counts[row.Tbl]++
	}
	return counts
}

func TestCollectStoresAPullRequestAndConvergesOnRerun(t *testing.T) {
	fake, server := newFakeGitHub(t)
	store := filepath.Join(t.TempDir(), "feedback.sqlite")

	out := collectAt(t, server, store, "2026-02-01T00:00:00Z")
	require.Equal(t, "o/r: 1 pull request, 3 comments, 2 reactions, 1 miss\n", out)
	first := storedRows(t, store)
	require.Equal(t, map[string]int{"repository": 1, "pull_request": 1, "comment": 3, "reaction": 2, "commit": 2, "commit_file": 2, "suppression": 1}, tableCounts(first))

	collectAt(t, server, store, "2026-02-02T00:00:00Z")
	second := storedRows(t, store)
	require.Len(t, second, len(first))
	for i := range first {
		require.Equal(t, first[i].Record, second[i].Record)
		require.Equal(t, "2026-02-01T00:00:00Z", second[i].FirstSeen, second[i].Record)
		require.Equal(t, "2026-02-02T00:00:00Z", second[i].LastSeen, second[i].Record)
	}

	fake.set("/repos/o/r/pulls/comments/11/reactions", `[{"id": 501, "user": {"login": "alice"}, "content": "+1", "created_at": "2026-01-02T03:00:00Z"}]`)
	collectAt(t, server, store, "2026-02-03T00:00:00Z")
	var reactions []string
	for _, row := range storedRows(t, store) {
		if row.Tbl == "reaction" {
			reactions = append(reactions, row.Record)
		}
	}
	require.Equal(t, []string{"501"}, reactions)
}

func TestCollectOnlyReadsAndAsksForCommentsSinceTheLastScan(t *testing.T) {
	fake, server := newFakeGitHub(t)
	store := filepath.Join(t.TempDir(), "feedback.sqlite")
	collectAt(t, server, store, "2026-02-01T00:00:00Z")
	collectAt(t, server, store, "2026-02-02T00:00:00Z")

	var since []string
	for _, request := range fake.requests {
		if request.URL.Path == "/graphql" {
			require.Equal(t, http.MethodPost, request.Method)
		} else {
			require.Equal(t, http.MethodGet, request.Method)
		}
		require.Equal(t, "Bearer secret-token", request.Header.Get("Authorization"))
		if request.URL.Path == "/repos/o/r/pulls/comments" {
			since = append(since, request.FormValue("since"))
		}
	}
	require.Equal(t, []string{"", "2026-02-01T00:00:00Z"}, since)
}

func TestCollectRetainsEarlierObservationsWhenReactionsAndCommentsChange(t *testing.T) {
	fake, server := newFakeGitHub(t)
	store := filepath.Join(t.TempDir(), "feedback.sqlite")
	collectAt(t, server, store, "2026-02-01T00:00:00Z")
	fake.set("/repos/o/r/pulls/comments/11/reactions", `[]`)
	fake.set("/repos/o/r/pulls/1/comments?page=2", `[]`)
	collectAt(t, server, store, "2026-02-01T00:00:00Z")
	rows, err := openStore(t, store).Queries().Observations(context.Background(), feedbackdb.ObservationsParams{Repository: "o/r"})
	require.NoError(t, err)
	require.Len(t, rows, 2)
	var before, after Observation
	require.NoError(t, json.Unmarshal([]byte(rows[0].Payload), &before))
	require.NoError(t, json.Unmarshal([]byte(rows[1].Payload), &after))
	require.Len(t, before.Record.Reactions, 2)
	require.Empty(t, after.Record.Reactions)
	require.Len(t, before.Record.Comments, 3)
	require.Len(t, after.Record.Comments, 2)
	current, err := openStore(t, store).Queries().FindingReactions(context.Background())
	require.NoError(t, err)
	require.Empty(t, current, "withdrawn reactions must disappear even when both collections have the same second timestamp")
	require.NotEqual(t, before.ID, after.ID)
	require.Contains(t, string(before.Resources["/repos/o/r/pulls/1/reviews?per_page=100"]), "ownership boundary")
	require.Contains(t, string(before.Resources["graphql/review-threads/1/"]), `"isResolved":true`)
	require.Contains(t, string(before.Resources["/repos/o/r/commits/aaa?per_page=100"]), "package f")
}

func TestTargetedCollectionDoesNotAdvanceDiscoveryCursor(t *testing.T) {
	fake, server := newFakeGitHub(t)
	store := filepath.Join(t.TempDir(), "feedback.sqlite")
	err := Collect(context.Background(), CollectOptions{Repositories: []string{"o/r"}, PullRequests: []int64{1}, StorePath: store, APIURL: server.URL, Token: "token", Now: time.Now}, &bytes.Buffer{})
	require.NoError(t, err)
	for _, request := range fake.requests {
		require.NotEqual(t, "/repos/o/r/pulls/comments", request.URL.Path)
		require.NotEqual(t, "/repos/o/r/issues/comments", request.URL.Path)
	}
	cursor, err := openStore(t, store).Queries().RepositoryScannedThrough(context.Background(), feedbackdb.RepositoryScannedThroughParams{Name: "o/r"})
	require.NoError(t, err)
	require.Nil(t, cursor)
}

func TestCollectReportsGitHubFailuresWithoutTheToken(t *testing.T) {
	tests := []struct {
		status  int
		headers map[string]string
		want    string
	}{
		{http.StatusUnauthorized, nil, "rejected the token (401)"},
		{http.StatusForbidden, map[string]string{"X-RateLimit-Remaining": "0", "X-RateLimit-Reset": "1767225600"}, "rate limit reached; try again after 2026-01-01T00:00:00Z"},
		{http.StatusNotFound, nil, "not found (404)"},
	}
	for _, test := range tests {
		t.Run(http.StatusText(test.status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				for key, value := range test.headers {
					w.Header().Set(key, value)
				}
				w.WriteHeader(test.status)
				fmt.Fprint(w, `{"message": "nope"}`)
			}))
			defer server.Close()
			err := Collect(context.Background(), CollectOptions{
				Repositories: []string{"o/r"}, StorePath: filepath.Join(t.TempDir(), "f.sqlite"), APIURL: server.URL,
				Token: "secret-token", Now: time.Now,
			}, &bytes.Buffer{})
			require.ErrorContains(t, err, test.want)
			require.False(t, strings.Contains(err.Error(), "secret-token"))
		})
	}
}
