package columbo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const selectionPrelude = `package fixture
type Sender interface { Send(string) error; Close() error }
type Email struct{}
func (Email) Send(string) error { return nil }
func (Email) Close() error { return nil }
type SMS struct{}
func (*SMS) Send(string) error { return nil }
func (*SMS) Close() error { return nil }
type Test struct{}
func (Test) Send(string) error { return nil }
func (Test) Close() error { return nil }
func arbitrary() *Email { return &Email{} }
func concrete() *SMS { return &SMS{} }
func hidden() Sender { return &Email{} }
func consume(Sender) {}
`
const selectSender = `var sender Sender; if email { sender = &Email{} } else { sender = &SMS{} };`

func selectionConfig() Config {
	config := quiet()
	config.Severity[selectionSmell] = "fail"
	return config
}
func selectionFunction(body string) string {
	return "func Send(email bool) error {" + body + "; return nil }"
}
func TestSelectionAcceptance(t *testing.T) {
	cases := []struct {
		name, body string
		sites      int
	}{
		{"selection-switch-use", `var sender Sender; switch email {case true: sender = &Email{}; case false: sender = &SMS{}}; sender.Send("x")`, 1},
		{"selection-if-use", selectSender + `sender.Send("x")`, 1},
		{"selection-baseline-override", `var sender Sender = &Email{}; if email {sender = &SMS{}}; sender.Send("x")`, 1},
		{"selection-three-implementations", `var sender Sender; if email {sender=&Email{}} else if len("x")>0 {sender=&SMS{}} else {sender=Test{}}; sender.Send("x")`, 1},
		{"selection-same-implementation", `var sender Sender; if email {sender=Email{}} else {sender=&Email{}}; sender.Send("x")`, 0},
		{"selection-constructor-name-irrelevant", `var sender Sender; if email {sender=arbitrary()} else {sender=concrete()}; sender.Send("x")`, 1},
		{"selection-concrete-return-helper", `var sender Sender=arbitrary(); if email {sender=concrete()}; sender.Send("x")`, 1},
		{"selection-interface-return-helper", `var sender Sender; if email {sender=hidden()} else {sender=concrete()}; sender.Send("x")`, 0},
		{"selection-use-multiple-messages", selectSender + `sender.Send("x"); sender.Close()`, 1},
		{"selection-local-alias", selectSender + `selected:=sender; active:=Sender((selected)); active.Send("x")`, 1},
		{"selection-overwritten-before-use", selectSender + `sender=Test{};sender.Send("x")`, 0},
		{"selection-alias-survives-original-overwrite", selectSender + `active:=sender;sender=Test{};active.Send("x")`, 1},
		{"selection-empty-interface", `var sender any; if email {sender=&Email{}} else {sender=&SMS{}};consume(sender.(Sender))`, 0},
		{"selection-anonymous-role", `var sender interface{Send(string) error}; if email {sender=&Email{}} else {sender=&SMS{}};sender.Send("x")`, 1},
		{"selection-nested-literal-boundary", selectSender + `_ = func(){sender.Send("x")}`, 0},
		{"selection-nested-selection-boundary", `var sender Sender;fn:=func(){if email {sender=&Email{}} else {sender=&SMS{}}};fn();sender.Send("x")`, 0},
		{"selection-pass-only", selectSender + `consume(sender)`, 0},
		{"selection-unknown-origin", `sender:=hidden();if email {sender=&SMS{}};sender.Send("x")`, 0},
		{"selection-unknown-third-origin", `var sender Sender;switch{case email:sender=&Email{};case !email:sender=&SMS{};default:sender=hidden()};sender.Send("x")`, 0},
		{"selection-nil-origin", `var sender Sender; if email {sender=&Email{}};sender.Send("x")`, 0},
		{"selection-nil-third-origin", `var sender Sender;switch {case email:sender=&Email{};case !email:sender=&SMS{};default:sender=nil};sender.Send("x")`, 1},
		{"selection-factory-clears", `sender:=hidden();sender.Send("x")`, 0},
		{"selection-go-defer", selectSender + `go sender.Send("x"); defer sender.Close()`, 1},
		{"selection-method-value-only", selectSender + `_ = sender.Send`, 0},
		{"selection-early-return", `var sender Sender; if email {sender=&Email{};return nil} else {sender=&SMS{}};sender.Send("x")`, 0},
		{"selection-switch-break", `var sender Sender; switch email {case true:sender=&Email{};break;case false:sender=&SMS{};break};sender.Send("x")`, 1},
		{"selection-fallthrough", `var sender Sender;switch email {case true:sender=&Email{};fallthrough;case false:sender=&SMS{}};sender.Send("x")`, 0},
		{"selection-type-switch", `var sender Sender;switch any(email).(type){case bool:sender=&Email{};default:sender=&SMS{}};sender.Send("x")`, 0},
		{"selection-goto", `var sender Sender; if email {sender=&Email{};goto done};sender=&SMS{};done:sender.Send("x")`, 0},
		{"selection-independent-twice", selectSender + `sender.Send("x"); if email {sender=&Email{}}else{sender=&SMS{}};sender.Close()`, 2},
		{"selection-unrelated-conditional", selectSender + `if email {println(1)} else {println(2)};sender.Send("x")`, 1},
		{"selection-overwrite-alias", selectSender + `active:=sender;active=Test{};active.Send("x")`, 0},
		{"selection-pointer-escape", selectSender + `ptr:=&sender;*ptr=Test{};sender.Send("x")`, 0},
		{"selection-simultaneous-assignment", selectSender + `other:=Sender(Test{});sender,other=other,sender;other.Send("x")`, 1},
		{"selection-loop-local", `for range 2 {` + selectSender + `sender.Send("x")}`, 1},
		{"selection-branch-use-before-merge", `var sender Sender;if email {sender=&Email{};sender.Send("x")}else{sender=&SMS{};sender.Send("x")}`, 0},
		{"selection-provisional-policy", selectSender + `// Intentionally choose a short-lived transport here.
sender.Send("x")`, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := &testHarness{T: t}
			report := h.investigate(h.fixture(selectionPrelude+selectionFunction(tc.body)), selectionConfig())
			if tc.sites == 0 {
				require.Empty(t, report.Cases)
				return
			}
			c := h.one(report, selectionSmell)
			require.Equal(t, tc.sites, h.clueValue(c, "selection-use-sites"))
			require.Equal(t, []PolicyReview{selectionPolicy}, c.PolicyReviews)
			require.Equal(t, "FAIL", c.Verdict)
		})
	}
}
func TestSelectionBoundaries(t *testing.T) {
	cases := []struct{ name, source string }{
		{"return-only", `func New(email bool) Sender {` + selectSender + `return sender}`},
		{"factory-configure-return", `func New(email bool)(Sender,error){` + selectSender + `if err:=sender.Send("setup");err!=nil{return nil,err};return sender,nil}`},
		{"named-return", `func New(email bool)(sender Sender){if email{sender=&Email{}}else{sender=&SMS{}};sender.Send("setup");return}`},
		{"alias-return", `func New(email bool) Sender {` + selectSender + `sender.Send("setup");active:=sender;return active}`},
		{"injection-clears", `func Send(sender Sender) error{return sender.Send("x")}`},
		{"init-exempt", `func init(){email:=true;` + selectSender + `sender.Send("x")}`},
		{"main-exempt", `func main(){email:=true;` + selectSender + `sender.Send("x")}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := &testHarness{T: t}
			source := selectionPrelude + tc.source
			if tc.name == "main-exempt" {
				source = strings.Replace(source, "package fixture", "package main", 1)
			}
			require.Empty(t, h.investigate(h.fixture(source), selectionConfig()).Cases)
		})
	}
}
func TestSelectionThreshold(t *testing.T) {
	for _, minimum := range []int64{2, 3, 4} {
		t.Run(string(rune('0'+minimum)), func(t *testing.T) {
			h := &testHarness{T: t}
			config := selectionConfig()
			config.Counts["selection-use-implementations"] = minimum
			body := `var sender Sender;if email{sender=&Email{}}else if !email{sender=&SMS{}}else{sender=Test{}};sender.Send("x")`
			report := h.investigate(h.fixture(selectionPrelude+selectionFunction(body)), config)
			if minimum > 3 {
				require.Empty(t, report.Cases)
				return
			}
			c := h.one(report, selectionSmell)
			require.Equal(t, 3, h.clueValue(c, "selected-implementation-count"))
		})
	}
}
func TestSelectionIdentity(t *testing.T) {
	bodies := []string{selectSender + `sender.Send("x")`,
		`var renamed Sender;switch email{case true:renamed=&Email{};default:renamed=&SMS{}};renamed.Send("x")`,
		`var sender Sender;if email{sender=&Email{}}else if !email{sender=&SMS{}}else{sender=Test{}};sender.Send("x");sender.Close()`,
	}
	var id string
	for i, body := range bodies {
		h := &testHarness{T: t}
		dir := h.fixture(selectionPrelude + selectionFunction(body))
		if i == 1 {
			require.NoError(t, os.Rename(filepath.Join(dir, "source.go"), filepath.Join(dir, "moved.go")))
		}
		c := h.one(h.investigate(dir, selectionConfig()), selectionSmell)
		if id == "" {
			id = c.ID
		}
		require.Equal(t, id, c.ID)
	}
}

func TestSelectionGoldens(t *testing.T) {
	for _, severity := range []string{"fail", "warn", "off", "suppressed"} {
		t.Run(severity, func(t *testing.T) {
			h := &testHarness{T: t}
			source := selectionPrelude
			if severity == "suppressed" {
				source += "//columbo:ignore selection-use-coupling -- intentional transport lifecycle in this fixture\n"
			}
			source += selectionFunction(selectSender + `selected:=sender;active:=selected;active.Send("x");defer active.Close()`)
			config := selectionConfig()
			if severity != "suppressed" {
				config.Severity[selectionSmell] = severity
			}
			report := h.investigate(h.fixture(source), config)
			for i := range report.Cases {
				report.Cases[i].Receipts = append(report.Cases[i].Receipts, History{Kind: "history", Commit: strings.Repeat("a", 40), CommittedAt: 1, Files: sortedSet(report.Cases[i].sourceFiles())})
			}
			h.snapshotGoldens(report, filepath.Join("testdata", "selection-"+severity))
		})
	}
}
func TestSelectionExactEvidence(t *testing.T) {
	h := &testHarness{T: t}
	source := selectionPrelude + selectionFunction(selectSender+`unused:=sender;_ = unused;selected:=sender;active:=selected;active.Send("x");active.Close()`)
	dir := h.fixture(source)
	report := h.investigate(dir, selectionConfig())
	c := h.one(report, selectionSmell)
	require.Equal(t, 2, h.clueValue(c, "selected-use-count"))
	require.Equal(t, []string{"fixture.Email", "fixture.SMS"}, h.clueValue(c, "selected-implementation-set"))
	require.Equal(t, []string{"Close() error", "Send(string) error"}, h.clueValue(c, "selected-message-set"))
	kinds := map[string][]string{}
	for _, r := range c.Receipts {
		if s, ok := r.(Source); ok {
			kinds[s.Kind] = append(kinds[s.Kind], source[s.StartOffset:s.EndOffset])
		}
	}
	require.ElementsMatch(t, []string{"&Email{}", "&SMS{}"}, kinds["selection-origin"])
	require.ElementsMatch(t, []string{"selected:=sender", "active:=selected"}, kinds["selection-flow"])
	require.ElementsMatch(t, []string{`active.Send("x")`, `active.Close()`}, kinds["selected-message"])
	require.Equal(t, canonical(report), canonical(h.investigate(dir, selectionConfig())))
	db := h.snapshot(report)
	require.Equal(t, 0, h.sqlCount(db, `SELECT COUNT(*) FROM clues c WHERE c.case_id=? AND NOT EXISTS (SELECT 1 FROM clue_receipts r WHERE r.clue_id=c.id)`, c.ID))
	text, exit, err := RenderSnapshot(testDatabaseQueries(db), "report.sqlite")
	require.NoError(t, err)
	require.Equal(t, 1, exit)
	for _, part := range []string{"fixture.Sender", "fixture.Email", "fixture.SMS", "selection origin source.go:", "selection flow source.go:", "selected message source.go:", "SUC-001", "Send(string) error"} {
		require.Contains(t, string(text), part)
	}
}
func TestSelectionGenericAndAliasIdentity(t *testing.T) {
	h := &testHarness{T: t}
	source := `package fixture
type Role[T any] interface{Send(T)}
type First[T any] struct{}
func(First[T]) Send(T){}
type Second[T any] struct{}
func(Second[T]) Send(T){}
type Alias = First[int]
type RoleAlias = Role[int]
func Send(choose bool){var role RoleAlias;if choose{role=Alias{}}else{role=Second[int]{}};role.Send(1)}
func Unfixed[T any](choose bool,v T){var role Role[T];if choose{role=First[T]{}}else{role=Second[T]{}};role.Send(v)}
`
	c := h.one(h.investigate(h.fixture(source), selectionConfig()), selectionSmell)
	require.Equal(t, []string{"fixture.Role[int]"}, h.clueValue(c, "selected-role"))
	require.Equal(t, []string{"fixture.First[int]", "fixture.Second[int]"}, h.clueValue(c, "selected-implementation-set"))
}
func TestSelectionRuleIndependence(t *testing.T) {
	h := &testHarness{T: t}
	config := selectionConfig()
	config.Severity[variantSmell] = "fail"
	config.Severity["duplicate-code"] = "fail"
	source := selectionPrelude + selectionFunction(selectSender+`sender.Send("x")`)
	require.Len(t, h.investigate(h.fixture(source), config).Cases, 1)
	source += `
type Kind int;const(A Kind=iota;B)
func One(k Kind){switch k{case A:println(1);case B:println(2)}}
func Two(k Kind){if k==A{return}else if k==B{println(0)}}`
	require.Len(t, h.investigate(h.fixture(source), config).Cases, 2)
}

func TestSelectionAdditionalFlows(t *testing.T) {
	cases := []struct {
		name, body string
		want       int
	}{
		{"panic-path-not-reaching", `var sender Sender;if email{sender=&Email{};panic("stop")}else{sender=&SMS{}};sender.Send("x")`, 0},
		{"read-only-closure-does-not-erase-local-use", selectSender + `_ = func(){sender.Send("closure")};sender.Send("local")`, 1},
		{"interface-method-expression", selectSender + `Sender.Send(sender,"x")`, 1},
		{"interface-conversion-receiver", selectSender + `interface{Send(string) error}(sender).Send("x")`, 1},
		{"partially-overwritten-three-origins", `var sender Sender;if email{sender=&Email{}}else{sender=&SMS{}};if email{sender=Test{}};sender.Send("x")`, 1},
		{"same-role-two-values-one-site", `var first,second Sender;if email{first=Email{};second=&SMS{}}else{first=&SMS{};second=Email{}};first.Send("x");second.Close()`, 1},
		{"normal-main-name", `email=true;` + selectSender + `sender.Send("x")`, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := &testHarness{T: t}
			r := h.investigate(h.fixture(selectionPrelude+selectionFunction(tc.body)), selectionConfig())
			require.Len(t, r.Cases, tc.want)
		})
	}
}
func TestSelectionTwoRoles(t *testing.T) {
	h := &testHarness{T: t}
	source := selectionPrelude + `type Closer interface{Close()error}
func Send(email bool){` + selectSender + `sender.Send("x");var closer Closer;if email{closer=Email{}}else{closer=&SMS{}};closer.Close()}`
	require.Len(t, h.investigate(h.fixture(source), selectionConfig()).Cases, 2)
}
func TestSelectionRVDComposition(t *testing.T) {
	h := &testHarness{T: t}
	config := selectionConfig()
	config.Severity[variantSmell] = "fail"
	prefix := selectionPrelude + `type Kind int;const(First Kind=iota;Second)
`
	body := `(kind Kind){var sender Sender;switch kind{case First:sender=Email{};case Second:sender=&SMS{}};sender.Send("x")}`
	source := prefix + "func One" + body + "\nfunc Two" + body
	report := h.investigate(h.fixture(source), config)
	require.Len(t, report.Cases, 3)
	factory := `func choose(kind Kind) Sender {switch kind{case First:return Email{};case Second:return &SMS{}};return nil}
func One(kind Kind){sender:=choose(kind);sender.Send("x")}
func Two(kind Kind){sender:=choose(kind);sender.Send("x")}`
	require.Empty(t, h.investigate(h.fixture(prefix+factory), config).Cases)
}

func TestSelectionConfiguration(t *testing.T) {
	require.Equal(t, "fail", Defaults().Severity[selectionSmell])
	require.Equal(t, int64(2), Defaults().Counts["selection-use-implementations"])
	for _, value := range []string{"0", "1", "-1", "2.5", "null", "true", "\"2\""} {
		_, err := decodeConfig([]byte("thresholds: {selection-use-implementations: "+value+"}"), Defaults())
		require.Error(t, err, value)
	}
	for _, value := range []string{"2", "3"} {
		_, err := decodeConfig([]byte("thresholds: {selection-use-implementations: "+value+"}"), Defaults())
		require.NoError(t, err, value)
	}
}

func TestSelectionLoops(t *testing.T) {
	cases := []struct {
		name, body string
		want       int
	}{
		{"selection-reaches-loop-exit", `var sender Sender;for email {if email{sender=Email{}}else{sender=&SMS{}};break};sender.Send("x")`, 1},
		{"loop-overwrite-kills-selection", selectSender + `for {sender=Test{};break};sender.Send("x")`, 0},
		{"loop-post-use", `var sender Sender;for i:=0;i<2;sender.Close(){if email{sender=Email{}}else{sender=&SMS{}};i++}`, 1},
		{"loop-carried-use", `var sender Sender;for email {if sender!=nil{sender.Send("x")};if email{sender=Email{}}else{sender=&SMS{}}}`, 1},
		{"loop-baseline-override", `var sender Sender=Email{};for email {if email{sender=&SMS{}};sender.Send("x")}`, 1},
		{"nonterminating-loop-no-use", `var sender Sender;for {if email{sender=Email{}}else{sender=&SMS{}}};sender.Send("x")`, 0},
		{"loop-unknown-origin", `sender:=hidden();for email{if email{sender=&SMS{}};sender.Send("x")}`, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := &testHarness{T: t}
			r := h.investigate(h.fixture(selectionPrelude+selectionFunction(tc.body)), selectionConfig())
			require.Len(t, r.Cases, tc.want)
		})
	}
}

func TestSelectionDynamicOrigins(t *testing.T) {
	cases := []struct{ pkg, origin string }{
		{"unsafe", `(*Email)(unsafe.Pointer(&Email{}))`},
		{"reflect", `reflect.New(reflect.TypeOf(Email{})).Interface().(*Email)`},
	}
	for _, tc := range cases {
		t.Run(tc.pkg, func(t *testing.T) {
			h := &testHarness{T: t}
			source := strings.Replace(selectionPrelude, "package fixture", "package fixture\nimport \""+tc.pkg+"\"", 1)
			source += selectionFunction(`first:=` + tc.origin + `;second:=first;var sender Sender;if email{sender=second}else{sender=&SMS{}};sender.Send("x")`)
			require.Empty(t, h.investigate(h.fixture(source), selectionConfig()).Cases)
		})
	}
}
func TestSelectionAnyReturnBoundary(t *testing.T) {
	h := &testHarness{T: t}
	source := selectionPrelude + `func New(email bool) any {` + selectSender + `sender.Send("setup");var result any=sender;return result}`
	require.Empty(t, h.investigate(h.fixture(source), selectionConfig()).Cases)
}

func TestSelectionAlternativesMustUseDifferentPaths(t *testing.T) {
	h := &testHarness{T: t}
	source := selectionPrelude + selectionFunction(selectSender+`var active Sender;if email{active=sender};active.Send("x")`)
	c := h.one(h.investigate(h.fixture(source), selectionConfig()), selectionSmell)
	require.Equal(t, 1, h.clueValue(c, "selection-use-sites"), "copying an existing selection on one path is not another concrete-player decision")
}
