package feedback

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type EvidenceGap struct {
	Source string `json:"source"`
	Reason string `json:"reason"`
}

type Revision struct {
	SHA     string   `json:"sha"`
	Roles   []string `json:"roles"`
	Present bool     `json:"present"`
	Parents []string `json:"parents,omitempty"`
}

type CIArtifact struct {
	ID         int64           `json:"id"`
	RunID      int64           `json:"run_id"`
	Name       string          `json:"name"`
	Path       string          `json:"path,omitempty"`
	SHA256     string          `json:"sha256,omitempty"`
	Provenance json.RawMessage `json:"provenance,omitempty"`
}

type Observation struct {
	Version    int                        `json:"version"`
	ID         string                     `json:"id"`
	ObservedAt string                     `json:"observed_at"`
	APIURL     string                     `json:"api_url"`
	Record     PullRequestRecord          `json:"record"`
	Resources  map[string]json.RawMessage `json:"resources"`
	Revisions  []Revision                 `json:"revisions"`
	GitDir     string                     `json:"git_dir,omitempty"`
	Artifacts  []CIArtifact               `json:"artifacts,omitempty"`
	Gaps       []EvidenceGap              `json:"gaps,omitempty"`
}

func (o Observation) payload() string {
	data, _ := json.Marshal(o)
	return string(data)
}

func (o *Observation) gap(source string, err error) {
	o.Gaps = append(o.Gaps, EvidenceGap{Source: source, Reason: err.Error()})
}

func (c collector) observe(ctx context.Context, number int64, f fetchedPullRequest, record PullRequestRecord) (Observation, error) {
	o := Observation{Version: 1, ID: f.observationID, ObservedAt: c.options.Now().UTC().Format(time.RFC3339Nano),
		APIURL: c.github.base.String(), Record: record, Resources: f.resources}
	pull := fmt.Sprintf("repos/%s/pulls/%d", c.repository, number)
	for _, endpoint := range []string{pull + "/reviews", fmt.Sprintf("repos/%s/issues/%d/timeline", c.repository, number)} {
		if _, err := list[json.RawMessage](ctx, c.github, endpoint, url.Values{}); err != nil {
			o.gap(endpoint, err)
		}
	}
	if err := c.github.threads(ctx, c.repository, number); err != nil {
		o.gap("review thread resolution", err)
	}
	o.addRevision(f.pull.Base.SHA, "pr-base")
	o.addRevision(f.pull.Head.SHA, "pr-head")
	if f.pull.Merged {
		o.addRevision(f.pull.MergeCommitSHA, "merge-result")
	}
	for _, commit := range f.commits {
		o.addRevision(commit.SHA, "pr-commit")
	}
	for _, comment := range f.reviewComments {
		if comment.OriginalCommitID != nil {
			o.addRevision(*comment.OriginalCommitID, "original-review-anchor")
		}
		if comment.CommitID != nil {
			o.addRevision(*comment.CommitID, "current-review-anchor")
		}
	}
	c.captureCI(ctx, &o)
	if c.options.Checkout != "" {
		gitDir, err := c.retainHistory(ctx, &o)
		if err != nil {
			return o, fmt.Errorf("retain Git history: %w", err)
		}
		o.GitDir = gitDir
	} else {
		o.gap("Git history", fmt.Errorf("not retained; collect with --checkout to enable refactoring trials"))
	}
	o.gap("historical completeness", fmt.Errorf("only currently available and previously collected evidence is known; deleted comments, earlier edits and unreachable force-pushed revisions may be unavailable"))
	return o, nil
}

func (o *Observation) addRevision(sha, role string) {
	if sha == "" {
		return
	}
	for i := range o.Revisions {
		if o.Revisions[i].SHA == sha {
			for _, existing := range o.Revisions[i].Roles {
				if role == existing {
					return
				}
			}
			o.Revisions[i].Roles = append(o.Revisions[i].Roles, role)
			return
		}
	}
	o.Revisions = append(o.Revisions, Revision{SHA: sha, Roles: []string{role}})
}

func (c collector) evidenceRoot() (string, error) {
	root := c.options.EvidencePath
	if root == "" {
		root = c.options.StorePath + ".evidence"
	}
	return filepath.Abs(root)
}

func (c collector) captureCI(ctx context.Context, o *Observation) {
	shas := map[string]bool{}
	for _, revision := range o.Revisions {
		for _, role := range revision.Roles {
			if role == "original-review-anchor" || role == "current-review-anchor" || role == "pr-head" || role == "pr-commit" {
				shas[revision.SHA] = true
			}
		}
	}
	var ordered []string
	for sha := range shas {
		ordered = append(ordered, sha)
	}
	sort.Strings(ordered)
	seen := map[int64]bool{}
	for _, sha := range ordered {
		endpoint := "repos/" + c.repository + "/actions/runs"
		err := c.github.walk(ctx, endpoint, url.Values{"head_sha": {sha}}, func(decoder *json.Decoder) error {
			var page struct {
				Runs []struct {
					ID      int64  `json:"id"`
					HeadSHA string `json:"head_sha"`
				} `json:"workflow_runs"`
			}
			if err := decoder.Decode(&page); err != nil {
				return err
			}
			for _, run := range page.Runs {
				if !seen[run.ID] {
					seen[run.ID] = true
					c.captureArtifacts(ctx, o, run.ID)
				}
			}
			return nil
		})
		if err != nil {
			o.gap("workflow runs for "+sha, err)
		}
	}
	if len(o.Artifacts) == 0 {
		o.gap("original analysis outputs", fmt.Errorf("no Columbo-named CI artifacts discovered; a later analysis must be labeled as a reconstruction"))
	}
}

