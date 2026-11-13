package harness

import (
	"database/sql"
	"strconv"
	"strings"
)

// ParameterReceipt.Index is the position after expanding one entry per declared parameter name.
type ParameterReceipt struct {
	Symbol string
	Index  int
	Type   string
}

func readParameterReceipts(db *sql.DB) (map[string][]ParameterReceipt, error) {
	rows, err := db.Query(`SELECT r.case_id, d.symbol, r.subject FROM source_receipts r
		JOIN declarations d ON d.id = r.source_declaration_id
		WHERE r.kind = 'parameter' ORDER BY r.case_id, r.ordinal`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]ParameterReceipt{}
	for rows.Next() {
		var caseID, symbol, subject string
		if err := rows.Scan(&caseID, &symbol, &subject); err != nil {
			return nil, err
		}
		index, typ, ok := strings.Cut(subject, ":")
		n, err := strconv.Atoi(index)
		if !ok || err != nil {
			continue
		}
		out[caseID] = append(out[caseID], ParameterReceipt{Symbol: symbol, Index: n, Type: typ})
	}
	return out, rows.Err()
}
