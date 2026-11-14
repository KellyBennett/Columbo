package columbo

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"

	"github.com/KellyBennett/Columbo/internal/snapshotdb"
)

const calibrateUsage = `Usage: columbo calibrate --export

Print the local case ledger (~/.columbo/ledger.sqlite) as aggregate JSON on stdout.
Columbo records the ledger only for runs with --ledger. The export holds case counts
per smell, verdict, suppression, and excess band. It drops every bucket with fewer
than 5 cases or 3 repositories, and holds no names, paths, hashes, or case IDs.
A Git repository counts once across its clones, worktrees, and modules. Outside
Git, each module path counts once, whatever its major version.
Share a single export. The floor protects each export on its own, not the
difference between two exports of the same ledger.
  --export            print the aggregate export
  --help              print usage
`

const ledgerExportFormat = "columbo-ledger-export"

var ledgerFloor = exportFloor{Cases: 5, Repos: 3}

type calibrateOptions struct{ export, help, shortHelp bool }

func (o *calibrateOptions) parse(args []string) error {
	fs := o.flagSet()
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("calibrate takes no arguments: %s", fs.Arg(0))
	}
	if !o.export && !o.help && !o.shortHelp {
		return fmt.Errorf("calibrate requires --export")
	}
	return nil
}

func (o *calibrateOptions) flagSet() *flag.FlagSet {
	fs := flag.NewFlagSet("calibrate", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.BoolVar(&o.export, "export", false, "")
	fs.BoolVar(&o.help, "help", false, "")
	fs.BoolVar(&o.shortHelp, "h", false, "")
	return fs
}

func (c *command) calibrate(args []string) int {
	var options calibrateOptions
	if err := options.parse(args); err != nil {
		return c.fatal(err)
	}
	if options.help || options.shortHelp {
		return c.deliver([]byte(calibrateUsage))
	}
	document, err := c.invocation.ledgerExport()
	if err != nil {
		return c.fatal(err)
	}
	return c.deliver(document)
}

func (i Invocation) ledgerExport() ([]byte, error) {
	path, err := i.ledgerPath()
	if err != nil {
		return nil, err
	}
	if err = requireLedger(path); err != nil {
		return nil, err
	}
	buckets, err := ledgerBuckets(path)
	if err != nil {
		return nil, err
	}
	document, err := json.MarshalIndent(newLedgerExport(buckets), "", "  ")
	return append(document, '\n'), err
}

func requireLedger(path string) error {
	exists, err := ledgerExists(path)
	if err == nil && !exists {
		err = fmt.Errorf("no ledger at %s; record one with columbo --ledger", path)
	}
	return err
}

func ledgerBuckets(path string) ([]snapshotdb.LedgerBucket, error) {
	ledger, err := snapshotdb.OpenLedger(path)
	if err != nil {
		return nil, err
	}
	defer ledger.Close()
	return ledger.Queries().LedgerBuckets(context.Background(), snapshotdb.LedgerBucketsParams{
		MinCases: ledgerFloor.Cases, MinRepos: ledgerFloor.Repos,
	})
}

type ledgerExport struct {
	Format  string       `json:"format"`
	Version int          `json:"version"`
	Floor   exportFloor  `json:"floor"`
	Excess  []exportBand `json:"excess"`
	Rules   []exportRule `json:"rules"`
}
type exportFloor struct {
	Cases int64 `json:"cases"`
	Repos int64 `json:"repos"`
}
type exportBand struct {
	Name     string   `json:"name"`
	MaxRatio *float64 `json:"max_ratio,omitempty"`
}
type exportRule struct {
	Smell   string         `json:"smell"`
	Buckets []exportBucket `json:"buckets"`
}
type exportBucket struct {
	Verdict    string `json:"verdict"`
	Suppressed bool   `json:"suppressed"`
	Excess     string `json:"excess"`
	Cases      int64  `json:"cases"`
	Repos      int64  `json:"repos"`
}

func newLedgerExport(buckets []snapshotdb.LedgerBucket) ledgerExport {
	export := ledgerExport{Format: ledgerExportFormat, Version: 1, Floor: ledgerFloor, Rules: []exportRule{}}
	for _, band := range excessBands {
		export.Excess = append(export.Excess, band.export())
	}
	for _, bucket := range buckets {
		export.add(bucket.Smell, newExportBucket(bucket))
	}
	return export
}

func (band excessBand) export() exportBand {
	if math.IsInf(band.maxRatio, 1) {
		return exportBand{Name: band.name}
	}
	return exportBand{Name: band.name, MaxRatio: &band.maxRatio}
}

func newExportBucket(bucket snapshotdb.LedgerBucket) exportBucket {
	return exportBucket{Verdict: bucket.Verdict, Suppressed: bucket.Suppressed == 1,
		Excess: excessName(bucket.Excess), Cases: bucket.Cases, Repos: bucket.Repos}
}

func (e *ledgerExport) add(smell string, bucket exportBucket) {
	if last := len(e.Rules) - 1; last >= 0 && e.Rules[last].Smell == smell {
		e.Rules[last].Buckets = append(e.Rules[last].Buckets, bucket)
		return
	}
	e.Rules = append(e.Rules, exportRule{Smell: smell, Buckets: []exportBucket{bucket}})
}
