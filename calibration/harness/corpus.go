package harness

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// PrepareCorpus copies source into a fresh base directory and records Columbo's view of the unmutated module.
// The caller removes c.Root when done.
func (e Env) PrepareCorpus(source string, config []byte, module, version string) (*Corpus, error) {
	base, err := TempDir("calibration-base-")
	if err != nil {
		return nil, err
	}
	c, err := e.prepare(base, source, config)
	if err != nil {
		_ = os.RemoveAll(base)
		return nil, err
	}
	c.Module, c.Version = module, version
	return c, nil
}

func (e Env) prepare(base, source string, config []byte) (*Corpus, error) {
	if err := CopyModule(source, base); err != nil {
		return nil, err
	}
	if config != nil {
		if err := os.WriteFile(filepath.Join(base, ".columbo.yml"), config, 0o644); err != nil {
			return nil, err
		}
	}
	if why := e.Check(base, nil); why != "" {
		return nil, fmt.Errorf("unmutated corpus is not valid: %s", why)
	}
	snap, err := e.Analyze(base)
	if err != nil {
		return nil, err
	}
	pkgs, err := LoadPackages(base)
	if err != nil {
		return nil, err
	}
	return &Corpus{Root: base, Packages: pkgs, Baseline: snap}, nil
}

// Probe is Columbo's view of a copy of the corpus with one edit applied.
// Invalid says why the copy is not a valid mutant; Snapshot is empty then.
type Probe struct {
	Dir      string
	Snapshot Snapshot
	Invalid  string
}

// Probe leaves Dir for the caller to remove, so a killed mutant can be reloaded or handed on.
func (e Env) Probe(base string, edit Edit) Probe {
	dir, err := TempDir("calibration-mutant-")
	if err != nil {
		return Probe{Invalid: err.Error()}
	}
	p := Probe{Dir: dir}
	if err := CopyModule(base, dir); err != nil {
		p.Invalid = err.Error()
		return p
	}
	if err := WriteEdit(dir, edit); err != nil {
		p.Invalid = err.Error()
		return p
	}
	if why := e.Check(dir, editedPackages(edit)); why != "" {
		p.Invalid = why
		return p
	}
	if p.Snapshot, err = e.Analyze(dir); err != nil {
		p.Invalid = err.Error()
	}
	return p
}

// ForEach calls f for every index below n on at most workers goroutines.
func ForEach(n, workers int, f func(i int)) {
	if workers <= 0 {
		workers = 1
	}
	var wg sync.WaitGroup
	ch := make(chan int)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range ch {
				f(i)
			}
		}()
	}
	for i := 0; i < n; i++ {
		ch <- i
	}
	close(ch)
	wg.Wait()
}
