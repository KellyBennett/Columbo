package columbo

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const historyWarning = "Git history unavailable; static analysis completed without history evidence."

type historyCommit struct {
	hash  string
	stamp int64
	paths map[string]bool
}
type gitHistory struct {
	root, prefix string
	maximum      int64
	commits      []historyCommit
}

func (a *engine) history() {
	h := &gitHistory{root: a.root, maximum: a.config.MaxCommits}
	if e := h.read(); e != nil {
		a.report.Warnings = append(a.report.Warnings, Warning{"history-unavailable", "", 0, historyWarning})
		return
	}
	for i := range a.report.Cases {
		h.attach(&a.report.Cases[i])
	}
}
func (h *gitHistory) run(args ...string) (string, error) {
	c := exec.Command("git", args...)
	c.Dir = h.root
	b, e := c.Output()
	return string(b), e
}
func (h *gitHistory) read() error {
	if e := h.repositoryPrefix(); e != nil {
		return e
	}
	if e := h.commitList(); e != nil {
		return e
	}
	h.sortAndLimit()
	for i := range h.commits {
		if e := h.commitPaths(&h.commits[i]); e != nil {
			return e
		}
	}
	return nil
}
func (h *gitHistory) repositoryPrefix() error {
	root, e := h.run("rev-parse", "--show-toplevel")
	if e != nil {
		return e
	}
	h.prefix, e = filepath.Rel(strings.TrimSpace(root), h.root)
	return e
}
func (h *gitHistory) commitList() error {
	raw, e := h.run("log", "--format=%H %ct", "HEAD")
	if e != nil {
		return e
	}
	for _, line := range strings.Split(strings.TrimSpace(raw), "\n") {
		c, e := parseHistoryCommit(line)
		if e != nil {
			return e
		}
		h.commits = append(h.commits, c)
	}
	return nil
}
func parseHistoryCommit(line string) (historyCommit, error) {
	parts := strings.Fields(line)
	if len(parts) != 2 {
		return historyCommit{}, fmt.Errorf("invalid Git history record")
	}
	stamp, e := strconv.ParseInt(parts[1], 10, 64)
	return historyCommit{hash: parts[0], stamp: stamp}, e
}
func (h *gitHistory) sortAndLimit() {
	sort.Slice(h.commits, func(i, j int) bool {
		x, y := h.commits[i], h.commits[j]
		if x.stamp != y.stamp {
			return x.stamp > y.stamp
		}
		return x.hash < y.hash
	})
	if int64(len(h.commits)) > h.maximum {
		h.commits = h.commits[:h.maximum]
	}
}
func (h *gitHistory) commitPaths(c *historyCommit) error {
	parents, e := h.run("rev-list", "--parents", "-n", "1", c.hash)
	if e != nil {
		return e
	}
	args, e := historyDiffArgs(c.hash, parents)
	if e != nil {
		return e
	}
	raw, e := h.run(args...)
	if e != nil {
		return e
	}
	c.paths = historyPaths(raw)
	return nil
}
func historyDiffArgs(hash, parents string) ([]string, error) {
	fields := strings.Fields(parents)
	if len(fields) == 0 {
		return nil, fmt.Errorf("invalid Git parent record")
	}
	args := []string{"diff-tree", "--no-commit-id", "--name-only", "--no-renames", "-r", "-z"}
	if len(fields) > 1 {
		return append(args, fields[1], hash), nil
	}
	return append(args, "--root", hash), nil
}
func historyPaths(raw string) map[string]bool {
	paths := map[string]bool{}
	for _, p := range strings.Split(raw, "\x00") {
		paths[p] = true
	}
	return paths
}
func (c *Case) sourceFiles() map[string]bool {
	files := map[string]bool{}
	for _, r := range c.Receipts {
		if s, ok := r.(Source); ok {
			files[s.File] = true
		}
	}
	return files
}
func (h *gitHistory) attach(c *Case) {
	files := c.sourceFiles()
	for _, commit := range h.commits {
		if matching := commit.matchingFiles(h.prefix, files); len(matching) > 0 {
			c.Receipts = append(c.Receipts, History{"history", commit.hash, commit.stamp, matching})
		}
	}
}
func (c historyCommit) matchingFiles(prefix string, files map[string]bool) []string {
	matching := []string{}
	for f := range files {
		if c.paths[filepath.ToSlash(filepath.Join(prefix, f))] {
			matching = append(matching, f)
		}
	}
	sort.Strings(matching)
	return matching
}
