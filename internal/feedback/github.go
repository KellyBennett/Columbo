package feedback

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type githubClient struct {
	base     *url.URL
	token    string
	http     *http.Client
	captured map[string]json.RawMessage
}

func newGitHubClient(apiURL, token string) (*githubClient, error) {
	base, err := url.Parse(strings.TrimSuffix(apiURL, "/"))
	if err != nil || base.Scheme == "" || base.Host == "" {
		return nil, fmt.Errorf("invalid GitHub API URL %q", apiURL)
	}
	return &githubClient{base: base, token: token, http: &http.Client{Timeout: time.Minute}}, nil
}

var nextLinkPattern = regexp.MustCompile(`<([^>]+)>;\s*rel="next"`)

func (c *githubClient) walk(ctx context.Context, path string, params url.Values, page func(*json.Decoder) error) error {
	params.Set("per_page", "100")
	target := c.base.JoinPath(path)
	target.RawQuery = params.Encode()
	for next := target.String(); next != ""; {
		link, err := c.get(ctx, next, page)
		if err != nil {
			return err
		}
		next = link
	}
	return nil
}

func (c *githubClient) get(ctx context.Context, target string, page func(*json.Decoder) error) (string, error) {
	request, err := c.request(ctx, target)
	if err != nil {
		return "", err
	}
	response, err := c.http.Do(request)
	if err != nil {
		return "", fmt.Errorf("GET %s: %w", request.URL.Path, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", responseError(request.URL.Path, response)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 32<<20+1))
	if err != nil {
		return "", err
	}
	if len(body) > 32<<20 {
		return "", fmt.Errorf("GET %s: response exceeds 32 MiB", request.URL.Path)
	}
	if err = page(json.NewDecoder(bytes.NewReader(body))); err != nil {
		return "", fmt.Errorf("GET %s: unexpected response: %w", request.URL.Path, err)
	}
	if c.captured != nil {
		c.captured[request.URL.RequestURI()] = json.RawMessage(body)
	}
	return nextLink(response.Header), nil
}

func (c *githubClient) request(ctx context.Context, target string) (*http.Request, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	if !c.sameOrigin(request.URL) {
		return nil, fmt.Errorf("GitHub returned a page link outside %s", c.base.Host)
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	request.Header.Set("User-Agent", "columbo-feedback")
	request.Header.Set("Authorization", "Bearer "+c.token)
	return request, nil
}

func nextLink(header http.Header) string {
	if match := nextLinkPattern.FindStringSubmatch(header.Get("Link")); match != nil {
		return match[1]
	}
	return ""
}

func responseError(path string, response *http.Response) error {
	if limit := rateLimit(response); limit != "" {
		return fmt.Errorf("GET %s: GitHub rate limit reached; try again %s", path, limit)
	}
	var payload struct{ Message string }
	_ = json.NewDecoder(io.LimitReader(response.Body, 1<<16)).Decode(&payload)
	switch response.StatusCode {
	case http.StatusUnauthorized:
		return fmt.Errorf("GET %s: GitHub rejected the token (401); set GH_TOKEN or run gh auth login", path)
	case http.StatusForbidden:
		return fmt.Errorf("GET %s: the token may not read this (403): %s", path, payload.Message)
	case http.StatusNotFound:
		return fmt.Errorf("GET %s: not found (404); check the repository name and that the token can read it", path)
	}
	return fmt.Errorf("GET %s: GitHub answered %s: %s", path, response.Status, payload.Message)
}

func rateLimit(response *http.Response) string {
	limited := response.StatusCode == http.StatusTooManyRequests ||
		(response.StatusCode == http.StatusForbidden && response.Header.Get("X-RateLimit-Remaining") == "0")
	if !limited {
		return ""
	}
	if seconds, err := strconv.ParseInt(response.Header.Get("X-RateLimit-Reset"), 10, 64); err == nil {
		return "after " + time.Unix(seconds, 0).UTC().Format(time.RFC3339)
	}
	return "later"
}

func list[T any](ctx context.Context, c *githubClient, path string, params url.Values) ([]T, error) {
	var all []T
	err := c.walk(ctx, path, params, func(decoder *json.Decoder) error {
		var items []T
		if err := decoder.Decode(&items); err != nil {
			return err
		}
		all = append(all, items...)
		return nil
	})
	return all, err
}

func one[T any](ctx context.Context, c *githubClient, path string) (T, error) {
	var item T
	err := c.walk(ctx, path, url.Values{}, func(decoder *json.Decoder) error { return decoder.Decode(&item) })
	return item, err
}

type ghUser struct {
	Login string `json:"login"`
}

func (u *ghUser) login() string {
	if u == nil || u.Login == "" {
		return "ghost"
	}
	return u.Login
}

type ghPull struct {
	ID             int64      `json:"id"`
	Number         int64      `json:"number"`
	Title          string     `json:"title"`
	User           *ghUser    `json:"user"`
	State          string     `json:"state"`
	Merged         bool       `json:"merged"`
	CreatedAt      time.Time  `json:"created_at"`
	ClosedAt       *time.Time `json:"closed_at"`
	Base           ghRef      `json:"base"`
	Head           ghRef      `json:"head"`
	MergeCommitSHA string     `json:"merge_commit_sha"`
}

type ghRef struct {
	SHA string `json:"sha"`
}

type ghComment struct {
	ID               int64     `json:"id"`
	User             *ghUser   `json:"user"`
	Body             string    `json:"body"`
	Path             *string   `json:"path"`
	Line             *int64    `json:"line"`
	OriginalLine     *int64    `json:"original_line"`
	CommitID         *string   `json:"commit_id"`
	OriginalCommitID *string   `json:"original_commit_id"`
	InReplyToID      *int64    `json:"in_reply_to_id"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	PullRequestURL   string    `json:"pull_request_url"`
	IssueURL         string    `json:"issue_url"`
}

type ghReaction struct {
	ID        int64     `json:"id"`
	User      *ghUser   `json:"user"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

type ghFile struct {
	Filename string  `json:"filename"`
	Patch    *string `json:"patch"`
}

type ghPullCommit struct {
	SHA    string  `json:"sha"`
	Author *ghUser `json:"author"`
	Commit struct {
		Message   string `json:"message"`
		Committer struct {
			Date time.Time `json:"date"`
		} `json:"committer"`
	} `json:"commit"`
}

type ghCommitDetail struct {
	Files []ghFile `json:"files"`
}

func (c *githubClient) sameOrigin(target *url.URL) bool {
	return target.Scheme == c.base.Scheme && target.Host == c.base.Host
}
