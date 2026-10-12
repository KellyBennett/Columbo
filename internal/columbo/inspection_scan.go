package columbo

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type inspectionInput struct {
	root           string
	dir            string
	configRelative string
	configSHA      string
	config         Config
	files          []inspectionFile
}

func validateInspectionInput(dir, config string) (inspectionInput, error) {
	input, err := inspectionLocation(dir)
	if err != nil {
		return inspectionInput{}, err
	}
	return input.withConfiguration(config)
}

func inspectionLocation(dir string) (inspectionInput, error) {
	directory, err := inspectionRoot(dir)
	if err != nil {
		return inspectionInput{}, err
	}
	root, err := moduleRoot(directory)
	return inspectionInput{root: root, dir: directory}, err
}

func inspectionRoot(dir string) (string, error) {
	root, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	real, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	if real != root {
		return "", fmt.Errorf("symlink source root refused: %s", root)
	}
	return root, nil
}

func (input inspectionInput) withConfiguration(config string) (inspectionInput, error) {
	path := input.configurationPath(config)
	relative, err := inspectionConfigRelative(input.root, path)
	if err != nil {
		return inspectionInput{}, err
	}
	input.configRelative = filepath.ToSlash(relative)
	input.config, input.configSHA, err = loadInspectionConfig(path, config != "")
	return input, err
}

func (input inspectionInput) configurationPath(path string) string {
	if path == "" {
		path = ".columbo.yml"
	}
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(input.dir, path)
}

func inspectionConfigRelative(root, path string) (string, error) {
	relative, err := filepath.Rel(root, path)
	if err != nil || inspectionOutside(relative) {
		return "", fmt.Errorf("inspection configuration must be inside source root")
	}
	return relative, inspectionConfigFile(path)
}

func inspectionConfigFile(path string) error {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("inspection configuration must be a regular non-symlink file")
	}
	_, err = inspectionRoot(path)
	return err
}

func inspectionOutside(relative string) bool {
	return relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func (input *inspectionInput) capture() error {
	files, err := (&engine{root: input.root}).inspectionFiles()
	if err != nil {
		return err
	}
	input.files = files
	return input.verifyConfiguration()
}

func (input inspectionInput) verifyConfiguration() error {
	var hash string
	for _, file := range input.files {
		if file.Path == input.configRelative {
			hash = file.SHA
		}
	}
	if hash != input.configSHA {
		return fmt.Errorf("configuration changed during inspection: %s", input.configRelative)
	}
	return nil
}

func inspectionIdentity(files []inspectionFile) string {
	hash := sha256.New()
	for _, file := range files {
		fmt.Fprintf(hash, "%d:%s:%s\n", len(file.Path), file.Path, file.SHA)
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func (input inspectionInput) verifyFiles(files []inspectionFile) error {
	if inspectionIdentity(input.files) != inspectionIdentity(files) {
		return fmt.Errorf("source changed during inspection; rerun with stable source")
	}
	return nil
}

func (input inspectionInput) analyze(patterns []string) (Report, error) {
	input.config = input.config.clone()
	patterns = slices.Clone(patterns)
	if err := input.capture(); err != nil {
		return Report{}, err
	}
	if len(patterns) == 0 {
		patterns = []string{"./..."}
	}
	a, err := input.loadEngine(patterns)
	if err != nil {
		return Report{}, err
	}
	return input.finish(a, patterns)
}

func (input inspectionInput) loadEngine(patterns []string) (*engine, error) {
	a, err := load(input.dir, patterns, input.config)
	if err != nil {
		return nil, err
	}
	if a.root != input.root {
		return nil, fmt.Errorf("source module changed during inspection; rerun with stable source")
	}
	return a, nil
}

func (input inspectionInput) finish(a *engine, patterns []string) (Report, error) {
	report, err := a.Analyze()
	if err != nil {
		return Report{}, err
	}
	report.inspection, err = input.sourceFacts(a, patterns)
	if err != nil {
		return Report{}, err
	}
	report.includeInspectionDeclarations(a.declarations)
	return report, nil
}

func (input inspectionInput) sourceFacts(a *engine, patterns []string) (*inspectionFacts, error) {
	facts, err := a.inspectionFacts(patterns)
	if err != nil {
		return nil, err
	}
	if err := input.verifyFiles(facts.Files); err != nil {
		return nil, err
	}
	facts.ConfigFile, facts.ConfigSHA = input.configRelative, input.configSHA
	facts.SourceSHA = inspectionIdentity(facts.Files)
	return &facts, nil
}

func (report *Report) includeInspectionDeclarations(declarations []*declaration) {
	seen := map[DeclarationRef]bool{}
	for _, entry := range report.Declarations {
		seen[entry.Ref] = true
	}
	for _, d := range declarations {
		if !seen[d.ref()] {
			report.Declarations = append(report.Declarations, d.evidence())
		}
	}
}

func (c *command) analyze() int {
	input, err := c.inspectionInput()
	if err != nil {
		return c.fatal(err)
	}
	report, err := input.analyze(c.options.patterns)
	if err != nil {
		return c.fatal(err)
	}
	return c.publish(report)
}

func (c *command) inspectionInput() (inspectionInput, error) {
	input, err := validateInspectionInput(c.invocation.Dir, c.options.inspectionConfig())
	if err != nil {
		return inspectionInput{}, err
	}
	input.config.History = input.config.History && !c.options.noHistory
	input.config.Staged = c.options.staged
	return input, input.validateOutput(c.snapshotPath())
}

func (options commandOptions) inspectionConfig() string {
	if options.explicit {
		return options.config
	}
	return ""
}

func (input inspectionInput) validateOutput(path string) error {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	directory, err := filepath.EvalSymlinks(filepath.Dir(absolute))
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(input.root, filepath.Join(directory, filepath.Base(absolute)))
	if err != nil || !inspectionOutside(relative) {
		return fmt.Errorf("inspection output must be outside source root")
	}
	return snapshotDestinationAbsent(absolute)
}
