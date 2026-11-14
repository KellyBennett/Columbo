package report

import (
	"bytes"
	"cmp"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"os"
	"slices"
	"strconv"
	"strings"
)

const (
	exportFormat = "columbo-ledger-export"
	unmeasured   = "unmeasured"
	allOperators = "(all)"
)

// Weights is a parsed `columbo calibrate --export` document; only LoadWeights makes one.
type Weights struct {
	floor floor
	// bands are ordered, and only the last has no upper bound.
	bands []excessBand
	// cases counts each smell's real cases per band name, summed over verdict and suppressed.
	cases map[string]map[string]int
}

type floor struct {
	Cases int `json:"cases"`
	Repos int `json:"repos"`
}

// MaxRatio is inclusive.
type excessBand struct {
	Name     string   `json:"name"`
	MaxRatio *float64 `json:"max_ratio"`
}

type exportDocument struct {
	Format  string       `json:"format"`
	Version int          `json:"version"`
	Floor   floor        `json:"floor"`
	Excess  []excessBand `json:"excess"`
	Rules   []struct {
		Smell   string `json:"smell"`
		Buckets []struct {
			Verdict    string `json:"verdict"`
			Suppressed bool   `json:"suppressed"`
			Excess     string `json:"excess"`
			Cases      int    `json:"cases"`
			Repos      int    `json:"repos"`
		} `json:"buckets"`
	} `json:"rules"`
}

func LoadWeights(path string) (*Weights, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	w, err := parseWeights(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return w, nil
}

func parseWeights(data []byte) (*Weights, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var doc exportDocument
	if err := dec.Decode(&doc); err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, fmt.Errorf("the export has content after its JSON document")
	}
	if doc.Format != exportFormat {
		return nil, fmt.Errorf("format is %q, want %q", doc.Format, exportFormat)
	}
	if doc.Version != 1 {
		return nil, fmt.Errorf("export version %d is not supported, only version 1", doc.Version)
	}
	if doc.Floor.Cases < 0 || doc.Floor.Repos < 0 {
		return nil, fmt.Errorf("the floor has a negative count")
	}
	if err := checkBands(doc.Excess); err != nil {
		return nil, err
	}
	w := &Weights{floor: doc.Floor, bands: doc.Excess, cases: map[string]map[string]int{}}
	columns := w.columns()
	for _, r := range doc.Rules {
		if w.cases[r.Smell] == nil {
			w.cases[r.Smell] = map[string]int{}
		}
		for _, b := range r.Buckets {
			if !slices.Contains(columns, b.Excess) {
				return nil, fmt.Errorf("%s: bucket excess %q is not a listed band or %s", r.Smell, b.Excess, unmeasured)
			}
			if b.Cases < 0 || b.Repos < 0 {
				return nil, fmt.Errorf("%s: a %s bucket has a negative count", r.Smell, b.Excess)
			}
			w.cases[r.Smell][b.Excess] += b.Cases
		}
	}
	return w, nil
}

func checkBands(bands []excessBand) error {
	if len(bands) == 0 {
		return fmt.Errorf("the export lists no excess bands")
	}
	seen := map[string]bool{unmeasured: true}
	for i, b := range bands {
		if b.Name == "" || seen[b.Name] {
			return fmt.Errorf("excess band %q is empty, repeated, or reserved", b.Name)
		}
		seen[b.Name] = true
		last := i == len(bands)-1
		if (b.MaxRatio == nil) != last {
			return fmt.Errorf("excess band %q: every band but the last needs max_ratio, and the last has none", b.Name)
		}
		if i > 0 && !last && *b.MaxRatio <= *bands[i-1].MaxRatio {
			return fmt.Errorf("excess band %q: max_ratio must exceed the band before it", b.Name)
		}
	}
	return nil
}

func (w *Weights) columns() []string {
	names := make([]string, 0, len(w.bands)+1)
	for _, b := range w.bands {
		names = append(names, b.Name)
	}
	return append(names, unmeasured)
}

