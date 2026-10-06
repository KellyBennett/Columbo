package columbo

import (
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

const rolePrelude = `package fixture
type Email struct{}
type SMS struct{}
func (Email) Send(msg string) error {return nil}
func (SMS) Send(m string) error {return nil}
func (Email) Health() error {return nil}
func (SMS) Health() error {return nil}
`
const roleBranches = `func deliver(email bool) error {if email {return Email{}.Send("x")};return SMS{}.Send("x")}`
const roleSwitch = `func deliver(email bool) error {switch email {case true:return Email{}.Send("x");default:return SMS{}.Send("x")}}`

func roleFixture(t *testing.T, src string, config Config) Report {
	t.Helper()
	h := &testHarness{T: t}
	return h.investigate(h.fixture(src), config)
}
func oneRole(t *testing.T, r Report) RoleCandidate {
	t.Helper()
	require.Len(t, r.Roles, 1)
	return r.Roles[0]
}
func TestRoleAcceptance(t *testing.T) {
	tests := []struct {
		name, extra, body string
		messages          []string
		exact, compatible string
	}{
		{"role-selected-common-message", "", roleSwitch, []string{"Send(string) error"}, "", ""},
		{"role-minimal-surface", "", roleSwitch, []string{"Send(string) error"}, "", ""},
		{"role-multiple-messages", "", `func deliver(email bool){if email {Email{}.Send("x");Email{}.Health()}else{SMS{}.Send("x");SMS{}.Health()}}`, []string{"Health() error", "Send(string) error"}, "", ""},
		{"role-existing-interface", `type Sender interface {Send(string) error}`, roleSwitch, []string{"Send(string) error"}, "fixture.Sender", ""},
		{"role-existing-superset", `type Sender interface {Send(string) error;Health() error}`, roleSwitch, []string{"Send(string) error"}, "", "fixture.Sender"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			report := roleFixture(t, rolePrelude+test.extra+"\n"+test.body, selectionConfig())
			role := oneRole(t, report)
			require.Equal(t, test.messages, role.Messages)
			require.Equal(t, "strong", role.Confidence)
			require.Empty(t, report.Cases)
			require.Equal(t, Summary{}, report.Summary)
			if test.exact != "" {
				require.Contains(t, role.ExactInterfaces, test.exact)
				require.Equal(t, "existing role", role.Classification)
			}
			if test.compatible != "" {
				require.Contains(t, role.CompatibleInterfaces, test.compatible)
				require.Equal(t, "inferred role", role.Classification)
			}
		})
	}
}
func TestRoleSelectionEvidence(t *testing.T) {
	report := roleFixture(t, selectionPrelude+selectionFunction(selectSender+`sender.Send("x")`), selectionConfig())
	role := oneRole(t, report)
	require.Equal(t, []string{"*fixture.Email", "*fixture.SMS"}, role.Implementations)
	require.Equal(t, []string{"Send(string) error"}, role.Messages)
	require.Equal(t, []string{"fixture.Sender"}, role.UsedInterfaces)
	require.Equal(t, []string{report.Cases[0].ID}, role.CaseIDs)
	require.Equal(t, 1, report.Summary.Failed)
	require.NotEmpty(t, role.Receipts)
}
func TestRoleIdentity(t *testing.T) {
	baseline := oneRole(t, roleFixture(t, rolePrelude+roleSwitch, selectionConfig()))
	variants := []struct{ name, source string }{
		{"role-renamed-parameters", strings.ReplaceAll(rolePrelude, "msg string", "renamed string") + roleSwitch},
		{"role-add-unrelated-method", rolePrelude + `func (Email) Debug(){};func (SMS) Debug(){}` + "\n" + roleSwitch},
		{"aliases", rolePrelude + `type Alias = Email` + "\n" + strings.ReplaceAll(roleSwitch, "Email{}", "Alias{}")},
		{"locals", rolePrelude + strings.ReplaceAll(roleSwitch, "email", "renamed")},
		{"role-interface-added", rolePrelude + `type Sender interface {Send(string) error}` + "\n" + roleSwitch},
		{"interface-used", rolePrelude + `type Sender interface {Send(string) error}
func deliver(email bool) error {var sender Sender;if email {sender=Email{}}else{sender=SMS{}};return sender.Send("x")}`},
		{"constructors", rolePrelude + `func one() Email{return Email{}};func two() SMS{return SMS{}}` + "\n" + strings.ReplaceAll(strings.ReplaceAll(roleSwitch, "Email{}", "one()"), "SMS{}", "two()")},
	}
	for _, test := range variants {
		t.Run(test.name, func(t *testing.T) {
			role := oneRole(t, roleFixture(t, test.source, selectionConfig()))
			require.Equal(t, baseline.ID, role.ID)
			require.Equal(t, baseline.Messages, role.Messages)
			if strings.Contains(test.name, "interface") {
				require.Equal(t, "existing role", role.Classification)
			}
		})
	}
}
func TestRolePointerPromotedGeneric(t *testing.T) {
	source := `package fixture
type Base[T any] struct{}
func (*Base[T]) Send(T) error{return nil}
type Email struct{Base[string]}
type SMS struct{Base[string]}
func deliver(flag bool) error {a:=Email{};b:=SMS{};if flag{return a.Send("x")}else{return b.Send("x")}}
type ValueSender interface{Send(string) error}
`
	role := oneRole(t, roleFixture(t, source, selectionConfig()))
	require.Equal(t, []string{"*fixture.Email", "*fixture.SMS"}, role.Implementations)
	require.Equal(t, []string{"Send(string) error"}, role.Messages)
	require.Contains(t, role.ExactInterfaces, "fixture.ValueSender")
}
func TestRoleNegativeEvidence(t *testing.T) {
	sources := []struct{ name, source string }{
		{"role-unrelated-shared-method", rolePrelude + `func one(){Email{}.Health()};func two(){SMS{}.Health()}`},
		{"mismatched-signatures", strings.Replace(rolePrelude, "Send(m string)", "Send(m int)", 1) + strings.Replace(roleSwitch, `SMS{}.Send("x")`, `SMS{}.Send(1)`, 1)},
		{"method-values", rolePrelude + `func deliver(flag bool){if flag {_=Email{}.Send}else{_=SMS{}.Send}}`},
		{"runtime-assertions", rolePrelude + `func deliver(flag bool,a,b any){if flag {a.(Email).Send("x")}else{b.(SMS).Send("x")}}`},
		{"same-player", rolePrelude + strings.ReplaceAll(roleSwitch, "SMS{}", "Email{}")},
		{"unobserved", rolePrelude + `func deliver(flag bool) any {if flag{return Email{}}else{return SMS{}}}`},
		{"nested-closures", rolePrelude + `func deliver(flag bool){if flag {_=func(){Email{}.Send("x")}}else{_=func(){SMS{}.Send("x")}}}`},
	}
	for _, test := range sources {
		t.Run(test.name, func(t *testing.T) { require.Empty(t, roleFixture(t, test.source, selectionConfig()).Roles) })
	}
}
func roleVariantConfig() Config {
	c := quiet()
	c.Severity[variantSmell] = "fail"
	c.Counts["repeated-variant-sites"] = 2
	c.Counts["repeated-variant-variants"] = 2
	return c
}

