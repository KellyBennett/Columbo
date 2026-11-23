package feedback

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"path"
	"slices"
	"strconv"
	"time"

	"github.com/KellyBennett/Columbo/internal/feedbackdb"
)

type CollectOptions struct {
	Repositories []string
	StorePath    string
	APIURL       string
	Token        string
	Now          func() time.Time
	Checkout     string
	EvidencePath string
	Artifacts    bool
	PullRequests []int64
}

func Collect(ctx context.Context, options CollectOptions, out io.Writer) error {
	client, err := newGitHubClient(options.APIURL, options.Token)
	if err != nil {
		return err
	}
	store, err := feedbackdb.Open(options.StorePath)
	if err != nil {
		return err
	}
	defer store.Close()
	for _, repository := range options.Repositories {
		c := collector{github: client, store: store, repository: repository, run: stamp(options.Now()), options: options}
		if err = c.collect(ctx, out); err != nil {
			return fmt.Errorf("%s: %w", repository, err)
		}
	}
	return nil
}

func stamp(t time.Time) string { return t.UTC().Format(time.RFC3339) }

type collector struct {
	github     *githubClient
	store      *feedbackdb.Store
	repository string
	run        string
	options    CollectOptions
}

func (c collector) collect(ctx context.Context, out io.Writer) error {
	if err := c.store.Update(ctx, func(q feedbackdb.Querier) error {
		return q.UpsertRepository(ctx, feedbackdb.UpsertRepositoryParams{Name: c.repository, Seen: c.run})
	}); err != nil {
		return err
	}
	numbers, err := c.pullRequests(ctx)
	if err != nil {
		return err
	}
	summary := repositorySummary{repository: c.repository}
	for _, number := range numbers {
		record, err := c.save(ctx, number)
		if err != nil {
			return fmt.Errorf("pull request %d: %w", number, err)
		}
		summary.add(record)
	}
	fmt.Fprintln(out, summary)
	if len(c.options.PullRequests) > 0 {
		return nil
	}
	return c.store.Update(ctx, func(q feedbackdb.Querier) error {
		return q.SetRepositoryScannedThrough(ctx, feedbackdb.SetRepositoryScannedThroughParams{ScannedThrough: &c.run, Name: c.repository})
	})
}

func (c collector) save(ctx context.Context, number int64) (PullRequestRecord, error) {
	fetched, err := c.fetch(ctx, number)
	if err != nil {
		return PullRequestRecord{}, err
	}
	record := fetched.record()
	observation, err := c.observe(ctx, number, fetched, record)
	if err != nil {
		return record, err
	}
	return record, c.store.Update(ctx, func(q feedbackdb.Querier) error {
		if err := saveRecord(ctx, q, record, c.run); err != nil {
			return err
		}
		return q.InsertObservation(ctx, feedbackdb.InsertObservationParams{
			ID: observation.ID, Repository: c.repository, Number: number,
			ObservedAt: observation.ObservedAt, Payload: observation.payload(),
		})
	})
}

func (c collector) pullRequests(ctx context.Context) ([]int64, error) {
	if len(c.options.PullRequests) > 0 {
		return c.options.PullRequests, nil
	}
	numbers, err := c.store.Queries().PullRequestNumbers(ctx, feedbackdb.PullRequestNumbersParams{Repository: c.repository})
	if err != nil {
		return nil, err
	}
	scanned, err := c.store.Queries().RepositoryScannedThrough(ctx, feedbackdb.RepositoryScannedThroughParams{Name: c.repository})
	if err != nil {
		return nil, err
	}
	for _, source := range []string{"pulls/comments", "issues/comments"} {
		found, err := c.discover(ctx, source, scanned)
		if err != nil {
			return nil, err
		}
		numbers = append(numbers, found...)
	}
	slices.Sort(numbers)
	return slices.Compact(numbers), nil
}

func (c collector) discover(ctx context.Context, source string, since *string) ([]int64, error) {
	params := url.Values{}
	if since != nil {
		params.Set("since", *since)
	}
	comments, err := list[ghComment](ctx, c.github, "repos/"+c.repository+"/"+source, params)
	if err != nil {
		return nil, err
	}
	var numbers []int64
	for _, comment := range comments {
		if parseMarker(comment.Body) == nil {
			continue
		}
		number, err := comment.pullRequestNumber()
		if err != nil {
			return nil, err
		}
		numbers = append(numbers, number)
	}
	return numbers, nil
}

