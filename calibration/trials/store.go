package trials

import (
	"database/sql"
	"strings"

	"github.com/KellyBennett/Columbo/calibration/harness"
)

const schema = `
CREATE TABLE IF NOT EXISTS trials (
 id INTEGER PRIMARY KEY,
 trial_key TEXT NOT NULL UNIQUE,
 smell TEXT NOT NULL,
 operator TEXT NOT NULL,
 corpus_module TEXT NOT NULL,
 corpus_version TEXT NOT NULL,
 site_symbol TEXT NOT NULL,
 dose INTEGER NOT NULL,
 repeat INTEGER NOT NULL,
 columbo_version TEXT NOT NULL,
 config_hash TEXT NOT NULL,
 validity TEXT NOT NULL,
 agent_command TEXT NOT NULL,
 model TEXT NOT NULL,
 model_id TEXT NOT NULL,
 prompt_version TEXT NOT NULL,
 prompt TEXT NOT NULL,
 mutant_cases TEXT NOT NULL,
 mutant_patch BLOB NOT NULL,
 agent_exit INTEGER NOT NULL,
 timed_out INTEGER NOT NULL,
 agent_stop TEXT NOT NULL,
 turns INTEGER NOT NULL,
 cost_usd REAL NOT NULL,
 wall_ms INTEGER NOT NULL,
 reply TEXT NOT NULL,
 infra_error TEXT NOT NULL,
 vet_pass INTEGER,
 tests_pass INTEGER,
 analyzed INTEGER,
 cleared INTEGER,
 target_remaining INTEGER,
 other_remaining INTEGER,
 introduced INTEGER,
 introduced_cases TEXT,
 out_of_scope_gone INTEGER,
 out_of_scope_gone_cases TEXT,
 suppressed INTEGER,
 gamed INTEGER,
 distance_fixed INTEGER,
 distance_mutant INTEGER,
 fix_distance REAL,
 files_changed INTEGER,
 test_files_changed INTEGER,
 flags TEXT,
 note TEXT,
 fix_patch BLOB
);
CREATE TABLE IF NOT EXISTS lead_checks (
 id INTEGER PRIMARY KEY,
 batch INTEGER NOT NULL,
 corpus_module TEXT NOT NULL,
 corpus_version TEXT NOT NULL,
 columbo_version TEXT NOT NULL,
 config_hash TEXT NOT NULL,
 validity TEXT NOT NULL,
 smell TEXT NOT NULL,
 case_id TEXT NOT NULL,
 symbol TEXT NOT NULL,
 lead_ordinal INTEGER NOT NULL,
 lead TEXT NOT NULL,
 refactoring TEXT NOT NULL,
 operator TEXT NOT NULL,
 outcome TEXT NOT NULL,
 applications INTEGER NOT NULL,
 dose INTEGER NOT NULL,
 reason TEXT NOT NULL
);
`

type Store struct{ db *sql.DB }

// NewStore adds the phase 8 tables to an open calibration database.
func NewStore(db *sql.DB) (*Store, error) {
	if _, err := db.Exec(schema); err != nil {
		return nil, err
	}
	return &Store{db}, nil
}

// Provenance ties stored rows to the corpus and Columbo build that produced them.
type Provenance struct {
	Module, Version, Columbo, ConfigHash, Validity string
}

// Row is one scored trial. Score is nil when the agent never got to work.
type Row struct {
	Key     string
	Trial   Trial
	Prov    Provenance
	Command string
	Model   string
	Prompt  string
	Run     AgentRun
	Score   *Score
}

// Done reports whether a trial already has a usable row. Rows with infrastructure errors are deleted so a rerun retries them.
func (s *Store) Done(key string) (bool, error) {
	if _, err := s.db.Exec(`DELETE FROM trials WHERE trial_key = ? AND infra_error <> ''`, key); err != nil {
		return false, err
	}
	var n int
	err := s.db.QueryRow(`SELECT count(*) FROM trials WHERE trial_key = ?`, key).Scan(&n)
	return n > 0, err
}

// record pairs each column with its value so the INSERT's two lists cannot drift apart.
type record struct {
	cols []string
	vals []any
}

