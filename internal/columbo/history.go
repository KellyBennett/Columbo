package columbo

import (
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const historyWarning = "Git history unavailable; static analysis completed without history evidence."

func (a *engine) history() {
	run := func(args ...string) (string, error) {
		c := exec.Command("git", args...)
		c.Dir = a.root
		b, e := c.Output()
		return string(b), e
	}
	fail := func() {
		a.report.Warnings = append(a.report.Warnings, Warning{"history-unavailable", "", 0, historyWarning})
	}
	root, e := run("rev-parse", "--show-toplevel")
	if e != nil {
		fail()
		return
	}
	prefix, e := filepath.Rel(strings.TrimSpace(root), a.root)
	if e != nil {
		fail()
		return
	}
	raw, e := run("log", "--format=%H %ct", "HEAD")
	if e != nil {
		fail()
		return
	}
	type commit struct {
		hash  string
		stamp int64
		paths map[string]bool
	}
	commits := []commit{}
	for _, line := range strings.Split(strings.TrimSpace(raw), "\n") {
		parts := strings.Fields(line)
		if len(parts) != 2 {
			fail()
			return
		}
		stamp, e := strconv.ParseInt(parts[1], 10, 64)
		if e != nil {
			fail()
			return
		}
		commits = append(commits, commit{parts[0], stamp, nil})
	}
	sort.Slice(commits, func(i, j int) bool {
		if commits[i].stamp != commits[j].stamp {
			return commits[i].stamp > commits[j].stamp
		}
		return commits[i].hash < commits[j].hash
	})
	if int64(len(commits)) > a.config.MaxCommits {
		commits = commits[:a.config.MaxCommits]
	}
	for i := range commits {
		parents, e := run("rev-list", "--parents", "-n", "1", commits[i].hash)
		if e != nil {
			fail()
			return
		}
		fields := strings.Fields(parents)
		if len(fields) == 0 {
			fail()
			return
		}
		args := []string{"diff-tree", "--no-commit-id", "--name-only", "--no-renames", "-r", "-z"}
		if len(fields) > 1 {
			args = append(args, fields[1], commits[i].hash)
		} else {
			args = append(args, "--root", commits[i].hash)
		}
		r, e := run(args...)
		if e != nil {
			fail()
			return
		}
		commits[i].paths = map[string]bool{}
		for _, p := range strings.Split(r, "\x00") {
			commits[i].paths[p] = true
		}
	}
	for i := range a.report.Cases {
		c := &a.report.Cases[i]
		files := map[string]bool{}
		for _, r := range c.Receipts {
			if s, ok := r.(Source); ok {
				files[s.File] = true
			}
		}
		for _, commit := range commits {
			matching := []string{}
			for f := range files {
				path := filepath.ToSlash(filepath.Join(prefix, f))
				if commit.paths[path] {
					matching = append(matching, f)
				}
			}
			if len(matching) > 0 {
				sort.Strings(matching)
				c.Receipts = append(c.Receipts, History{"history", commit.hash, commit.stamp, matching})
			}
		}
	}
}
