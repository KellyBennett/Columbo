package feedback

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// checkFormatting records which of the candidate's own changed Go files gofmt
// would rewrite. It is evidence for the report, not a verification gate.
func (a *RefactoringAttempt) checkFormatting(ctx context.Context, options PilotOptions, dir string, changed []string, prefix string) {
	files := existingGoFiles(dir, changed)
	if len(files) == 0 {
		return
	}
	command := runTrialCommand(ctx, dir, append([]string{"gofmt", "-l", "--"}, files...), "", prefix+"-gofmt", options.CheckTimeout, options.PassEnv...)
	a.Gofmt = &command
	listed, err := os.ReadFile(command.Stdout)
	if err != nil {
		return
	}
	a.Unformatted = slices.DeleteFunc(strings.Split(string(listed), "\n"), func(name string) bool { return name == "" })
}

func existingGoFiles(dir string, paths []string) []string {
	var files []string
	for _, path := range paths {
		if filepath.Ext(path) != ".go" {
			continue
		}
		if info, err := os.Lstat(filepath.Join(dir, path)); err == nil && info.Mode().IsRegular() {
			files = append(files, path)
		}
	}
	return files
}
