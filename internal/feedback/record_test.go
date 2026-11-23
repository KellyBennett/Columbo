package feedback

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseMarker(t *testing.T) {
	tests := []struct {
		name string
		body string
		want *Marker
	}{
		{"finding with rule", "<!-- columbo-feedback case=C-1 role=finding state=open -->\n**Columbo: long-function** in `p.F` (new)\nWhy", &Marker{CaseID: "C-1", Role: "finding", State: "open", Rule: "long-function"}},
		{"cleared", "<!-- columbo-feedback case=C-2 role=finding state=cleared -->\n**Columbo: data-clump** in `p.F`", &Marker{CaseID: "C-2", Role: "finding", State: "cleared", Rule: "data-clump"}},
		{"other role kept as written", "<!-- columbo-feedback case=C-3 role=fix state=open -->\nApplied", &Marker{CaseID: "C-3", Role: "fix", State: "open"}},
		{"header without rule", "<!-- columbo-feedback case=C-4 role=finding state=open -->\nSomething else", &Marker{CaseID: "C-4", Role: "finding", State: "open"}},
		{"marker only", "<!-- columbo-feedback case=C-5 role=finding state=open -->", &Marker{CaseID: "C-5", Role: "finding", State: "open"}},
		{"not at start", "quote\n<!-- columbo-feedback case=C-6 role=finding state=open -->", nil},
		{"unknown state", "<!-- columbo-feedback case=C-7 role=finding state=gone -->", nil},
		{"plain comment", "Looks good", nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.want, parseMarker(test.body))
		})
	}
}

func TestParseSuppressions(t *testing.T) {
	tests := []struct {
		name  string
		patch string
		want  []Suppression
	}{
		{"added line numbered on the right side",
			"@@ -1,3 +1,4 @@\n package a\n-old\n+// columbo:ignore long-function -- table-driven by design  \n func A() {}",
			[]Suppression{{Path: "a.go", Line: 2, Rule: "long-function", Justification: "table-driven by design"}}},
		{"second hunk restarts numbering, tabs allowed",
			"@@ -1 +1 @@\n x\n@@ -40,2 +50,3 @@\n y\n+\t//\tcolumbo:ignore\tdata-clump\t--\tkept for the wire format",
			[]Suppression{{Path: "a.go", Line: 51, Rule: "data-clump", Justification: "kept for the wire format"}}},
		{"context and removed lines are not suppressions",
			"@@ -1,2 +1,1 @@\n // columbo:ignore long-function -- old\n-// columbo:ignore data-clump -- gone",
			nil},
		{"trailing comment after code is not a directive",
			"@@ -1 +1,2 @@\n x\n+f() // columbo:ignore long-function -- no",
			nil},
		{"missing justification",
			"@@ -1 +1,2 @@\n x\n+// columbo:ignore long-function --   ",
			nil},
		{"no newline marker does not count",
			"@@ -1 +1,2 @@\n x\n\\ No newline at end of file\n+// columbo:ignore feature-envy -- reads the peer's state on purpose",
			[]Suppression{{Path: "a.go", Line: 2, Rule: "feature-envy", Justification: "reads the peer's state on purpose"}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.want, parseSuppressions("a.go", test.patch))
		})
	}
}

func TestMissText(t *testing.T) {
	tests := []struct {
		body string
		want string
		ok   bool
	}{
		{"Columbo missed: long-function in F", " long-function in F", true},
		{"\n  columbo MISSED:data-clump", "data-clump", true},
		{"Columbo missed", "", false},
		{"I think Columbo missed: this", "", false},
		{"", "", false},
	}
	for _, test := range tests {
		got, ok := missText(test.body)
		require.Equal(t, test.ok, ok, test.body)
		require.Equal(t, test.want, got, test.body)
	}
}
