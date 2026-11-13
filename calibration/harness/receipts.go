package harness

import (
	"database/sql"
	"strconv"
	"strings"
)

// ParameterReceipt is one parameter Columbo cited as evidence for a case: the declaration
// that holds it, its expanded index in that declaration's parameter list, and its canonical type.
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
