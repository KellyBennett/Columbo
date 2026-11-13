package report

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type TriageEntry struct {
	Operator string `yaml:"operator"`
	Symbol   string `yaml:"symbol"`
	Dose     int    `yaml:"dose"`
	Status   string `yaml:"status"`
	Reason   string `yaml:"reason"`
}

func LoadTriage(path string) ([]TriageEntry, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var entries []TriageEntry
	if err := yaml.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	for _, e := range entries {
		if e.Status != "equivalent" && e.Status != "blind-spot" {
			return nil, fmt.Errorf("%s: %s %s: status must be equivalent or blind-spot", path, e.Operator, e.Symbol)
		}
		if strings.TrimSpace(e.Reason) == "" {
			return nil, fmt.Errorf("%s: %s %s: reason is required", path, e.Operator, e.Symbol)
		}
	}
	return entries, nil
}

func loadTriageTable(db *sql.DB, entries []TriageEntry) error {
	if _, err := db.Exec(`DROP TABLE IF EXISTS temp.triage; CREATE TEMP TABLE triage (operator TEXT, symbol TEXT, dose INTEGER, status TEXT, reason TEXT)`); err != nil {
		return err
	}
	for _, e := range entries {
		if _, err := db.Exec(`INSERT INTO temp.triage VALUES (?,?,?,?,?)`, e.Operator, e.Symbol, e.Dose, e.Status, e.Reason); err != nil {
			return err
		}
	}
	return nil
}

type section struct {
	title, note, query string
}

// latestRuns selects, per corpus and operator, the mutants of the newest run that measured
// that operator, so families can be measured in separate runs without hiding each other.
const latestRuns = `latest AS (SELECT max(id) AS id, corpus_module FROM runs GROUP BY corpus_module),
cur AS (SELECT m.* FROM mutants m JOIN runs r ON r.id = m.run_id
 WHERE m.run_id = (SELECT max(m2.run_id) FROM mutants m2 JOIN runs r2 ON r2.id = m2.run_id
  WHERE r2.corpus_module = r.corpus_module AND m2.operator = m.operator))`

