package harness

import (
	"database/sql"
	"fmt"
	"sort"

	_ "modernc.org/sqlite"
)

type Snapshot struct {
	Findings   []Finding
	Thresholds map[string]float64
	Version    string
	// Dependencies maps a declaration symbol to its scored dependency identities.
	Dependencies map[string][]string
	// DuplicateFragments holds every declaration symbol that anchors a duplicate-code fragment.
	DuplicateFragments map[string]bool
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
