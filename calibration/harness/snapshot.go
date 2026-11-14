package harness

import (
	"database/sql"
	"fmt"
	"sort"

	_ "modernc.org/sqlite"
)

type Snapshot struct {
	Findings           []Finding
	Thresholds         map[string]float64
	Version            string
	Dependencies       map[string][]string
	DuplicateFragments map[string]bool
	// Parameters and Guidance are keyed by case ID.
	Parameters map[string][]ParameterReceipt
	Guidance   map[string]Guidance
	// Suppressed counts cases a directive hid; Findings leaves them out.
	Suppressed int
	// Output is Columbo's terminal summary for the analysis.
	Output string
}

// Guidance is what Columbo tells the next agent about a case.
type Guidance struct {
	Why, Diagnosis string
	Leads, Avoid   []string
}

func ReadSnapshot(path string) (Snapshot, error) {
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		return Snapshot{}, err
	}
	defer db.Close()
	s := Snapshot{Thresholds: map[string]float64{}, Dependencies: map[string][]string{}, DuplicateFragments: map[string]bool{}}
	if err := db.QueryRow(`SELECT columbo_version FROM report`).Scan(&s.Version); err != nil {
		return Snapshot{}, fmt.Errorf("read report: %w", err)
	}
	rows, err := db.Query(`SELECT c.smell, d.symbol, f.path, c.id, c.verdict
		FROM cases c JOIN declarations d ON d.id = c.primary_declaration_id JOIN files f ON f.id = d.file_id
		WHERE c.suppressed = 0`)
	if err != nil {
		return Snapshot{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var f Finding
		if err := rows.Scan(&f.Smell, &f.Symbol, &f.File, &f.CaseID, &f.Verdict); err != nil {
			return Snapshot{}, err
		}
		s.Findings = append(s.Findings, f)
	}
	if err := rows.Err(); err != nil {
		return Snapshot{}, err
	}
	if err := readSupport(db, s.Findings); err != nil {
		return Snapshot{}, err
	}
	for _, table := range []string{"source_config_counts", "source_config_ratios"} {
		if err := readThresholds(db, table, s.Thresholds); err != nil {
			return Snapshot{}, err
		}
	}
	if err := readDependencies(db, s.Dependencies); err != nil {
		return Snapshot{}, err
	}
	if err := readDuplicateFragments(db, s.DuplicateFragments); err != nil {
		return Snapshot{}, err
	}
	if s.Parameters, err = readParameterReceipts(db); err != nil {
		return Snapshot{}, err
	}
	if s.Guidance, err = readGuidance(db); err != nil {
		return Snapshot{}, err
	}
	if err := db.QueryRow(`SELECT count(*) FROM cases WHERE suppressed = 1`).Scan(&s.Suppressed); err != nil {
		return Snapshot{}, err
	}
	sortFindings(s.Findings)
	return s, nil
}

func readDependencies(db *sql.DB, into map[string][]string) error {
	rows, err := db.Query(`SELECT d.symbol, dep.identity FROM declaration_dependencies dd
		JOIN declarations d ON d.id = dd.declaration_id JOIN dependencies dep ON dep.id = dd.dependency_id
		WHERE dd.scored = 1 ORDER BY d.symbol, dep.identity`)
	if err != nil {
		return fmt.Errorf("read dependencies: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var symbol, identity string
		if err := rows.Scan(&symbol, &identity); err != nil {
			return err
		}
		into[symbol] = append(into[symbol], identity)
	}
	return rows.Err()
}

func readSupport(db *sql.DB, fs []Finding) error {
	byID := map[string]*Finding{}
	for i := range fs {
		byID[fs[i].CaseID] = &fs[i]
	}
	rows, err := db.Query(`SELECT DISTINCT cd.case_id, f.path, d.symbol, d.start_offset
		FROM case_declarations cd JOIN declarations d ON d.id = cd.declaration_id JOIN files f ON f.id = d.file_id
		ORDER BY cd.case_id, f.path, d.start_offset, d.symbol`)
	if err != nil {
		return fmt.Errorf("read case_declarations: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var d Decl
		if err := rows.Scan(&id, &d.File, &d.Symbol, &d.Offset); err != nil {
			return err
		}
		if f := byID[id]; f != nil {
			f.Support = append(f.Support, d)
		}
	}
	return rows.Err()
}

func readThresholds(db *sql.DB, table string, into map[string]float64) error {
	rows, err := db.Query(`SELECT name, value FROM ` + table)
	if err != nil {
		return fmt.Errorf("read %s: %w", table, err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		var value float64
		if err := rows.Scan(&name, &value); err != nil {
			return err
		}
		into[name] = value
	}
	return rows.Err()
}

func sortFindings(fs []Finding) {
	sort.Slice(fs, func(i, j int) bool {
		a, b := fs[i], fs[j]
		if a.Smell != b.Smell {
			return a.Smell < b.Smell
		}
		if a.Symbol != b.Symbol {
			return a.Symbol < b.Symbol
		}
		return a.CaseID < b.CaseID
	})
}

func readDuplicateFragments(db *sql.DB, into map[string]bool) error {
	rows, err := db.Query(`SELECT DISTINCT d.symbol FROM source_receipts r
		JOIN declarations d ON d.id = r.source_declaration_id WHERE r.kind = 'duplicate-fragment'`)
	if err != nil {
		return fmt.Errorf("read duplicate fragments: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var symbol string
		if err := rows.Scan(&symbol); err != nil {
			return err
		}
		into[symbol] = true
	}
	return rows.Err()
}

func readGuidance(db *sql.DB) (map[string]Guidance, error) {
	out := map[string]Guidance{}
	rows, err := db.Query(`SELECT id, why, diagnosis FROM cases`)
	if err != nil {
		return nil, fmt.Errorf("read guidance: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var g Guidance
		if err := rows.Scan(&id, &g.Why, &g.Diagnosis); err != nil {
			return nil, err
		}
		out[id] = g
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	items, err := db.Query(`SELECT case_id, kind, item FROM case_guidance ORDER BY case_id, kind, ordinal`)
	if err != nil {
		return nil, fmt.Errorf("read case_guidance: %w", err)
	}
	defer items.Close()
	for items.Next() {
		var id, kind, item string
		if err := items.Scan(&id, &kind, &item); err != nil {
			return nil, err
		}
		g := out[id]
		if kind == "lead" {
			g.Leads = append(g.Leads, item)
		} else {
			g.Avoid = append(g.Avoid, item)
		}
		out[id] = g
	}
	return out, items.Err()
}
