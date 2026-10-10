package columbo

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRoleMergePreservesUsedInterfaces(t *testing.T) {
	for _, shape := range []string{"if", "switch", "repeated-variants"} {
		for _, named := range []bool{false, true} {
			name := shape + "/anonymous"
			conversion := "(interface{Send(string) error})"
			declaration := ""
			if named {
				name = shape + "/named"
				conversion = "Sender"
				declaration = "type Sender interface{Send(string) error}\n"
			}
			t.Run(name, func(t *testing.T) {
				body := `func deliver(flag bool) error {if flag {return Email{}.Send("x")} else {return SMS{}.Send("x")}}`
				config := selectionConfig()
				if shape == "switch" {
					body = roleSwitch
				}
				if shape == "repeated-variants" {
					config = roleVariantConfig()
					body = `type Kind int
const(Mail Kind=iota;Text)
func first(k Kind)error{switch k{case Mail:return Email{}.Send("x");case Text:return SMS{}.Send("x")};return nil}
func second(k Kind)error{switch k{case Mail:return Email{}.Send("y");case Text:return SMS{}.Send("y")};return nil}`
				}
				body = strings.ReplaceAll(body, "Email{}.Send", conversion+"(Email{}).Send")
				body = strings.ReplaceAll(body, "SMS{}.Send", conversion+"(SMS{}).Send")
				role := oneRole(t, roleFixture(t, rolePrelude+declaration+body, config))
				require.Len(t, role.UsedInterfaces, 1)
				require.Contains(t, role.ExactInterfaces, role.UsedInterfaces[0])
				require.Equal(t, "existing role", role.Classification)
				if named {
					require.Equal(t, []string{"fixture.Sender"}, role.UsedInterfaces)
				}
			})
		}
	}
}

func TestRoleMergeUnionAndFiltering(t *testing.T) {
	tests := []struct {
		name, prelude, declarations, body string
		used, compatible                  []string
	}{
		{"one-arm-use", rolePrelude, `type Sender interface{Send(string) error}`, `func deliver(flag bool) error {if flag{return Sender(Email{}).Send("x")}else{return SMS{}.Send("x")}}`, []string{"fixture.Sender"}, nil},
		{"distinct-interfaces-union", rolePrelude, `type First interface{Send(string) error};type Second interface{Send(string) error}`, `func deliver(flag bool) error {if flag{return First(Email{}).Send("x")}else{return Second(SMS{}).Send("x")}}`, []string{"fixture.First", "fixture.Second"}, nil},
		{"used-superset", rolePrelude, `type Sender interface{Send(string) error;Health() error}`, `func deliver(flag bool) error {if flag{return Sender(Email{}).Send("x")}else{return SMS{}.Send("x")}}`, []string{"fixture.Sender"}, []string{"fixture.Sender"}},
		{"not-all-players-implement", strings.ReplaceAll(rolePrelude, "func (SMS) Health() error {return nil}", ""), `type Sender interface{Send(string) error;Health() error}`, `func deliver(flag bool) error {if flag{return Sender(Email{}).Send("x")}else{return SMS{}.Send("x")}}`, nil, nil},
		{"unused-superset", rolePrelude, `type Sender interface{Send(string) error;Health() error}`, roleSwitch, nil, []string{"fixture.Sender"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			role := oneRole(t, roleFixture(t, test.prelude+test.declarations+"\n"+test.body, selectionConfig()))
			require.Equal(t, test.used, role.UsedInterfaces)
			require.Equal(t, test.compatible, role.CompatibleInterfaces)
			require.Equal(t, []string{"Send(string) error"}, role.Messages)
			if len(test.used) > 0 {
				require.Equal(t, "existing role", role.Classification)
			} else {
				require.Equal(t, "inferred role", role.Classification)
			}
		})
	}
}
