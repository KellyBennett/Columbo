package columbo

import (
	"context"
	"crypto/rand"
	"flag"
	"fmt"
	"io"
	"path/filepath"

	"github.com/KellyBennett/Columbo/internal/snapshotdb"
)

const usage = `Usage: columbo [flags] [packages...]

Investigate Go code smells. Packages default to ./...; flags precede packages.
  --config PATH       configuration (default .columbo.yml)
  --output PATH       fresh SQLite snapshot (default columbo-<random>.sqlite)
  --choice-sets-output PATH  experimental choice-set evidence JSON (opt-in)
  --no-history        disable optional Git provenance
  --version           print build version
  --help              print usage
`

type Invocation struct {
	Dir, Version   string
	Stdout, Stderr io.Writer
}
type commandOptions struct {
	config, output, choiceSets                        string
	noHistory, showVersion, help, shortHelp, explicit bool
	patterns                                          []string
}
type command struct {
	invocation Invocation
	options    commandOptions
	config     Config
	choices    *ChoiceSetReport
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
	return []byte("columbo " + i.buildVersion() + "\n")
}
func (i Invocation) buildVersion() string {
	if i.Version == "" {
		return "dev"
	}
	return i.Version
}
func (o *commandOptions) parse(args []string) error {
	fs := o.flagSet()
	if e := fs.Parse(args); e != nil {
		return e
	}
	o.arguments(fs)
	if o.choiceSets == "-" {
		return fmt.Errorf("choice-sets-output must name a fresh JSON file, not stdout")
	}
	if o.output == "" || o.output == "-" {
		return fmt.Errorf("output must name a SQLite file, not stdout")
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
	fs.StringVar(&o.output, "output", "columbo-"+rand.Text()+".sqlite", "")
	fs.StringVar(&o.choiceSets, "choice-sets-output", "", "")
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
	r, e := c.investigate(patterns)
	if e != nil {
		return c.fatal(e)
	}
	return c.publish(r)
}
func (c *command) snapshotPath() string {
	if filepath.IsAbs(c.options.output) {
		return c.options.output
	}
	return filepath.Join(c.invocation.Dir, c.options.output)
}
func (c *command) publish(report Report) int {
	path := c.snapshotPath()
	if e := WriteSnapshot(path, report, c.invocation.buildVersion()); e != nil {
		return c.fatal(e)
	}
	if e := c.publishChoices(); e != nil {
		return c.fatal(e)
	}
	snapshot, e := OpenSnapshot(path)
	if e != nil {
		return c.fatal(e)
	}
	defer snapshot.Close()
	return c.output(snapshot.Queries(), path)
}
func (c *command) output(queries snapshotdb.Querier, path string) int {
	b, code, e := RenderSnapshot(queries, path)
	if e != nil {
		return c.fatal(e)
	}
	if e := c.warnings(queries); e != nil {
		return c.fatal(e)
	}
	return c.deliverSummary(b, code)
}
func (c *command) deliverSummary(b []byte, code int) int {
	if e := writeOutput(c.invocation.Stdout, b); e != nil {
		return c.fatal(fmt.Errorf("output write failed: %w", e))
	}
	return code
}
func (c *command) warnings(queries snapshotdb.Querier) error {
	messages, e := queries.WarningMessages(context.Background())
	if e != nil {
		return e
	}
	for _, message := range messages {
		c.invocation.warning(message)
	}
	return nil
}

func (o *commandOptions) flagSet() *flag.FlagSet {
	fs := flag.NewFlagSet("columbo", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	o.flags(fs)
	return fs
}

func (i Invocation) warning(message string) { fmt.Fprintln(i.Stderr, message) }
