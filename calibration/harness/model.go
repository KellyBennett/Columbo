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

// Edit maps module-relative slash paths to new contents; a nil value deletes the file.
type Edit map[string][]byte

// Prediction is the operator's own expectation; Kill is nil when the operator cannot say.
type Prediction struct {
	Kill   *bool
	Reason string
}

type Expectation struct {
	Touched []string
	// Catches empty means the operator's target.
	Catches []string
	Renames map[string]string
	Moves   map[string]string
	// Files scopes a baseline evade to its edited files, since file-wide cases in one package share a package-clause symbol.
	Files        []string
	FromBaseline bool
	// AnyAdded counts any smell newly added on a touched symbol as noticing an evade.
	AnyAdded bool
	Target   string
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
	Support []Decl
}

type Decl struct {
	File   string
	Symbol string
	Offset int
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
