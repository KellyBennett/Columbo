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
	c := Config{map[string]string{}, map[string]int64{"function-lines": 10, "parameters": 4, "cognitive-complexity": 7, "dependencies": 5, "private-type-files": 3, "feature-envy-foreign-accesses": 5, "data-clump-size": 3, "data-clump-occurrences": 3, "cosmetic-min-helpers": 2, "duplicate-tokens": 50, "repeated-variant-sites": 2, "repeated-variant-variants": 2, "selection-use-implementations": 2}, map[string]float64{"feature-envy-ratio": 2, "cosmetic-dependency-overlap": .75, "cosmetic-parameter-overlap": .75}, true, 500, []string{"**/*_generated.go", "**/vendor/**"}}
	for _, s := range smells {
		c.Severity[s] = "fail"
	}
	c.Severity["duplicate-code"] = "warn"
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
	return decodeConfig(b, c)
}
func decodeConfig(b []byte, c Config) (Config, error) {
	n, e := configDocument(b)
	if e != nil || n == nil {
		return c, e
	}
	if e = validateYAML(n); e != nil {
		return c, e
	}
	return c, mapping(n, c.set)
}

type configReader struct{ decoder *yaml.Decoder }

func newConfigReader(data []byte) *configReader {
	return &configReader{yaml.NewDecoder(strings.NewReader(string(data)))}
}
func configDocument(data []byte) (*yaml.Node, error) { return newConfigReader(data).document() }
func (r *configReader) document() (*yaml.Node, error) {
	var document yaml.Node
	if err := r.decoder.Decode(&document); err != nil {
		if err == io.EOF {
			return nil, nil
		}
		return nil, err
	}
	if err := r.singleDocument(); err != nil {
		return nil, err
	}
	return configRoot(&document)
}
func (r *configReader) singleDocument() error {
	var extra yaml.Node
	if err := r.decoder.Decode(&extra); err != io.EOF {
		return fmt.Errorf("configuration must contain exactly one YAML document")
	}
	return nil
}
func configRoot(document *yaml.Node) (*yaml.Node, error) {
	if len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("configuration root must be a mapping")
	}
	return document.Content[0], nil
}

// The section schema dispatches assignment without changing validation order.
type configSetter func(*Config, *yaml.Node) error

func mappingSetter(set func(*Config, string, *yaml.Node) error) configSetter {
	return func(config *Config, node *yaml.Node) error {
		return mapping(node, func(key string, value *yaml.Node) error { return set(config, key, value) })
	}
}

var configSections = map[string]configSetter{
	"version":    func(_ *Config, node *yaml.Node) error { return configVersion(node) },
	"severity":   mappingSetter((*Config).setSeverity),
	"thresholds": mappingSetter((*Config).setThreshold),
	"history":    mappingSetter((*Config).setHistory),
	"exclude":    (*Config).setExclude,
}

func (c *Config) set(key string, value *yaml.Node) error {
	set := configSections[key]
	if set == nil {
		return fmt.Errorf("unknown configuration key %s", key)
	}
	return set(c, value)
}
func configVersion(v *yaml.Node) error {
	i, e := integer(v)
	if e != nil || i != 1 {
		return fmt.Errorf("version must be integer 1")
	}
	return nil
}
func (c *Config) setSeverity(k string, v *yaml.Node) error {
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
}
func (c *Config) setThreshold(k string, v *yaml.Node) error {
	if _, ok := c.Counts[k]; ok {
		return c.setCount(k, v)
	}
	if _, ok := c.Ratios[k]; !ok {
		return fmt.Errorf("unknown threshold %s", k)
	}
	f, e := configRatio(k, v)
	if e != nil {
		return e
	}
	c.Ratios[k] = f
	return nil
}
func (c *Config) setCount(k string, v *yaml.Node) error {
	i, e := integer(v)
	if e != nil || i < 1 || (k == "cosmetic-min-helpers" || k == "repeated-variant-sites" || k == "repeated-variant-variants" || k == "selection-use-implementations") && i < 2 {
		return fmt.Errorf("invalid positive integer threshold %s", k)
	}
	c.Counts[k] = i
	return nil
}
func configRatio(k string, v *yaml.Node) (float64, error) {
	if v.Kind != yaml.ScalarNode || (v.Tag != "!!int" && v.Tag != "!!float") {
		return 0, fmt.Errorf("%s must be numeric", k)
	}
	var f float64
	if e := v.Decode(&f); e != nil {
		return 0, e
	}
	if invalidRatio(k, f) {
		return 0, fmt.Errorf("invalid ratio %s", k)
	}
	return f, nil
}
func invalidRatio(k string, f float64) bool {
	if math.IsNaN(f) || math.IsInf(f, 0) || f < 0 {
		return true
	}
	if k == "feature-envy-ratio" {
		return f == 0
	}
	return f > 1
}
func (c *Config) setHistory(k string, v *yaml.Node) error {
	switch k {
	case "enabled":
		return historyEnabled(v, &c.History)
	case "max-commits":
		return c.setMaxCommits(v)
	}
	return fmt.Errorf("unknown history key %s", k)
}
func historyEnabled(v *yaml.Node, dst *bool) error {
	if v.Tag != "!!bool" || v.Kind != yaml.ScalarNode {
		return fmt.Errorf("history.enabled must be boolean")
	}
	return v.Decode(dst)
}
func (c *Config) setMaxCommits(v *yaml.Node) error {
	i, e := integer(v)
	if e != nil || i < 1 {
		return fmt.Errorf("history.max-commits must be positive integer")
	}
	c.MaxCommits = i
	return nil
}
func (c *Config) setExclude(v *yaml.Node) error {
	if v.Kind != yaml.SequenceNode {
		return fmt.Errorf("exclude must be a sequence")
	}
	c.Exclude = []string{}
	for _, x := range v.Content {
		if e := c.addExclude(x); e != nil {
			return e
		}
	}
	return nil
}
func (c *Config) addExclude(v *yaml.Node) error {
	s, e := str(v)
	if e != nil {
		return e
	}
	if !doublestar.ValidatePattern(s) {
		return fmt.Errorf("invalid exclusion glob %q", s)
	}
	c.Exclude = append(c.Exclude, s)
	return nil
}
func validateYAML(node *yaml.Node) error {
	if err := validateYAMLNode(node); err != nil {
		return err
	}
	for _, child := range node.Content {
		if err := validateYAML(child); err != nil {
			return err
		}
	}
	return nil
}
func validateYAMLNode(node *yaml.Node) error {
	if node.Kind == yaml.AliasNode || node.Tag == "!!null" || node.Tag == "!!merge" {
		return fmt.Errorf("nulls, aliases, anchors and merge keys are not allowed")
	}
	if node.Kind == yaml.MappingNode {
		return validateKeys(node)
	}
	return nil
}
func validateKeys(n *yaml.Node) error {
	seen := map[string]bool{}
	for i := 0; i < len(n.Content); i += 2 {
		if e := uniqueKey(n.Content[i], seen); e != nil {
			return e
		}
	}
	return nil
}
func uniqueKey(k *yaml.Node, seen map[string]bool) error {
	if k.Kind != yaml.ScalarNode || k.Tag != "!!str" {
		return fmt.Errorf("mapping keys must be strings")
	}
	if seen[k.Value] {
		return fmt.Errorf("duplicate key %s", k.Value)
	}
	seen[k.Value] = true
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