func (c collector) captureArtifacts(ctx context.Context, o *Observation, runID int64) {
	endpoint := fmt.Sprintf("repos/%s/actions/runs/%d/artifacts", c.repository, runID)
	err := c.github.walk(ctx, endpoint, url.Values{}, func(decoder *json.Decoder) error {
		var page struct {
			Artifacts []struct {
				ID      int64  `json:"id"`
				Name    string `json:"name"`
				Expired bool   `json:"expired"`
			} `json:"artifacts"`
		}
		if err := decoder.Decode(&page); err != nil {
			return err
		}
		for _, a := range page.Artifacts {
			if !strings.Contains(strings.ToLower(a.Name), "columbo") {
				continue
			}
			artifact := CIArtifact{ID: a.ID, RunID: runID, Name: a.Name}
			switch {
			case a.Expired:
				o.gap(fmt.Sprintf("artifact %d", a.ID), fmt.Errorf("expired"))
			case !c.options.Artifacts:
				o.gap(fmt.Sprintf("artifact %d", a.ID), fmt.Errorf("not downloaded; use --artifacts"))
			default:
				if err := c.downloadArtifact(ctx, &artifact); err != nil {
					o.gap(fmt.Sprintf("artifact %d", a.ID), err)
				}
			}
			o.Artifacts = append(o.Artifacts, artifact)
		}
		return nil
	})
	if err != nil {
		o.gap(endpoint, err)
	}
}

func (c collector) downloadArtifact(ctx context.Context, artifact *CIArtifact) error {
	endpoint := c.github.base.JoinPath(fmt.Sprintf("repos/%s/actions/artifacts/%d/zip", c.repository, artifact.ID))
	request, err := c.github.request(ctx, endpoint.String())
	if err != nil {
		return err
	}
	response, err := c.github.http.Do(request)
	if err != nil {
		return fmt.Errorf("artifact download failed")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return responseError(endpoint.Path, response)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 128<<20+1))
	if err != nil {
		return fmt.Errorf("artifact download incomplete")
	}
	if len(data) > 128<<20 {
		return fmt.Errorf("artifact exceeds 128 MiB")
	}
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("artifact is not a ZIP archive: %w", err)
	}
	for _, entry := range reader.File {
		if filepath.Base(entry.Name) != "feedback-provenance.json" {
			continue
		}
		r, err := entry.Open()
		if err != nil {
			return err
		}
		provenance, err := io.ReadAll(io.LimitReader(r, 1<<20))
		r.Close()
		if err == nil && json.Valid(provenance) {
			artifact.Provenance = provenance
		}
	}
	digest := sha256.Sum256(data)
	artifact.SHA256 = hex.EncodeToString(digest[:])
	root, err := c.evidenceRoot()
	if err != nil {
		return err
	}
	artifact.Path = filepath.Join(root, "artifacts", artifact.SHA256+".zip")
	return writePrivate(artifact.Path, data)
}

var commitPattern = regexp.MustCompile(`^[0-9a-f]{40,64}$`)

func (c collector) retainHistory(ctx context.Context, o *Observation) (string, error) {
	source, err := filepath.Abs(c.options.Checkout)
	if err != nil {
		return "", err
	}
	if _, err := git(ctx, source, "rev-parse", "--git-dir"); err != nil {
		return "", err
	}
	root, err := c.evidenceRoot()
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256([]byte(c.github.base.String() + "/" + c.repository))
	archive := filepath.Join(root, "repositories", hex.EncodeToString(digest[:])+".git")
	if _, err := os.Stat(archive); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(archive), 0o700); err != nil {
			return "", err
		}
		if _, err := git(ctx, "", "clone", "--bare", "--no-local", source, archive); err != nil {
			return "", err
		}
	}
	if shallow, err := git(ctx, archive, "rev-parse", "--is-shallow-repository"); err == nil && strings.TrimSpace(shallow) == "true" {
		o.gap("Git ancestry", fmt.Errorf("retained repository is shallow; earlier history is incomplete"))
	}
	remote, _ := git(ctx, source, "remote", "get-url", "origin")
	remote = strings.TrimSpace(remote)
	parsed, _ := url.Parse(remote)
	if parsed != nil && parsed.User != nil {
		remote = ""
	}
	for _, artifact := range o.Artifacts {
		var p struct {
			AnalyzedCommit string `json:"analyzed_commit"`
		}
		if json.Unmarshal(artifact.Provenance, &p) == nil {
			o.addRevision(p.AnalyzedCommit, "analyzed-commit")
		}
	}
	for i := range o.Revisions {
		revision := &o.Revisions[i]
		if !commitPattern.MatchString(revision.SHA) {
			o.gap(revision.SHA, fmt.Errorf("invalid commit ID"))
			continue
		}
		_, err := git(ctx, archive, "cat-file", "-e", revision.SHA+"^{commit}")
		if err != nil {
			_, err = git(ctx, archive, "fetch", "--no-tags", source, revision.SHA)
		}
		if err != nil && remote != "" {
			_, err = git(ctx, archive, "fetch", "--no-tags", remote, revision.SHA)
		}
		if err != nil {
			o.gap(revision.SHA, fmt.Errorf("commit unavailable locally and from origin"))
			continue
		}
		parents, err := git(ctx, archive, "show", "-s", "--format=%P", revision.SHA)
		if err != nil {
			return "", err
		}
		revision.Parents = strings.Fields(parents)
		_, err = git(ctx, archive, "update-ref", "refs/columbo/"+o.ID+"/"+revision.SHA, revision.SHA)
		if err != nil {
			return "", err
		}
		revision.Present = true
	}

	return archive, nil
}

func writePrivate(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".columbo-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
