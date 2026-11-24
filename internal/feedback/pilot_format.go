package feedback

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

func (o PilotOptions) unformattedFiles(ctx context.Context, dir, prefix string, changed []string) (*CommandResult, []string) {
	files := existingGoFiles(dir, changed)
	if len(files) == 0 {
		return nil, nil
	}
	command := runTrialCommand(ctx, dir, append([]string{"gofmt", "-l", "--"}, files...), "", prefix+"-gofmt", o.CheckTimeout, o.PassEnv...)
	return &command, listedFiles(command.Stdout)
}

func listedFiles(path string) []string {
	listed, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return slices.DeleteFunc(strings.Split(string(listed), "\n"), func(name string) bool { return name == "" })
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