func (c ghComment) pullRequestNumber() (int64, error) {
	link := c.PullRequestURL
	if link == "" {
		link = c.IssueURL
	}
	number, err := strconv.ParseInt(path.Base(link), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("comment %d does not link to a pull request number", c.ID)
	}
	return number, nil
}

func (c collector) fetch(ctx context.Context, number int64) (fetchedPullRequest, error) {
	pull := fmt.Sprintf("repos/%s/pulls/%d", c.repository, number)
	f := fetchedPullRequest{repository: c.repository, reactions: map[commentKey][]ghReaction{}, commitFiles: map[string][]ghFile{}, observationID: rand.Text()}
	f.resources = map[string]json.RawMessage{}
	c.github.captured = f.resources
	steps := []func() error{
		func() (err error) { f.pull, err = one[ghPull](ctx, c.github, pull); return },
		func() (err error) {
			f.reviewComments, err = list[ghComment](ctx, c.github, pull+"/comments", url.Values{})
			return
		},
		func() (err error) {
			f.issueComments, err = list[ghComment](ctx, c.github, fmt.Sprintf("repos/%s/issues/%d/comments", c.repository, number), url.Values{})
			return
		},
		func() (err error) { f.files, err = list[ghFile](ctx, c.github, pull+"/files", url.Values{}); return },
		func() (err error) {
			f.commits, err = list[ghPullCommit](ctx, c.github, pull+"/commits", url.Values{})
			return
		},
		func() error {
			return c.fetchReactions(ctx, commentSource{ReviewComment, "pulls", f.reviewComments}, f.reactions)
		},
		func() error {
			return c.fetchReactions(ctx, commentSource{IssueComment, "issues", f.issueComments}, f.reactions)
		},
		func() error { return c.fetchCommitFiles(ctx, &f) },
	}
	for _, step := range steps {
		if err := step(); err != nil {
			return f, err
		}
	}
	return f, nil
}

type commentSource struct {
	kind     CommentKind
	path     string
	comments []ghComment
}

func (c collector) fetchReactions(ctx context.Context, source commentSource, into map[commentKey][]ghReaction) error {
	for _, comment := range source.comments {
		if parseMarker(comment.Body) == nil {
			continue
		}
		reactions, err := list[ghReaction](ctx, c.github, fmt.Sprintf("repos/%s/%s/comments/%d/reactions", c.repository, source.path, comment.ID), url.Values{})
		if err != nil {
			return err
		}
		into[commentKey{source.kind, comment.ID}] = reactions
	}
	return nil
}

func (c collector) fetchCommitFiles(ctx context.Context, f *fetchedPullRequest) error {
	for _, commit := range f.commits {
		files, err := c.commitFiles(ctx, commit.SHA)
		if err != nil {
			return err
		}
		f.commitFiles[commit.SHA] = files
	}
	return nil
}

func (c collector) commitFiles(ctx context.Context, sha string) ([]ghFile, error) {
	var files []ghFile
	err := c.github.walk(ctx, "repos/"+c.repository+"/commits/"+sha, url.Values{}, func(decoder *json.Decoder) error {
		var detail ghCommitDetail
		err := decoder.Decode(&detail)
		files = append(files, detail.Files...)
		return err
	})
	return files, err
}

type repositorySummary struct {
	repository                                string
	pullRequests, comments, reactions, misses int
}

func (s *repositorySummary) add(record PullRequestRecord) {
	s.pullRequests++
	s.comments += len(record.Comments)
	s.reactions += len(record.Reactions)
	for _, comment := range record.Comments {
		if _, ok := missText(comment.Body); ok && comment.Marker == nil {
			s.misses++
		}
	}
}

func (s repositorySummary) String() string {
	return fmt.Sprintf("%s: %s, %s, %s, %s", s.repository, count(s.pullRequests, "pull request", "pull requests"),
		count(s.comments, "comment", "comments"), count(s.reactions, "reaction", "reactions"), count(s.misses, "miss", "misses"))
}

func count(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}
