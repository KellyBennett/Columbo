package harness

import (
	"database/sql"
	"encoding/json"
)

const schema = `
CREATE TABLE IF NOT EXISTS runs (
 id INTEGER PRIMARY KEY,
 corpus_module TEXT NOT NULL,
 corpus_version TEXT NOT NULL,
 columbo_version TEXT NOT NULL,
 config_hash TEXT NOT NULL,
 validity TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS baseline_findings (
 run_id INTEGER NOT NULL REFERENCES runs(id),
 smell TEXT NOT NULL, symbol TEXT NOT NULL, file TEXT NOT NULL, case_id TEXT NOT NULL, verdict TEXT NOT NULL, ratio REAL
);
CREATE TABLE IF NOT EXISTS mutants (
 id INTEGER PRIMARY KEY,
 run_id INTEGER NOT NULL REFERENCES runs(id),
 parent_id INTEGER REFERENCES mutants(id),
 class TEXT NOT NULL,
 operator TEXT NOT NULL,
 target TEXT NOT NULL,
 site_package TEXT NOT NULL,
 site_symbol TEXT NOT NULL,
 dose INTEGER NOT NULL,
 chain TEXT NOT NULL,
 size INTEGER NOT NULL,
 status TEXT NOT NULL,
 reason TEXT NOT NULL,
 predicted_kill INTEGER,
 prediction TEXT NOT NULL,
 patch BLOB NOT NULL
);
CREATE TABLE IF NOT EXISTS mutant_findings (
 mutant_id INTEGER NOT NULL REFERENCES mutants(id),
 side TEXT NOT NULL CHECK (side IN ('added','removed')),
 smell TEXT NOT NULL, symbol TEXT NOT NULL, case_id TEXT NOT NULL, verdict TEXT NOT NULL, ratio REAL
);
`

type Store struct{ db *sql.DB }

type RunInfo struct {
	Module, Version, Columbo, ConfigHash string
	Validity                             Validity
}

func OpenStore(path string) (*Store, error) {
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		return nil, err
	}
	if err := addRatioColumns(db); err != nil {
		return nil, err
	}
	return &Store{db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) DB() *sql.DB { return s.db }

func (s *Store) BeginRun(info RunInfo, baseline []Finding) (int64, error) {
	res, err := s.db.Exec(`INSERT INTO runs (corpus_module, corpus_version, columbo_version, config_hash, validity) VALUES (?,?,?,?,?)`,
		info.Module, info.Version, info.Columbo, info.ConfigHash, string(info.Validity))
	if err != nil {
		return 0, err
	}
	run, _ := res.LastInsertId()
	for _, f := range baseline {
		if _, err := s.db.Exec(`INSERT INTO baseline_findings (run_id, smell, symbol, file, case_id, verdict, ratio) VALUES (?,?,?,?,?,?,?)`, run, f.Smell, f.Symbol, f.File, f.CaseID, f.Verdict, f.Ratio); err != nil {
			return 0, err
		}
	}
	return run, nil
}

func (s *Store) Record(run int64, parent int64, o Outcome) (int64, error) {
	chain, _ := json.Marshal(o.Mutant.Chain)
	last := o.Mutant.Chain[len(o.Mutant.Chain)-1]
	var predicted any
	if o.Predict.Kill != nil {
		predicted = *o.Predict.Kill
	}
	var parentID any
	if parent > 0 {
		parentID = parent
	}
	res, err := s.db.Exec(`INSERT INTO mutants (run_id, parent_id, class, operator, target, site_package, site_symbol, dose, chain, size, status, reason, predicted_kill, prediction, patch)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		run, parentID, string(o.Class), last.Operator, o.Target, last.Site.Package, last.Site.Symbol, last.Dose, string(chain), o.Mutant.Size,
		string(o.Status), o.Reason, predicted, o.Predict.Reason, o.Mutant.Patch)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	for side, fs := range map[string][]Finding{"added": o.Added, "removed": o.Removed} {
		for _, f := range fs {
			if _, err := s.db.Exec(`INSERT INTO mutant_findings (mutant_id, side, smell, symbol, case_id, verdict, ratio) VALUES (?,?,?,?,?,?,?)`, id, side, f.Smell, f.Symbol, f.CaseID, f.Verdict, f.Ratio); err != nil {
				return 0, err
			}
		}
	}
	return id, nil
}
