package trials

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/KellyBennett/Columbo/calibration/harness"
)

type PlanRow struct {
	Smell, Operator, Corpus string
}

// ReadPlan reads tab-separated smell, inject operator, and corpus lines; # starts a comment.
func ReadPlan(path string) ([]PlanRow, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var rows []PlanRow
	sc := bufio.NewScanner(f)
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 3 {
			return nil, fmt.Errorf("%s:%d: want smell, operator, corpus", path, n)
		}
		rows = append(rows, PlanRow{Smell: fields[0], Operator: fields[1], Corpus: fields[2]})
	}
	return rows, sc.Err()
}

// Source is a module to copy: a fixture directory, or a pinned module@version from the module cache.
type Source struct {
	Dir, Module, Version string
}

func Resolve(spec, planDir string) (Source, error) {
	module, version, pinned := strings.Cut(spec, "@")
	if !pinned {
		dir := spec
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(planDir, dir)
		}
		return Source{Dir: dir, Module: filepath.Base(dir), Version: "fixture"}, nil
	}
	dir, err := download(module, version)
	return Source{Dir: dir, Module: module, Version: version}, err
}

// download asks the go command for the checksum-verified module cache directory.
func download(module, version string) (string, error) {
	probe, err := harness.TempDir("calibration-download-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(probe)
	if err := os.WriteFile(filepath.Join(probe, "go.mod"), []byte("module calibration.probe\n"), 0o644); err != nil {
		return "", err
	}
	cmd := exec.Command("go", "mod", "download", "-json", module+"@"+version)
	cmd.Dir = probe
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("go mod download %s@%s: %v", module, version, err)
	}
	var info struct{ Dir string }
	if err := json.Unmarshal(out, &info); err != nil || info.Dir == "" {
		return "", fmt.Errorf("go mod download %s@%s: no directory", module, version)
	}
	return info.Dir, nil
}

// Trial is one agent attempt at one planted mutant; Repeat tells attempts at the same mutant apart.
type Trial struct {
	Plant  *Plant
	Repeat int
}

// Schedule gives each smell perSmell trials, spread over corpora first, then sites, then doses.
// When a smell has fewer mutants than trials it repeats them, since agent nondeterminism is part of the spread.
func Schedule(plants []*Plant, perSmell int) []Trial {
	bySmell := map[string][]*Plant{}
	var smells []string
	for _, p := range plants {
		if _, ok := bySmell[p.Smell]; !ok {
			smells = append(smells, p.Smell)
		}
		bySmell[p.Smell] = append(bySmell[p.Smell], p)
	}
	var out []Trial
	for _, smell := range smells {
		ps := spread(bySmell[smell])
		for i := 0; i < perSmell; i++ {
			out = append(out, Trial{Plant: ps[i%len(ps)], Repeat: i / len(ps)})
		}
	}
	return out
}

// spread orders plants so each round takes one mutant per corpus before any corpus contributes a second site or dose.
func spread(ps []*Plant) []*Plant {
	type ranked struct {
		p                  *Plant
		dose, site, corpus int
	}
	corpusRank, siteRank, doseRank := map[string]int{}, map[string]int{}, map[string]int{}
	sites := map[string]int{}
	rs := make([]ranked, len(ps))
	for i, p := range ps {
		corpus := p.Corpus.Module
		if _, ok := corpusRank[corpus]; !ok {
			corpusRank[corpus] = len(corpusRank)
		}
		site := strings.Join([]string{corpus, p.Operator, p.Site.Symbol, fmt.Sprint(p.Site.Offset)}, "|")
		if _, ok := siteRank[site]; !ok {
			siteRank[site] = sites[corpus]
			sites[corpus]++
		}
		rs[i] = ranked{p, doseRank[site], siteRank[site], corpusRank[corpus]}
		doseRank[site]++
	}
	sort.SliceStable(rs, func(i, j int) bool {
		a, b := rs[i], rs[j]
		if a.dose != b.dose {
			return a.dose < b.dose
		}
		if a.site != b.site {
			return a.site < b.site
		}
		return a.corpus < b.corpus
	})
	out := make([]*Plant, len(rs))
	for i, r := range rs {
		out[i] = r.p
	}
	return out
}
