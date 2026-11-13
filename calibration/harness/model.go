package harness

import (
	"golang.org/x/tools/go/packages"
)

type Class string

const (
	Inject  Class = "inject"
	Evade   Class = "evade"
	Neutral Class = "neutral"
	Cure    Class = "cure"
)

type Status string

const (
	Killed     Status = "killed"
	Survived   Status = "survived"
	Invalid    Status = "invalid"
	Equivalent Status = "equivalent"
	Conflict   Status = "conflict"
	IDDrift    Status = "id-drift"
)

type Site struct {
	Package string
	Symbol  string
	Offset  int
	File    string
}

// Edit maps module-relative slash paths to their new contents. A nil value deletes the file.
type Edit map[string][]byte

// Prediction is the operator's own expectation, computed from metrics it can see in the
// mutated source. Kill is nil when the operator cannot say.
type Prediction struct {
	Kill   *bool
	Reason string
}

// Expectation tells the scorer which findings count for this mutant.
type Expectation struct {
	// Touched are the declaration symbols a finding may anchor to and still count.
	Touched []string
	// Catches are the smells that count as Columbo noticing an evade; empty means the target.
	Catches []string
	// Renames maps an original declaration symbol to its symbol in the mutant (neutral only).
	Renames map[string]string
	// Moves maps an original declaration symbol to its new module-relative file (neutral only).
	Moves map[string]string
	// FromBaseline marks an evade of a case already in the baseline, such as a red-team replay.
	FromBaseline bool
	// AnyAdded makes any smell newly added on a touched symbol count as noticing an evade.
	AnyAdded bool
	// Target overrides the operator's target for one mutant, for operators that replay many smells.
	Target string
	Prediction
}

type Applied struct {
	Edit Edit
	Expectation
}

type Corpus struct {
	Root     string
	Module   string
	Version  string
	Packages []*packages.Package
	Baseline Snapshot
}

type Operator interface {
	ID() string
	Class() Class
	Target() string
	Sites(c *Corpus) []Site
	Doses(c *Corpus, site Site) []int
	Apply(c *Corpus, site Site, dose int) (Applied, error)
}

// Evader is an evade operator that only runs on top of a killed inject mutant.
type Evader interface {
	Operator
	Follows() string
}

type Step struct {
	Operator string `json:"operator"`
	Site     Site   `json:"site"`
	Dose     int    `json:"dose"`
}

type Mutant struct {
	Chain []Step
	Patch []byte
	Size  int
}

type Finding struct {
	Smell   string
	Symbol  string
	File    string
	CaseID  string
	Verdict string
}

type Outcome struct {
	Mutant  Mutant
	Class   Class
	Target  string
	Added   []Finding
	Removed []Finding
	Status  Status
	Reason  string
	Predict Prediction
}
