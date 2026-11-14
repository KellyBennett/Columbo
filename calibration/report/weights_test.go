package report_test

import (
	"bytes"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KellyBennett/Columbo/calibration/harness"
	"github.com/KellyBennett/Columbo/calibration/report"
)

const longFunction = "long-function"

// The FAIL and WARN at-limit buckets sum to 6, so the bands weigh 6, 3, 1, and 0.
const export = `{"format":"columbo-ledger-export","version":1,"floor":{"cases":5,"repos":3},
 "excess":[{"name":"at-limit","max_ratio":1},{"name":"near","max_ratio":1.25},{"name":"over","max_ratio":2},{"name":"far"}],
 "rules":[{"smell":"long-function","buckets":[
  {"verdict":"FAIL","suppressed":false,"excess":"at-limit","cases":4,"repos":3},
  {"verdict":"FAIL","suppressed":false,"excess":"near","cases":3,"repos":3},
  {"verdict":"FAIL","suppressed":false,"excess":"over","cases":1,"repos":1},
  {"verdict":"WARN","suppressed":true,"excess":"at-limit","cases":2,"repos":2}]}]}`

func writeFile(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "export.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func ratio(v float64) *float64 { return &v }

func status(killed bool) harness.Status {
	if killed {
		return harness.Killed
	}
	return harness.Survived
}

type recorder struct {
	t     *testing.T
	store *harness.Store
	run   int64
}

func (r recorder) record(parent int64, o harness.Outcome, steps ...string) int64 {
	r.t.Helper()
	for _, op := range steps {
		o.Mutant.Chain = append(o.Mutant.Chain, harness.Step{Operator: op, Site: harness.Site{Package: "example.com/m", Symbol: "example.com/m.Run"}, Dose: 1})
	}
	o.Mutant.Patch = []byte{}
	id, err := r.store.Record(r.run, parent, o)
	if err != nil {
		r.t.Fatal(err)
	}
	return id
}

// evade records a killed inject whose added long-function case has the given ratio, then an evade on top of it.
func (r recorder) evade(subject float64, killed bool) {
	parent := r.record(0, harness.Outcome{Class: harness.Inject, Target: longFunction, Status: harness.Killed,
		Added: []harness.Finding{{Smell: longFunction, Symbol: "example.com/m.Run", CaseID: "C-1", Verdict: "FAIL", Ratio: ratio(subject)}}},
		"inline-function")
	r.record(parent, harness.Outcome{Class: harness.Evade, Target: longFunction, Status: status(killed)}, "inline-function", "split-long-function")
}

func TestWeightedKillRateWeighsEachBandByItsShareOfRealCases(t *testing.T) {
	weights, err := report.LoadWeights(writeFile(t, export))
	if err != nil {
		t.Fatal(err)
	}
	store, err := harness.OpenStore(filepath.Join(t.TempDir(), "calibration.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	baseline := []harness.Finding{{Smell: "feature-envy", Symbol: "example.com/m.Envious", File: "m.go", CaseID: "C-2", Verdict: "FAIL", Ratio: ratio(1)}}
	run, err := store.BeginRun(harness.RunInfo{Module: "example.com/m", Validity: harness.ValidityTest}, baseline)
	if err != nil {
		t.Fatal(err)
	}
	r := recorder{t, store, run}
	for _, killed := range []bool{true, true, false, false} {
		r.evade(1, killed)
	}
	r.evade(12.0/11, true)
	r.evade(1.25, true)
	r.evade(1.1, true)
	r.evade(2.5, true)
	r.record(0, harness.Outcome{Class: harness.Evade, Target: longFunction, Status: harness.Killed}, "redteam-replay")
	r.record(0, harness.Outcome{Class: harness.Inject, Target: longFunction, Status: harness.Survived}, "inline-function")
	cure := harness.Outcome{Class: harness.Cure, Target: "feature-envy", Status: harness.Killed}
	cure.Mutant.Chain = []harness.Step{{Operator: "move-method", Site: harness.Site{Package: "example.com/m", Symbol: "example.com/m.Envious"}, Dose: 1}}
	r.record(0, cure)

	var out bytes.Buffer
	if err := report.WriteWeighted(&out, store.DB(), weights); err != nil {
		t.Fatal(err)
	}
	t.Log("\n" + out.String())

	shapes := table(t, out.String(), "Real case shapes")
	wantShapes := map[string]string{"smell": longFunction, "exported cases": "10", "at-limit": "60% (6)", "near": "30% (3)", "over": "10% (1)", "far": "", "unmeasured": ""}
	if len(shapes) != 1 {
		t.Fatalf("real case shapes has %d rows, want 1", len(shapes))
	}
	check(t, "real case shapes", shapes[0], wantShapes)
	if !strings.Contains(out.String(), "at least 5 cases in at least 3 repositories, so a blank band can still hold real cases that the floor dropped") {
		t.Error("the real case shapes note does not name the export's floor")
	}

	rates := table(t, out.String(), "Weighted kill rate")
	var evades []map[string]string
	byKey := map[string]map[string]string{}
	for _, row := range rates {
		byKey[row["class"]+" "+row["operator"]+" "+row["target"]] = row
		if row["class"] == "evade" {
			evades = append(evades, row)
		}
	}
	if len(evades) != 3 || evades[0]["operator"] != "(all)" || evades[1]["operator"] != "redteam-replay" || evades[2]["operator"] != "split-long-function" {
		t.Fatalf("evade rows %v, want the (all) row and then each operator", evades)
	}
	check(t, "evade split-long-function", evades[2], map[string]string{"scored": "8", "unshaped": "0", "kill rate": "75%", "weighted": "67%", "covered": "90%",
		"at-limit": "2/4", "near": "3/3", "over": "uncovered", "far": "1/1", "unmeasured": ""})
	withheld := map[string]string{"weighted": "withheld", "covered": "", "at-limit": "", "near": "", "over": "", "far": "", "unmeasured": ""}
	check(t, "evade (all)", evades[0], merge(withheld, map[string]string{"scored": "9", "unshaped": "1", "kill rate": "78%"}))
	check(t, "evade redteam-replay", evades[1], merge(withheld, map[string]string{"scored": "1", "unshaped": "1", "kill rate": "100%"}))
	check(t, "inject (all)", byKey["inject (all) long-function"], merge(withheld, map[string]string{"scored": "9", "unshaped": "1", "kill rate": "89%"}))
	check(t, "cure move-method", byKey["cure move-method feature-envy"], map[string]string{"scored": "1", "unshaped": "0",
		"kill rate": "100%", "weighted": "no exported cases", "covered": "", "at-limit": "1/1", "near": ""})
}

func merge(rows ...map[string]string) map[string]string {
	out := map[string]string{}
	for _, row := range rows {
		maps.Copy(out, row)
	}
	return out
}

func check(t *testing.T, name string, row, want map[string]string) {
	t.Helper()
	if row == nil {
		t.Errorf("%s: no row", name)
		return
	}
	for col, w := range want {
		if row[col] != w {
			t.Errorf("%s: %s = %q, want %q", name, col, row[col], w)
		}
	}
}

// table parses the Markdown table under a level-two heading into rows keyed by column.
func table(t *testing.T, markdown, title string) []map[string]string {
	t.Helper()
	_, section, ok := strings.Cut(markdown, "\n## "+title+"\n")
	if !ok {
		t.Fatalf("no %q section", title)
	}
	section, _, _ = strings.Cut(section, "\n## ")
	var cols []string
	var rows []map[string]string
	for _, line := range strings.Split(section, "\n") {
		if !strings.HasPrefix(line, "| ") {
			continue
		}
		cells := strings.Split(strings.TrimSuffix(strings.TrimPrefix(line, "| "), " |"), " | ")
		if cols == nil {
			cols = cells
			continue
		}
		row := map[string]string{}
		for i, c := range cols {
			row[c] = strings.TrimSpace(cells[i])
		}
		rows = append(rows, row)
	}
	return rows
}

func TestLoadWeightsRejectsDocumentsItCannotTrust(t *testing.T) {
	if _, err := report.LoadWeights(writeFile(t, export)); err != nil {
		t.Fatalf("the valid export: %v", err)
	}
	for name, edit := range map[string][2]string{
		"version 2":             {`"version":1`, `"version":2`},
		"another format":        {`"columbo-ledger-export"`, `"columbo-ledger-summary"`},
		"an unknown field":      {`"floor":`, `"total":10,"floor":`},
		"a case key":            {`"repos":2}`, `"repos":2,"case_key":"C-1"}`},
		"an unknown band":       {`"excess":"near"`, `"excess":"huge"`},
		"an unbounded middle":   {`{"name":"near","max_ratio":1.25}`, `{"name":"near"}`},
		"a negative case count": {`"cases":4,`, `"cases":-4,`},
		"a negative repo count": {`"repos":2}`, `"repos":-2}`},
		"a negative floor":      {`"floor":{"cases":5`, `"floor":{"cases":-5`},
		"trailing content":      {`}]}]}`, `}]}]} {}`},
	} {
		body := strings.Replace(export, edit[0], edit[1], 1)
		if body == export {
			t.Fatalf("%s: the edit did not apply", name)
		}
		if _, err := report.LoadWeights(writeFile(t, body)); err == nil {
			t.Errorf("LoadWeights accepted %s", name)
		}
	}
}

func TestReportReadsADatabaseFromBeforeRatiosWithoutWritingIt(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root writes to read-only files")
	}
	path := filepath.Join(t.TempDir(), "calibration.sqlite")
	store, err := harness.OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	run, err := store.BeginRun(harness.RunInfo{Module: "example.com/m", Validity: harness.ValidityTest}, nil)
	if err != nil {
		t.Fatal(err)
	}
	recorder{t, store, run}.evade(1, true)
	if _, err := store.DB().Exec(`ALTER TABLE baseline_findings DROP COLUMN ratio; ALTER TABLE mutant_findings DROP COLUMN ratio`); err != nil {
		t.Fatal(err)
	}
	store.Close()
	if err := os.Chmod(path, 0o444); err != nil {
		t.Fatal(err)
	}
	weights, err := report.LoadWeights(writeFile(t, export))
	if err != nil {
		t.Fatal(err)
	}
	store, err = harness.OpenStore(path)
	if err != nil {
		t.Fatalf("opening a read-only database: %v", err)
	}
	defer store.Close()
	var out bytes.Buffer
	if _, err := report.Write(&out, store.DB(), nil); err != nil {
		t.Fatalf("report: %v", err)
	}
	if err := report.WriteWeighted(&out, store.DB(), weights); err != nil {
		t.Fatalf("weighted report: %v", err)
	}
	for _, row := range table(t, out.String(), "Weighted kill rate") {
		if row["class"] == "evade" && row["unmeasured"] != "1/1" {
			t.Errorf("evade %s: unmeasured = %q, want 1/1 because no finding has a ratio", row["operator"], row["unmeasured"])
		}
	}
	var columns int
	if err := store.DB().QueryRow(`SELECT count(*) FROM pragma_table_info('mutant_findings') WHERE name = 'ratio'`).Scan(&columns); err != nil || columns != 0 {
		t.Errorf("the report added a ratio column (%d, %v)", columns, err)
	}
}
