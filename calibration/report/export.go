package report

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// exports lists the raw tables a reviewer can trace report numbers to.
var exports = map[string]string{
	"trials":      `SELECT * FROM trials WHERE infra_error = '' ORDER BY smell, corpus_module, site_symbol, dose, repeat`,
	"lead_checks": `SELECT * FROM lead_checks WHERE (corpus_module, batch) IN (SELECT corpus_module, max(batch) FROM lead_checks GROUP BY corpus_module) ORDER BY corpus_module, smell, case_id, lead_ordinal`,
}

// wide columns hold patches and prose; TSV leaves them out and JSON Lines keeps them.
var wide = map[string]bool{"prompt": true, "reply": true, "mutant_patch": true, "fix_patch": true, "note": true}

func Export(w io.Writer, db *sql.DB, table, format string) error {
	query, ok := exports[table]
	if !ok {
		return fmt.Errorf("unknown table %q", table)
	}
	rows, err := db.Query(query)
	if err != nil {
		return err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return err
	}
	if format == "tsv" {
		fmt.Fprintln(w, strings.Join(narrow(cols, cols), "\t"))
	}
	enc := json.NewEncoder(w)
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return err
		}
		for i, v := range vals {
			if b, isBytes := v.([]byte); isBytes {
				vals[i] = string(b)
			}
		}
		switch format {
		case "tsv":
			cells := make([]string, len(vals))
			for i, v := range vals {
				cells[i] = tsvCell(v)
			}
			fmt.Fprintln(w, strings.Join(narrow(cols, cells), "\t"))
		case "jsonl":
			obj := map[string]any{}
			for i, c := range cols {
				obj[c] = vals[i]
			}
			if err := enc.Encode(obj); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown format %q", format)
		}
	}
	return rows.Err()
}

func narrow(cols, cells []string) []string {
	var out []string
	for i, c := range cols {
		if !wide[c] {
			out = append(out, cells[i])
		}
	}
	return out
}

func tsvCell(v any) string {
	if v == nil {
		return ""
	}
	return strings.NewReplacer("\t", " ", "\n", " ", "\r", " ").Replace(fmt.Sprint(v))
}