const roleVariants = `type Kind int
const (Mail Kind=iota;Text)
func send(k Kind)error{switch k{case Mail:return Email{}.Send("x");case Text:return SMS{}.Send("x")};return nil}
func health(k Kind)error{switch k{case Mail:return Email{}.Health();case Text:return SMS{}.Health()};return nil}`

func TestRoleRepeatedVariants(t *testing.T) {
	report := roleFixture(t, rolePrelude+roleVariants, roleVariantConfig())
	role := oneRole(t, report)
	require.Equal(t, []string{"Health() error", "Send(string) error"}, role.Messages)
	require.Equal(t, []string{report.Cases[0].ID}, role.CaseIDs)
}
func TestRoleAmbiguousVariantMap(t *testing.T) {
	source := strings.Replace(rolePrelude+roleVariants, "case Mail:return Email{}.Health();case Text:return SMS{}.Health()", "case Mail:return SMS{}.Health();case Text:return Email{}.Health()", 1)
	report := roleFixture(t, source, roleVariantConfig())
	require.Len(t, report.Cases, 1)
	require.Empty(t, report.Roles)
}
func TestRoleEquivalentNamedTypesAreNotMessages(t *testing.T) {
	source := `package fixture
type A struct{};type B struct{};type X string;type Y string
func(A) Send(X){};func(B) Send(Y){}
func deliver(flag bool){if flag {A{}.Send("x")}else{B{}.Send("x")}}`
	require.Empty(t, roleFixture(t, source, selectionConfig()).Roles)
}

