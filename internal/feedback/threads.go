package feedback

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const reviewThreadsQuery = `query ColumboReviewThreads($owner:String!,$name:String!,$number:Int!,$after:String) {
 repository(owner:$owner,name:$name) { pullRequest(number:$number) {
  reviewThreads(first:100,after:$after) {
   nodes { id isResolved isOutdated path line originalLine startLine originalStartLine diffSide startDiffSide
    resolvedBy { login } comments(first:1) { nodes { databaseId } } }
   pageInfo { hasNextPage endCursor }
  }
 } }
}`

func (c *githubClient) threads(ctx context.Context, repository string, number int64) error {
	parts := strings.SplitN(repository, "/", 2)
	endpoint := *c.base
	endpoint.Path = strings.TrimSuffix(endpoint.Path, "/")
	if strings.HasSuffix(endpoint.Path, "/api/v3") {
		endpoint.Path = strings.TrimSuffix(endpoint.Path, "/api/v3") + "/api/graphql"
	} else {
		endpoint.Path += "/graphql"
	}
	var after *string
	for {
		body, err := json.Marshal(map[string]any{"query": reviewThreadsQuery, "variables": map[string]any{
			"owner": parts[0], "name": parts[1], "number": number, "after": after,
		}})
		if err != nil {
			return err
		}
		request, err := c.request(ctx, endpoint.String())
		if err != nil {
			return err
		}
		request.Method, request.Body = http.MethodPost, io.NopCloser(bytes.NewReader(body))
		request.ContentLength = int64(len(body))
		request.Header.Set("Content-Type", "application/json")
		response, err := c.http.Do(request)
		if err != nil {
			return fmt.Errorf("review threads request failed")
		}
		data, readErr := io.ReadAll(io.LimitReader(response.Body, 32<<20+1))
		response.Body.Close()
		if readErr != nil {
			return readErr
		}
		if response.StatusCode != http.StatusOK {
			return fmt.Errorf("review threads: HTTP %d", response.StatusCode)
		}
		if len(data) > 32<<20 {
			return fmt.Errorf("review threads response exceeds 32 MiB")
		}
		var result struct {
			Errors []struct{ Message string } `json:"errors"`
			Data   struct {
				Repository *struct {
					PullRequest *struct {
						Threads struct {
							Page struct {
								HasNext bool    `json:"hasNextPage"`
								End     *string `json:"endCursor"`
							} `json:"pageInfo"`
						} `json:"reviewThreads"`
					} `json:"pullRequest"`
				} `json:"repository"`
			} `json:"data"`
		}
		if err := json.Unmarshal(data, &result); err != nil {
			return err
		}
		cursor := ""
		if after != nil {
			cursor = *after
		}
		if c.captured != nil {
			c.captured[fmt.Sprintf("graphql/review-threads/%d/%s", number, cursor)] = data
		}
		if len(result.Errors) > 0 {
			return fmt.Errorf("review threads: %s", result.Errors[0].Message)
		}
		if result.Data.Repository == nil || result.Data.Repository.PullRequest == nil {
			return fmt.Errorf("review threads unavailable")
		}
		page := result.Data.Repository.PullRequest.Threads.Page
		if !page.HasNext {
			return nil
		}
		if page.End == nil || *page.End == cursor {
			return fmt.Errorf("review threads pagination did not advance")
		}
		after = page.End
	}
}