var sections = []section{
	{
		title: "Runs",
		note:  "The latest run per corpus module, and how many of its mutants are current. Each operator's numbers come from the newest run that measured it on that corpus.",
		query: `WITH ` + latestRuns + `
SELECT r.id, r.corpus_module, r.corpus_version, r.columbo_version, r.config_hash, r.validity,
 (SELECT count(*) FROM baseline_findings b WHERE b.run_id = r.id) AS baseline_findings,
 (SELECT count(*) FROM cur m WHERE m.run_id = r.id) AS mutants
FROM runs r JOIN latest l ON l.id = r.id ORDER BY r.corpus_module`,
	},
	{
		title: "Outcomes by operator",
		note:  "Invalid mutants failed vet or tests and are excluded from every rate.",
		query: `WITH ` + latestRuns + `
SELECT class, operator, target,
 sum(status='killed') AS killed, sum(status='survived') AS survived, sum(status='equivalent') AS equivalent,
 sum(status='conflict') AS conflict, sum(status='id-drift') AS id_drift, sum(status='invalid') AS invalid,
 CASE WHEN sum(status IN ('killed','survived','conflict')) = 0 THEN '' ELSE
  printf('%.0f%%', 100.0 * sum(status='killed') / sum(status IN ('killed','survived','conflict'))) END AS kill_rate
FROM cur
GROUP BY class, operator, target ORDER BY class, operator`,
	},
	{
		title: "Outcomes by corpus",
		query: `WITH ` + latestRuns + `
SELECT r.corpus_module, m.class, sum(m.status='killed') AS killed, sum(m.status='survived') AS survived,
 sum(m.status='equivalent') AS equivalent, sum(m.status='conflict') AS conflict, sum(m.status='id-drift') AS id_drift, sum(m.status='invalid') AS invalid
FROM cur m JOIN runs r ON r.id = m.run_id
GROUP BY r.corpus_module, m.class ORDER BY r.corpus_module, m.class`,
	},
	{
		title: "Kill matrix",
		note:  "Valid inject and evade mutants, by operator and by every smell they added anywhere. Read across a row to see which rules notice an operator; down a column to see what a rule notices.",
		query: `WITH ` + latestRuns + `
SELECT m.operator, f.smell, count(DISTINCT m.id) AS mutants
FROM cur m JOIN mutant_findings f ON f.mutant_id = m.id AND f.side = 'added'
WHERE m.class IN ('inject','evade') AND m.status <> 'invalid'
GROUP BY m.operator, f.smell ORDER BY m.operator, mutants DESC`,
	},
	{
		title: "Operator families no rule notices",
		note:  "Valid inject or evade operators whose mutants never added any finding. Each is a candidate missing rule.",
		query: `WITH ` + latestRuns + `
SELECT m.operator, count(*) AS valid_mutants
FROM cur m WHERE m.class IN ('inject','evade') AND m.status <> 'invalid'
GROUP BY m.operator
HAVING sum(EXISTS (SELECT 1 FROM mutant_findings f WHERE f.mutant_id = m.id AND f.side='added')) = 0`,
	},
	{
		title: "Redundant rules",
		note:  "A smell whose every killing mutant also added another given smell. The second rule already notices everything the first does on this corpus.",
		query: `WITH ` + latestRuns + `, added AS (
 SELECT DISTINCT m.id, f.smell FROM cur m JOIN mutant_findings f ON f.mutant_id = m.id AND f.side='added'
 WHERE m.class IN ('inject','evade') AND m.status <> 'invalid')
SELECT a.smell AS rule, b.smell AS covered_by, count(*) AS mutants
FROM added a JOIN added b ON a.id = b.id AND a.smell <> b.smell
GROUP BY a.smell, b.smell
HAVING count(*) = (SELECT count(*) FROM added x WHERE x.smell = a.smell) AND count(*) >= 3
ORDER BY a.smell`,
	},
	{
		title: "Dose curves",
		note:  "For each inject operator and site, the first dose that killed. A site with no killing dose is counted under never.",
		query: `WITH ` + latestRuns + `, sites AS (
 SELECT operator, site_symbol, min(CASE WHEN status='killed' THEN dose END) AS first_kill, max(dose) AS max_dose
 FROM cur WHERE class='inject' AND status <> 'invalid' GROUP BY operator, site_symbol)
SELECT operator, coalesce(CAST(first_kill AS TEXT), 'never') AS first_killing_dose, count(*) AS sites
FROM sites GROUP BY operator, first_kill ORDER BY operator, first_kill`,
	},
	{
		title: "Prediction agreement",
		note:  "Inject and cure mutants whose operator predicted the outcome. Disagreements are either harness metric bugs or Columbo behavior worth a look.",
		query: `WITH ` + latestRuns + `
SELECT operator,
 sum(predicted_kill IS NOT NULL AND status <> 'invalid') AS predicted,
 sum(predicted_kill = 1 AND status IN ('survived')) AS predicted_kill_but_survived,
 sum(predicted_kill = 0 AND status = 'killed') AS predicted_survive_but_killed
FROM cur WHERE class IN ('inject','cure')
GROUP BY operator ORDER BY operator`,
	},
	{
		title: "Cure conflicts",
		note:  "A cure that cleared its target but set off another rule. If a proper refactoring trips a different rule, that code has no green path.",
		query: `WITH ` + latestRuns + `
SELECT operator, site_symbol, dose, size, reason FROM cur
WHERE status = 'conflict' ORDER BY operator, site_symbol, dose`,
	},
	{
		title: "Conflicting rules",
		note:  "Which rules the conflicts set off, per cure operator.",
		query: `WITH ` + latestRuns + `
SELECT m.operator, f.smell, count(*) AS added
FROM cur m JOIN mutant_findings f ON f.mutant_id = m.id AND f.side='added'
WHERE m.status = 'conflict'
GROUP BY m.operator, f.smell ORDER BY m.operator, added DESC`,
	},
	{
		title: "Cost asymmetry",
		note:  "Per smell: the smallest surviving evade against the smallest successful cure, in changed AST nodes. The README promises a violation is harder to silence than to properly refactor; that holds where the evade is larger.",
		query: `WITH ` + latestRuns + `, ev AS (
 SELECT target, min(size) AS s, count(*) AS n FROM cur WHERE class='evade' AND status='survived' GROUP BY target),
cu AS (
 SELECT target, min(size) AS s, count(*) AS n FROM cur WHERE class='cure' AND status='killed' GROUP BY target),
t AS (SELECT target FROM ev UNION SELECT target FROM cu)
SELECT t.target AS smell, coalesce(CAST(ev.s AS TEXT), 'none survived') AS smallest_surviving_evade, coalesce(ev.n, 0) AS surviving_evades,
 coalesce(CAST(cu.s AS TEXT), 'no cure reached green') AS smallest_green_cure, coalesce(cu.n, 0) AS green_cures,
 CASE WHEN ev.s IS NULL THEN 'holds (no evade survives)' WHEN cu.s IS NULL THEN 'fails (no green cure)'
  WHEN ev.s > cu.s THEN 'holds' ELSE 'fails' END AS promise
FROM t LEFT JOIN ev ON ev.target = t.target LEFT JOIN cu ON cu.target = t.target ORDER BY t.target`,
	},
	{
		title: "Neutral drift",
		query: `WITH ` + latestRuns + `
SELECT operator, site_symbol, reason FROM cur
WHERE status = 'id-drift' ORDER BY operator, site_symbol`,
	},
	{
		title: "Triaged survivors",
		query: `WITH ` + latestRuns + `
SELECT m.operator, m.site_symbol, m.dose, t.status, t.reason FROM cur m
JOIN temp.triage t ON t.operator = m.operator AND (t.symbol = '*' OR t.symbol = m.site_symbol) AND (t.dose = 0 OR t.dose = m.dose)
WHERE m.status = 'survived' ORDER BY t.status, m.operator`,
	},
	{
		title: "Untriaged survivors",
		note:  "Survivors the operator's prediction cannot explain and calibration/triage.yaml does not cover. CI warns on every row here.",
		query: `WITH ` + latestRuns + `
SELECT m.operator, m.site_symbol, m.dose, m.reason FROM cur m
WHERE m.status = 'survived'
 AND NOT EXISTS (SELECT 1 FROM temp.triage t WHERE t.operator = m.operator AND (t.symbol = '*' OR t.symbol = m.site_symbol) AND (t.dose = 0 OR t.dose = m.dose))
ORDER BY m.operator, m.site_symbol, m.dose`,
	},
}