func (r *record) set(col string, v any) {
	r.cols = append(r.cols, col)
	r.vals = append(r.vals, v)
}

func (s *Store) Record(r Row) error {
	p := r.Trial.Plant
	var rec record
	for _, kv := range []struct {
		col string
		v   any
	}{
		{"trial_key", r.Key}, {"smell", p.Smell}, {"operator", p.Operator}, {"corpus_module", r.Prov.Module},
		{"corpus_version", r.Prov.Version}, {"site_symbol", p.Site.Symbol}, {"dose", p.Dose}, {"repeat", r.Trial.Repeat},
		{"columbo_version", r.Prov.Columbo}, {"config_hash", r.Prov.ConfigHash}, {"validity", r.Prov.Validity},
		{"agent_command", r.Command}, {"model", r.Model}, {"model_id", r.Run.ModelID}, {"prompt_version", PromptVersion},
		{"prompt", r.Prompt}, {"mutant_cases", cases(p.Added)}, {"mutant_patch", p.Patch}, {"agent_exit", r.Run.Exit},
		{"timed_out", r.Run.TimedOut}, {"agent_stop", r.Run.Stop}, {"turns", r.Run.Turns}, {"cost_usd", r.Run.CostUSD},
		{"wall_ms", r.Run.Wall.Milliseconds()}, {"reply", r.Run.Reply}, {"infra_error", r.Run.InfraError},
	} {
		rec.set(kv.col, kv.v)
	}
	if sc := r.Score; sc != nil {
		var tests any
		if sc.TestsPass != nil {
			tests = *sc.TestsPass
		}
		for _, kv := range []struct {
			col string
			v   any
		}{
			{"vet_pass", sc.VetPass}, {"tests_pass", tests}, {"analyzed", sc.Analyzed}, {"cleared", sc.Cleared()},
			{"target_remaining", sc.TargetRemaining}, {"other_remaining", sc.OtherRemaining},
			{"introduced", len(sc.Introduced)}, {"introduced_cases", cases(sc.Introduced)},
			{"out_of_scope_gone", len(sc.OutOfScopeGone)}, {"out_of_scope_gone_cases", cases(sc.OutOfScopeGone)},
			{"suppressed", sc.Suppressed}, {"gamed", sc.Gamed()}, {"distance_fixed", sc.Distance.Fixed},
			{"distance_mutant", sc.Distance.Mutant}, {"fix_distance", sc.Distance.Normalized()},
			{"files_changed", sc.FilesChanged}, {"test_files_changed", sc.TestFilesChanged},
			{"flags", strings.Join(sc.Flags, ",")}, {"note", sc.Note}, {"fix_patch", sc.Patch},
		} {
			rec.set(kv.col, kv.v)
		}
	}
	_, err := s.db.Exec(`INSERT INTO trials (`+strings.Join(rec.cols, ", ")+`) VALUES (`+
		strings.TrimSuffix(strings.Repeat("?,", len(rec.cols)), ",")+`)`, rec.vals...)
	return err
}

func cases(fs []harness.Finding) string {
	out := make([]string, len(fs))
	for i, f := range fs {
		out[i] = f.Smell + "@" + f.Symbol
	}
	return strings.Join(out, ", ")
}

// NextBatch numbers one lead-check invocation so the report can read the newest per corpus.
func (s *Store) NextBatch() (int64, error) {
	var n int64
	err := s.db.QueryRow(`SELECT coalesce(max(batch), 0) + 1 FROM lead_checks`).Scan(&n)
	return n, err
}

func (s *Store) RecordLead(batch int64, prov Provenance, c LeadCheck) error {
	_, err := s.db.Exec(`INSERT INTO lead_checks (batch, corpus_module, corpus_version, columbo_version, config_hash, validity,
		smell, case_id, symbol, lead_ordinal, lead, refactoring, operator, outcome, applications, dose, reason)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		batch, prov.Module, prov.Version, prov.Columbo, prov.ConfigHash, prov.Validity,
		c.Case.Smell, c.Case.CaseID, c.Case.Symbol, c.Ordinal, c.Lead, c.Refactoring.Name, c.Refactoring.Operator,
		string(c.Outcome), c.Applications, c.Dose, c.Reason)
	return err
}
