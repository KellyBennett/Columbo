package operators

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/KellyBennett/Columbo/calibration/harness"
	"gopkg.in/yaml.v3"
)

// frozenSuffix keeps frozen Go sources out of this module's ./... and out of the
// repository's database-boundary scan, which inspect every .go file.
const frozenSuffix = ".frozen"

// redTeam replays evasions an agent wrote against real Columbo output, frozen as whole files.
// No model runs during calibration; only these files do.
type redTeam struct {
	dir     string
	entries []redTeamEntry
}

type redTeamEntry struct {
	Name      string   `yaml:"-"`
	Module    string   `yaml:"module"`
	Version   string   `yaml:"version"`
	Smell     string   `yaml:"smell"`
	Symbol    string   `yaml:"symbol"`
	Touched   []string `yaml:"touched"`
	Technique string   `yaml:"technique"`
	Evaded    *bool    `yaml:"evaded"`
}

func RedTeam(dir string) (harness.Operator, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "*", "meta.yaml"))
	if err != nil {
		return nil, err
	}
	sort.Strings(matches)
	rt := redTeam{dir: dir}
	for _, m := range matches {
		data, err := os.ReadFile(m)
		if err != nil {
			return nil, err
		}
		var e redTeamEntry
		if err := yaml.Unmarshal(data, &e); err != nil {
			return nil, fmt.Errorf("%s: %w", m, err)
		}
		if e.Evaded != nil && !*e.Evaded {
			continue
		}
		e.Name = filepath.Base(filepath.Dir(m))
		rt.entries = append(rt.entries, e)
	}
	return rt, nil
}

func (redTeam) ID() string                                { return "redteam-replay" }
func (redTeam) Class() harness.Class                      { return harness.Evade }
func (redTeam) Target() string                            { return "" }
func (redTeam) Doses(*harness.Corpus, harness.Site) []int { return []int{1} }

func (o redTeam) Sites(c *harness.Corpus) []harness.Site {
	var out []harness.Site
	for i, e := range o.entries {
		if e.Module == c.Module && (e.Version == "" || e.Version == c.Version) {
			out = append(out, harness.Site{Package: e.Module, Symbol: e.Symbol, Offset: i, File: e.Name})
		}
	}
	return out
}

func (o redTeam) Apply(c *harness.Corpus, site harness.Site, _ int) (harness.Applied, error) {
	e := o.entries[site.Offset]
	root := filepath.Join(o.dir, e.Name, "files")
	edit := harness.Edit{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		data, err := os.ReadFile(path)
		edit[strings.TrimSuffix(filepath.ToSlash(rel), frozenSuffix)] = data
		return err
	})
	if err != nil {
		return harness.Applied{}, err
	}
	if len(edit) == 0 {
		return harness.Applied{}, errors.New("no frozen files")
	}
	return harness.Applied{
		Edit: edit,
		Expectation: harness.Expectation{
			Touched:      append([]string{e.Symbol}, e.Touched...),
			Catches:      []string{e.Smell, "cosmetic-extraction"},
			AnyAdded:     true,
			FromBaseline: true,
			Target:       e.Smell,
		},
	}, nil
}
