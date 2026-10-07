package columbo

import (
	"fmt"
	"os"
	"path/filepath"
)

func (c *command) investigate(patterns []string) (Report, error) {
	a, err := load(c.invocation.Dir, patterns, c.config)
	if err != nil {
		return Report{}, err
	}
	choices, tangles := a.choiceSets(), a.tangles()
	c.choices, c.tangles = &choices, &tangles
	return a.Analyze()
}
func (c *command) publishChoices() error {
	if c.choices == nil {
		return nil
	}
	path := c.choiceSetPath()
	if err := writeEvidenceJSON(path, *c.choices); err != nil {
		return err
	}
	_, err := fmt.Fprintf(c.invocation.Stdout, "Choice-set evidence: %s (%d groups; no verdict)\n", path, len(c.choices.Groups))
	return err
}
func (c *command) choiceSetPath() string {
	if c.options.choiceSets == "" {
		return c.snapshotPath() + ".choices.json"
	}
	if filepath.IsAbs(c.options.choiceSets) {
		return c.options.choiceSets
	}
	return filepath.Join(c.invocation.Dir, c.options.choiceSets)
}
func writeEvidenceJSON(path string, report any) error {
	data, err := encodeCanonicalJSON(report)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	return finishChoiceFile(file, data)
}
func finishChoiceFile(file *os.File, data []byte) error {
	err := writeOutput(file, data)
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(file.Name())
	}
	return err
}
