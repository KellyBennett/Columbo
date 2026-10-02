package columbo

import (
	"fmt"
	"github.com/bmatcuk/doublestar/v4"
	"gopkg.in/yaml.v3"
	"io"
	"math"
	"os"
	"strings"
)

type Config struct {
	Severity   map[string]string
	Counts     map[string]int64
	Ratios     map[string]float64
	History    bool
	MaxCommits int64
	Exclude    []string
}

func Defaults() Config {
	c := Config{map[string]string{}, map[string]int64{"function-lines": 10, "parameters": 4, "cognitive-complexity": 7, "dependencies": 5, "feature-envy-foreign-accesses": 5, "data-clump-size": 3, "data-clump-occurrences": 3, "cosmetic-min-helpers": 2}, map[string]float64{"feature-envy-ratio": 2, "cosmetic-dependency-overlap": .75, "cosmetic-parameter-overlap": .75}, true, 500, []string{"**/*_generated.go", "**/vendor/**"}}
	for _, s := range smells {
		c.Severity[s] = "fail"
	}
	return c
}
func LoadConfig(path string, explicit bool) (Config, error) {
	c := Defaults()
	b, e := os.ReadFile(path)
	if os.IsNotExist(e) && !explicit {
		return c, nil
	}
	if e != nil {
		return c, e
	}
	d := yaml.NewDecoder(strings.NewReader(string(b)))
	var n yaml.Node
	if e = d.Decode(&n); e == io.EOF {
		return c, nil
	}
	if e != nil {
		return c, e
	}
	var extra yaml.Node
	if e = d.Decode(&extra); e != io.EOF {
		return c, fmt.Errorf("configuration must contain exactly one YAML document")
	}
	if len(n.Content) != 1 || n.Content[0].Kind != yaml.MappingNode {
		return c, fmt.Errorf("configuration root must be a mapping")
	}
	if e = validateYAML(n.Content[0]); e != nil {
		return c, e
	}
	err := mapping(n.Content[0], func(k string, v *yaml.Node) error {
		switch k {
		case "version":
			i, e := integer(v)
			if e != nil || i != 1 {
				return fmt.Errorf("version must be integer 1")
			}
		case "severity":
			return mapping(v, func(k string, v *yaml.Node) error {
				if _, ok := c.Severity[k]; !ok {
					return fmt.Errorf("unknown smell %s", k)
				}
				s, e := str(v)
				if e != nil {
					return e
				}
				if s != "off" && s != "warn" && s != "fail" {
					return fmt.Errorf("invalid severity %s", s)
				}
				c.Severity[k] = s
				return nil
			})
		case "thresholds":
			return mapping(v, func(k string, v *yaml.Node) error {
				if _, ok := c.Counts[k]; ok {
					i, e := integer(v)
					if e != nil || i < 1 || k == "cosmetic-min-helpers" && i < 2 {
						return fmt.Errorf("invalid positive integer threshold %s", k)
					}
					c.Counts[k] = i
					return nil
				}
				if _, ok := c.Ratios[k]; !ok {
					return fmt.Errorf("unknown threshold %s", k)
				}
				if v.Kind != yaml.ScalarNode || (v.Tag != "!!int" && v.Tag != "!!float") {
					return fmt.Errorf("%s must be numeric", k)
				}
				var f float64
				if e := v.Decode(&f); e != nil {
					return e
				}
				if math.IsNaN(f) || math.IsInf(f, 0) || f < 0 || k == "feature-envy-ratio" && f == 0 || k != "feature-envy-ratio" && f > 1 {
					return fmt.Errorf("invalid ratio %s", k)
				}
				c.Ratios[k] = f
				return nil
			})
		case "history":
			return mapping(v, func(k string, v *yaml.Node) error {
				switch k {
				case "enabled":
					if v.Tag != "!!bool" || v.Kind != yaml.ScalarNode {
						return fmt.Errorf("history.enabled must be boolean")
					}
					return v.Decode(&c.History)
				case "max-commits":
					i, e := integer(v)
					if e != nil || i < 1 {
						return fmt.Errorf("history.max-commits must be positive integer")
					}
					c.MaxCommits = i
					return nil
				default:
					return fmt.Errorf("unknown history key %s", k)
				}
			})
		case "exclude":
			if v.Kind != yaml.SequenceNode {
				return fmt.Errorf("exclude must be a sequence")
			}
			c.Exclude = []string{}
			for _, x := range v.Content {
				s, e := str(x)
				if e != nil {
					return e
				}
				if !doublestar.ValidatePattern(s) {
					return fmt.Errorf("invalid exclusion glob %q", s)
				}
				c.Exclude = append(c.Exclude, s)
			}
		default:
			return fmt.Errorf("unknown configuration key %s", k)
		}
		return nil
	})
	return c, err
}
func validateYAML(n *yaml.Node) error {
	if n.Kind == yaml.AliasNode || n.Tag == "!!null" || n.Tag == "!!merge" {
		return fmt.Errorf("nulls, aliases, anchors and merge keys are not allowed")
	}
	if n.Kind == yaml.MappingNode {
		seen := map[string]bool{}
		for i := 0; i < len(n.Content); i += 2 {
			k := n.Content[i]
			if k.Kind != yaml.ScalarNode || k.Tag != "!!str" {
				return fmt.Errorf("mapping keys must be strings")
			}
			if seen[k.Value] {
				return fmt.Errorf("duplicate key %s", k.Value)
			}
			seen[k.Value] = true
		}
	}
	for _, x := range n.Content {
		if e := validateYAML(x); e != nil {
			return e
		}
	}
	return nil
}
func mapping(n *yaml.Node, f func(string, *yaml.Node) error) error {
	if n.Kind != yaml.MappingNode {
		return fmt.Errorf("expected mapping")
	}
	for i := 0; i < len(n.Content); i += 2 {
		if e := f(n.Content[i].Value, n.Content[i+1]); e != nil {
			return e
		}
	}
	return nil
}
func integer(n *yaml.Node) (int64, error) {
	var i int64
	if n.Kind != yaml.ScalarNode || n.Tag != "!!int" {
		return i, fmt.Errorf("expected integer")
	}
	e := n.Decode(&i)
	return i, e
}
func str(n *yaml.Node) (string, error) {
	if n.Kind != yaml.ScalarNode || n.Tag != "!!str" {
		return "", fmt.Errorf("expected string")
	}
	return n.Value, nil
}
