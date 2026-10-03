package columbo

import (
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

type Invocation struct {
	Dir, Version   string
	Stdout, Stderr io.Writer
}
type commandOptions struct {
	config, format                                    string
	noHistory, showVersion, help, shortHelp, explicit bool
	patterns                                          []string
}
type command struct {
	invocation Invocation
	options    commandOptions
	config     Config
	report     Report
}

func Run(args []string, invocation Invocation) int {
	c := &command{invocation: invocation}
	if e := c.options.parse(args); e != nil {
		return c.fatal(e)
	}
	if c.options.help || c.options.shortHelp {
		return c.deliver([]byte(usage))
	}
	if c.options.showVersion {
		return c.deliver(c.invocation.versionText())
	}
	return c.analyze()
}
func (i Invocation) versionText() []byte {
	version := i.Version
	if version == "" {
		version = "dev"
	}
	return []byte("columbo " + version + "\n")
}
func (o *commandOptions) parse(args []string) error {
	fs := o.flagSet()
	if e := fs.Parse(args); e != nil {
		return e
	}
	o.arguments(fs)
	if o.format != "text" && o.format != "json" {
		return fmt.Errorf("invalid format %q", o.format)
	}
	return nil
}
func (o *commandOptions) arguments(fs *flag.FlagSet) {
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "config" {
			o.explicit = true
		}
	})
	o.patterns = fs.Args()
}
func (o *commandOptions) flags(fs *flag.FlagSet) {
	fs.StringVar(&o.config, "config", ".columbo.yml", "")
	fs.StringVar(&o.format, "format", "text", "")
	fs.BoolVar(&o.noHistory, "no-history", false, "")
	fs.BoolVar(&o.showVersion, "version", false, "")
	fs.BoolVar(&o.help, "help", false, "")
	fs.BoolVar(&o.shortHelp, "h", false, "")
}
func (c *command) fatal(err error) int {
	fmt.Fprintf(c.invocation.Stderr, "columbo: %v\n", err)
	return 2
}
func (c *command) deliver(b []byte) int {
	if e := writeOutput(c.invocation.Stdout, b); e != nil {
		return c.fatal(e)
	}
	return 0
}
func writeOutput(w io.Writer, b []byte) error {
	n, e := w.Write(b)
	if e != nil {
		return e
	}
	if n != len(b) {
		return io.ErrShortWrite
	}
	return nil
}
func (c *command) loadConfig() error {
	path := c.options.config
	if !filepath.IsAbs(path) {
		path = filepath.Join(c.invocation.Dir, path)
	}
	cfg, e := LoadConfig(path, c.options.explicit)
	if e != nil {
		return e
	}
	if c.options.noHistory {
		cfg.History = false
	}
	c.config = cfg
	return nil
}
func (c *command) analyze() int {
	if e := c.loadConfig(); e != nil {
		return c.fatal(e)
	}
	patterns := c.options.patterns
	if len(patterns) == 0 {
		patterns = []string{"./..."}
	}
	r, e := Analyze(c.invocation.Dir, patterns, c.config)
	if e != nil {
		return c.fatal(e)
	}
	c.report = r
	return c.output()
}
func (c *command) output() int {
	b, e := Serialize(c.report, c.options.format)
	if e != nil {
		return c.fatal(e)
	}
	c.warnings()
	if e = writeOutput(c.invocation.Stdout, b); e != nil {
		return c.fatal(fmt.Errorf("output write failed: %w", e))
	}
	if c.report.Summary.Failed > 0 {
		return 1
	}
	return 0
}
func (c *command) warnings() {
	for _, w := range c.report.Warnings {
		if c.options.format == "text" || w.Code == "history-unavailable" {
			c.invocation.warning(w.Message)
		}
	}
}

func (o *commandOptions) flagSet() *flag.FlagSet {
	fs := flag.NewFlagSet("columbo", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	o.flags(fs)
	return fs
}

func (i Invocation) warning(message string) { fmt.Fprintln(i.Stderr, message) }
