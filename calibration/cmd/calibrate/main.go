package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/KellyBennett/Columbo/calibration/harness"
	"github.com/KellyBennett/Columbo/calibration/operators"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "calibrate:", err)
		os.Exit(2)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: calibrate run|report [flags]")
	}
	if err := resolveTempDir(); err != nil {
		return err
	}
	switch args[0] {
	case "run":
		return runCmd(args[1:])
	case "report":
		return reportCmd(args[1:])
	}
	return fmt.Errorf("unknown command %q", args[0])
}

// resolveTempDir points TMPDIR at its symlink-free path, because columbo refuses a
// symlinked source root and macOS keeps temp directories under the /var symlink.
func resolveTempDir() error {
	dir, err := filepath.EvalSymlinks(os.TempDir())
	if err != nil {
		return err
	}
	return os.Setenv("TMPDIR", dir)
}

func runCmd(args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	corpus := fs.String("corpus", ".", "module root to mutate")
	module := fs.String("module", "", "corpus module path")
	version := fs.String("version", "", "corpus version (tag, pseudo-version, or commit)")
	columbo := fs.String("columbo", "columbo", "columbo binary")
	db := fs.String("db", "calibration.sqlite", "results database")
	ops := fs.String("operators", "noop", "comma-separated operator IDs, or all")
	validity := fs.String("validity", "test", "test or build")
	workers := fs.Int("workers", 4, "parallel mutants")
	maxSites := fs.Int("max-sites", 0, "deterministic per-operator site cap; 0 means all")
	if err := fs.Parse(args); err != nil {
		return err
	}
	bin, err := filepath.Abs(*columbo)
	if err != nil {
		return err
	}
	src, err := filepath.Abs(*corpus)
	if err != nil {
		return err
	}
	selected, err := selectOperators(*ops)
	if err != nil {
		return err
	}
	store, err := harness.OpenStore(*db)
	if err != nil {
		return err
	}
	defer store.Close()
	r := &harness.Runner{
		Env:      harness.Env{Columbo: bin, Validity: harness.Validity(*validity), Timeout: 15 * time.Minute},
		Store:    store,
		Info:     harness.RunInfo{Module: *module, Version: *version, Validity: harness.Validity(*validity)},
		Source:   src,
		Ops:      selected,
		Workers:  *workers,
		MaxSites: *maxSites,
		Log:      func(f string, a ...any) { fmt.Fprintf(os.Stderr, f+"\n", a...) },
	}
	if err := r.Prepare(); err != nil {
		return err
	}
	defer r.Cleanup()
	return r.Run()
}

func selectOperators(spec string) ([]harness.Operator, error) {
	if spec == "all" {
		return operators.All(), nil
	}
	return operators.Select(strings.Split(spec, ","))
}

func reportCmd(args []string) error {
	return fmt.Errorf("report: not built yet")
}
