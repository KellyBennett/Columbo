package harness

import (
	"database/sql"
	"fmt"
)

// caseRatio mirrors Columbo's excess rule because this module cannot import it. The ratio
// is the case's first limited clue divided by the smallest value its comparison accepts.
const caseRatio = `(SELECT CASE WHEN l.trigger > 0 THEN l.value / l.trigger END FROM (
 SELECT CAST(numeric_value AS REAL) AS value, CASE
   WHEN operator = '>=' THEN limit_value
   WHEN operator = '>' AND value_type = 'integer' THEN limit_value + 1
   WHEN operator = '>' AND value_type = 'real' THEN limit_value END AS trigger
 FROM clues WHERE case_id = c.id AND limit_value IS NOT NULL ORDER BY ordinal LIMIT 1) l)`

var ratioTables = []string{"baseline_findings", "mutant_findings"}

func addRatioColumns(db *sql.DB) error {
	for _, table := range ratioTables {
		var n int
		if err := db.QueryRow(`SELECT count(*) FROM pragma_table_info(?) WHERE name = 'ratio'`, table).Scan(&n); err != nil {
			return fmt.Errorf("inspect %s: %w", table, err)
		}
		if n > 0 {
			continue
		}
		if _, err := db.Exec(`ALTER TABLE ` + table + ` ADD COLUMN ratio REAL`); err != nil {
			return fmt.Errorf("add %s.ratio: %w", table, err)
		}
	}
	return nil
}