func (w *Weights) bandOf(ratio sql.NullFloat64) string {
	if !ratio.Valid {
		return unmeasured
	}
	for _, b := range w.bands {
		if b.MaxRatio == nil || ratio.Float64 <= *b.MaxRatio {
			return b.Name
		}
	}
	return unmeasured
}

// subjectQuery gives every scored mutant its subject case's ratio; shaped is false when it has none.
// %[1]s and %[2]s are the mutant and baseline ratio columns, or NULL in a database recorded before them.
const subjectQuery = `WITH ` + latestRuns + `,
scored AS (SELECT * FROM cur WHERE class IN ('inject','evade','cure') AND status IN ('killed','survived','conflict')),
subjects AS (
 SELECT m.id, %[1]s AS ratio FROM scored m JOIN mutant_findings f ON f.mutant_id = m.id AND f.side = 'added' AND f.smell = m.target
 WHERE m.class = 'inject'
 UNION ALL
 SELECT m.id, %[1]s FROM scored m JOIN mutant_findings f ON f.mutant_id = m.parent_id AND f.side = 'added' AND f.smell = m.target
 WHERE m.class = 'evade'
 UNION ALL
 SELECT m.id, %[2]s FROM scored m JOIN baseline_findings b ON b.run_id = m.run_id AND b.smell = m.target AND b.symbol = m.site_symbol
 WHERE m.class = 'cure')
SELECT m.class, m.target, m.operator, m.status = 'killed', s.id IS NOT NULL, s.ratio
FROM scored m LEFT JOIN (SELECT id, max(ratio) AS ratio FROM subjects GROUP BY id) s ON s.id = m.id`

