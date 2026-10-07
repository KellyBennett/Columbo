package columbo

import (
	"fmt"
	"path/filepath"
)

func (c *command) publishEvidence() error {
	if err := c.publishChoices(); err != nil {
		return err
	}
	return c.publishTangles()
}
func (c *command) publishTangles() error {
	if c.tangles == nil {
		return nil
	}
	path := c.tanglePath()
	if err := writeEvidenceJSON(path, *c.tangles); err != nil {
		return err
	}
	_, err := fmt.Fprintf(c.invocation.Stdout, "Nested field-decision evidence: %s (%d groups; no verdict)\n", path, len(c.tangles.Groups))
	return err
}

func (c *command) tanglePath() string {
	if c.options.tangles == "" {
		return c.snapshotPath() + ".tangles.json"
	}
	if filepath.IsAbs(c.options.tangles) {
		return c.options.tangles
	}
	return filepath.Join(c.invocation.Dir, c.options.tangles)
}
