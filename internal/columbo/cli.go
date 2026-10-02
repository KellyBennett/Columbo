package columbo

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"path/filepath"
)

const usage = `Usage: columbo [flags] [packages...]

Investigate Go code smells. Packages default to ./...; flags precede packages.
  --config PATH       configuration (default .columbo.yml)
  --format text|json  report format (default text)
  --no-history        disable optional Git provenance
  --version           print build version
  --help              print usage
`

func Run(args []string, dir, version string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("columbo", flag.ContinueOnError)
	var flagErrors bytes.Buffer
	fs.SetOutput(&flagErrors)
	config := fs.String("config", ".columbo.yml", "")
	format := fs.String("format", "text", "")
	noHistory := fs.Bool("no-history", false, "")
	showVersion := fs.Bool("version", false, "")
	help := fs.Bool("help", false, "")
	shortHelp := fs.Bool("h", false, "")
	fatal := func(err error) int { fmt.Fprintf(stderr, "columbo: %v\n", err); return 2 }
	deliver := func(b []byte) error {
		n, e := stdout.Write(b)
		if e != nil {
			return e
		}
		if n != len(b) {
			return io.ErrShortWrite
		}
		return nil
	}
	if e := fs.Parse(args); e != nil {
		return fatal(e)
	}
	if *format != "text" && *format != "json" {
		return fatal(fmt.Errorf("invalid format %q", *format))
	}
	if *help || *shortHelp {
		if e := deliver([]byte(usage)); e != nil {
			return fatal(e)
		}
		return 0
	}
	if *showVersion {
		if version == "" {
			version = "dev"
		}
		if e := deliver([]byte("columbo " + version + "\n")); e != nil {
			return fatal(e)
		}
		return 0
	}
	explicit := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "config" {
			explicit = true
		}
	})
	path := *config
	if !filepath.IsAbs(path) {
		path = filepath.Join(dir, path)
	}
	c, e := LoadConfig(path, explicit)
	if e != nil {
		return fatal(e)
	}
	if *noHistory {
		c.History = false
	}
	patterns := fs.Args()
	if len(patterns) == 0 {
		patterns = []string{"./..."}
	}
	r, e := Analyze(dir, patterns, c)
	if e != nil {
		return fatal(e)
	}
	b, e := Serialize(r, *format)
	if e != nil {
		return fatal(e)
	}
	for _, w := range r.Warnings {
		if *format == "text" || w.Code == "history-unavailable" {
			fmt.Fprintln(stderr, w.Message)
		}
	}
	if e = deliver(b); e != nil {
		return fatal(fmt.Errorf("output write failed: %w", e))
	}
	if r.Summary.Failed > 0 {
		return 1
	}
	return 0
}
