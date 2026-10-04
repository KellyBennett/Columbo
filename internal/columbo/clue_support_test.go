package columbo

import "testing"

func TestSnapshotClueSupportRoles(t *testing.T) {
	h := &testHarness{T: t}
	db := h.snapshot(h.allSmellsReport())
	h.equal(clueSupportRoles, h.sqlStrings(db, clueSupportRolesQuery))
}

var clueSupportRoles = []string{
	"clump-occurrences|declaration|3",
	"clump-size|parameter|9",
	"clump-types|parameter|9",
	"foreign-accesses|foreign-access|5",
	"foreign-own-ratio|foreign-access|5",
	"own-accesses|none|0",
	"parameters|parameter|11",
}

// Count only each clue's established support, including the zero-own case.
const clueSupportRolesQuery = `SELECT q.kind || '|' || COALESCE(r.kind, 'none') || '|' || COUNT(r.id)
FROM clues q JOIN cases c ON c.id = q.case_id
LEFT JOIN clue_receipts link ON link.clue_id = q.id AND link.case_id = q.case_id
LEFT JOIN source_receipts r ON r.id = link.receipt_id AND r.case_id = link.case_id
WHERE c.smell IN ('data-clump', 'long-parameter-list', 'feature-envy')
GROUP BY q.kind, r.kind ORDER BY q.kind, r.kind`

func TestFeatureEnvyRatioReceiptSupport(t *testing.T) {
	h := &testHarness{T: t}
	db := h.snapshot(h.nonzeroOwnAccessReport())
	h.equal(nonzeroOwnAccessSupport, h.sqlStrings(db, clueSupportRolesQuery))
}

func (h *testHarness) nonzeroOwnAccessReport() Report {
	config := quiet()
	config.Severity["feature-envy"] = "fail"
	config.Counts["feature-envy-foreign-accesses"] = 1
	config.Ratios["feature-envy-ratio"] = .1
	return h.investigate(h.fixture(stableAccessSource), config)
}

var nonzeroOwnAccessSupport = []string{
	"foreign-accesses|foreign-access|5",
	"foreign-own-ratio|foreign-access|5",
	"foreign-own-ratio|own-access|4",
	"own-accesses|own-access|1",
}