// ratioColumn lets the report read a database that predates finding ratios without altering it.
func ratioColumn(db *sql.DB, table, column string) (string, error) {
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM pragma_table_info(?) WHERE name = 'ratio'`, table).Scan(&n); err != nil || n == 0 {
		return "NULL", err
	}
	return column, nil
}

type tally struct{ killed, scored int }

type rateGroup struct {
	class, target, operator string
	unshaped                int
	bands                   map[string]tally
}

func readGroups(db *sql.DB, weights *Weights) ([]*rateGroup, error) {
	mutantRatio, err := ratioColumn(db, "mutant_findings", "f.ratio")
	if err != nil {
		return nil, err
	}
	baselineRatio, err := ratioColumn(db, "baseline_findings", "b.ratio")
	if err != nil {
		return nil, err
	}
	rows, err := db.Query(fmt.Sprintf(subjectQuery, mutantRatio, baselineRatio))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	groups := map[[3]string]*rateGroup{}
	for rows.Next() {
		var class, target, operator string
		var killed, shaped bool
		var ratio sql.NullFloat64
		if err := rows.Scan(&class, &target, &operator, &killed, &shaped, &ratio); err != nil {
			return nil, err
		}
		for _, op := range []string{operator, allOperators} {
			key := [3]string{class, target, op}
			g := groups[key]
			if g == nil {
				g = &rateGroup{class: class, target: target, operator: op, bands: map[string]tally{}}
				groups[key] = g
			}
			if !shaped {
				g.unshaped++
				continue
			}
			b := weights.bandOf(ratio)
			t := g.bands[b]
			t.scored++
			if killed {
				t.killed++
			}
			g.bands[b] = t
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	pooledFirst := func(g *rateGroup) int {
		if g.operator == allOperators {
			return 0
		}
		return 1
	}
	return slices.SortedFunc(maps.Values(groups), func(a, b *rateGroup) int {
		return cmp.Or(cmp.Compare(a.class, b.class), cmp.Compare(a.target, b.target),
			cmp.Compare(pooledFirst(a), pooledFirst(b)), cmp.Compare(a.operator, b.operator))
	}), nil
}

func (g *rateGroup) cells(weights *Weights) []string {
	cases := weights.cases[g.target]
	total := sum(cases)
	shaped, killed := 0, 0
	var weighted, covered float64
	var bands []string
	for _, name := range weights.columns() {
		t := g.bands[name]
		shaped += t.scored
		killed += t.killed
		share := 0.0
		if total > 0 {
			share = float64(cases[name]) / float64(total)
		}
		switch {
		case t.scored > 0:
			weighted += share * float64(t.killed) / float64(t.scored)
			covered += share
			bands = append(bands, fmt.Sprintf("%d/%d", t.killed, t.scored))
		case share > 0:
			bands = append(bands, "uncovered")
		default:
			bands = append(bands, "")
		}
	}
	unweightedCell, weightedCell, coveredCell := "", "no real cases", ""
	if shaped > 0 {
		unweightedCell = percent(float64(killed) / float64(shaped))
	}
	if total > 0 {
		weightedCell, coveredCell = "", percent(covered)
		if covered > 0 {
			weightedCell = percent(weighted / covered)
		}
	}
	return append([]string{g.class, g.operator, g.target, strconv.Itoa(shaped), strconv.Itoa(g.unshaped),
		unweightedCell, weightedCell, coveredCell}, bands...)
}

func sum(counts map[string]int) int {
	total := 0
	for _, n := range counts {
		total += n
	}
	return total
}

func percent(share float64) string { return fmt.Sprintf("%.0f%%", 100*share) }

// WriteWeighted renders the export's real case shapes and each operator's kill rate weighted by them.
func WriteWeighted(w io.Writer, db *sql.DB, weights *Weights) error {
	columns := weights.columns()
	var shapes [][]string
	for _, smell := range slices.Sorted(maps.Keys(weights.cases)) {
		cases := weights.cases[smell]
		total := sum(cases)
		row := []string{smell, strconv.Itoa(total)}
		for _, name := range columns {
			cell := ""
			if n := cases[name]; n > 0 {
				cell = fmt.Sprintf("%s (%d)", percent(float64(n)/float64(total)), n)
			}
			row = append(row, cell)
		}
		shapes = append(shapes, row)
	}
	writeTable(w, "Real case shapes",
		fmt.Sprintf("Each band cell is that band's share of the smell's exported cases, then its count, from buckets with at least %d cases in at least %d repositories.", weights.floor.Cases, weights.floor.Repos),
		append([]string{"smell", "cases"}, columns...), shapes)

	groups, err := readGroups(db, weights)
	if err != nil {
		return fmt.Errorf("weighted kill rate: %w", err)
	}
	var rates [][]string
	for _, g := range groups {
		rates = append(rates, g.cells(weights))
	}
	writeTable(w, "Weighted kill rate",
		"Each row counts scored inject, evade, and cure mutants by the excess band of their subject case. "+
			"Unweighted is killed over scored. "+
			"Weighted averages each band's kill rate by that band's share of the target smell's real cases, over the bands that have scored mutants. "+
			"Covered is the share of real cases those bands hold. "+
			"A band cell shows killed over scored, and uncovered marks a band that holds real cases but no scored mutant. "+
			"An unshaped mutant has no subject case, such as an inject that added nothing, and counts in neither rate.",
		append([]string{"class", "operator", "target", "shaped", "unshaped", "unweighted", "weighted", "covered"}, columns...), rates)
	return nil
}

func writeTable(w io.Writer, title, note string, cols []string, rows [][]string) {
	fmt.Fprintf(w, "\n## %s\n\n%s\n\n", title, note)
	if len(rows) == 0 {
		fmt.Fprintln(w, "None.")
		return
	}
	fmt.Fprintln(w, "| "+strings.Join(cols, " | ")+" |")
	fmt.Fprintln(w, "|"+strings.Repeat("---|", len(cols)))
	for _, r := range rows {
		cells := make([]string, len(r))
		for i, c := range r {
			cells[i] = escape(c)
		}
		fmt.Fprintln(w, "| "+strings.Join(cells, " | ")+" |")
	}
}