func TestRoleLoadedDependencyInterface(t *testing.T) {
	source := `package fixture
import "io"
var _ io.Closer
type A struct{};type B struct{}
func(A) Close() error{return nil};func(B) Close() error{return nil}
func close(flag bool)error{if flag{return A{}.Close()}else{return B{}.Close()}}`
	role := oneRole(t, roleFixture(t, source, selectionConfig()))
	require.Contains(t, role.ExactInterfaces, "io.Closer")
}
func TestRoleVariadicDistinction(t *testing.T) {
	source := `package fixture
type A struct{};type B struct{}
func(A) Send(...string){};func(B) Send([]string){}
func deliver(flag bool){if flag {A{}.Send("x")}else{B{}.Send([]string{"x"})}}`
	require.Empty(t, roleFixture(t, source, selectionConfig()).Roles)
}
func TestRoleRepeatedIfArms(t *testing.T) {
	source := rolePrelude + `type Kind int
const (Mail Kind=iota;Text)
func send(k Kind)error{if k==Mail{return Email{}.Send("x")}else if k==Text{return SMS{}.Send("x")};return nil}
func health(k Kind)error{if k==Mail{return Email{}.Health()}else if k==Text{return SMS{}.Health()};return nil}`
	require.Equal(t, []string{"Health() error", "Send(string) error"}, oneRole(t, roleFixture(t, source, roleVariantConfig())).Messages)
}
func TestRoleMovedMethods(t *testing.T) {
	baseline := oneRole(t, roleFixture(t, rolePrelude+roleSwitch, selectionConfig()))
	h := &testHarness{T: t}
	dir := h.fixture(rolePrelude)
	h.write(dir, "consumer.go", "package fixture\n"+roleSwitch)
	moved := oneRole(t, h.investigate(dir, selectionConfig()))
	require.Equal(t, baseline.ID, moved.ID)
}
func TestRoleNoVerdictWhenSelectionDisabled(t *testing.T) {
	report := roleFixture(t, rolePrelude+roleSwitch, quiet())
	require.Empty(t, report.Roles)
	require.Empty(t, report.Cases)
	require.Equal(t, Summary{}, report.Summary)
}

func TestRoleInlineInterfaceConversion(t *testing.T) {
	baseline := oneRole(t, roleFixture(t, rolePrelude+roleSwitch, selectionConfig()))
	source := rolePrelude + `type Sender interface{Send(string)error}` + "\n" + strings.ReplaceAll(strings.ReplaceAll(roleSwitch, "Email{}", "Sender(Email{})"), "SMS{}", "Sender(SMS{})")
	role := oneRole(t, roleFixture(t, source, selectionConfig()))
	require.Equal(t, baseline.ID, role.ID)
	require.Equal(t, "existing role", role.Classification)
}
