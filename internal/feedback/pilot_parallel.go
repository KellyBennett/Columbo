package feedback

import (
	"context"
	"fmt"
	"io"
	"sync"
)

type pilotRunner struct {
	options  PilotOptions
	cases    []PilotCase
	mu       sync.Mutex
	out      io.Writer
	failures int
}

func runPilotCases(ctx context.Context, options PilotOptions, cases []PilotCase, out io.Writer) int {
	runner := &pilotRunner{options: options, cases: cases, out: out}
	indices := make(chan int)
	var workers sync.WaitGroup
	for range max(options.Parallel, 1) {
		workers.Go(func() { runner.work(ctx, indices) })
	}
	dispatch(ctx, len(cases), indices)
	workers.Wait()
	return runner.failures
}

func dispatch(ctx context.Context, count int, indices chan<- int) {
	defer close(indices)
	for i := 0; i < count && ctx.Err() == nil; i++ {
		select {
		case indices <- i:
		case <-ctx.Done():
		}
	}
}

func (r *pilotRunner) work(ctx context.Context, indices <-chan int) {
	for i := range indices {
		label := r.cases[i].progressLabel(i, len(r.cases))
		r.println(label + ": started")
		r.finish(label, runPilotCase(ctx, r.options, r.cases[i]))
	}
}

func (r *pilotRunner) println(line string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	fmt.Fprintln(r.out, line)
}

func (r *pilotRunner) finish(label string, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err != nil {
		r.failures++
		fmt.Fprintf(r.out, "%s: incomplete: %v\n", label, err)
		return
	}
	fmt.Fprintln(r.out, label+": done")
}

func (c PilotCase) progressLabel(i, total int) string {
	return fmt.Sprintf("[%d/%d] PR %d %s (%s)", i+1, total, c.PR, c.CaseID, c.Reaction)
}