// Write renders every section as Markdown and returns the number of untriaged survivors.
func Write(w io.Writer, db *sql.DB, triage []TriageEntry) (int, error) {
	if err := loadTriageTable(db, triage); err != nil {
		return 0, err
	}
	fmt.Fprintln(w, "# Calibration report")
	untriaged := 0
	for _, s := range sections {
		n, err := writeSection(w, db, s)
		if err != nil {
			return 0, fmt.Errorf("%s: %w", s.title, err)
		}
		if s.title == "Untriaged survivors" {
			untriaged = n
		}
	}
	return untriaged, nil
}

func writeSection(w io.Writer, db *sql.DB, s section) (int, error) {
	rows, err := db.Query(s.query)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return 0, err
	}
	fmt.Fprintf(w, "\n## %s\n\n", s.title)
	if s.note != "" {
		fmt.Fprintf(w, "%s\n\n", s.note)
	}
	var lines []string
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return 0, err
		}
		cells := make([]string, len(cols))
		for i, v := range vals {
			cells[i] = cell(v)
		}
		lines = append(lines, "| "+strings.Join(cells, " | ")+" |")
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if len(lines) == 0 {
		fmt.Fprintln(w, "None.")
		return 0, nil
	}
	fmt.Fprintln(w, "| "+strings.Join(cols, " | ")+" |")
	fmt.Fprintln(w, "|"+strings.Repeat("---|", len(cols)))
	for _, l := range lines {
		fmt.Fprintln(w, l)
	}
	return len(lines), nil
}

func cell(v any) string {
	switch v := v.(type) {
	case nil:
		return ""
	case []byte:
		return escape(string(v))
	case string:
		return escape(v)
	}
	return escape(fmt.Sprint(v))
}

func escape(s string) string {
	s = strings.ReplaceAll(s, "|", `\|`)
	s = strings.ReplaceAll(s, "\n", " ")
	if i := strings.Index(s, "github.com/"); i >= 0 {
		s = strings.ReplaceAll(s, "github.com/KellyBennett/Columbo/", "")
	}
	return s
}
