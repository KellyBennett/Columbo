package columbo

import (
	"github.com/stretchr/testify/require"
	"testing"
)

const mutationPrelude = `package fixture
 type Account struct { Balance, Limit int }
 type Status string
 const Pending Status="pending"
 const Running Status="running"
 const Waiting Status="pending"
 type Job struct { Status Status; Ready bool }
 func debit(account *Account,amount int){if account.Balance>=amount{account.Balance-=amount}}
 func start(job *Job){if job.Status==Pending{job.Status=Running}}
`

func TestGuardedMutationCorrespondingInputs(t *testing.T) {
	tests := []struct {
		name, source string
		groups       int
	}{
		{"account variable amount", `func withdraw(other *Account,quantity int){if other.Balance>=quantity{other.Balance-=quantity}}`, 1},
		{"assignment arithmetic", `func withdraw(other *Account,quantity int){if quantity<=other.Balance{other.Balance=other.Balance-quantity}}`, 1},
		{"different dependent input", `func withdraw(other *Account,quantity,fee int){if other.Balance>=quantity{other.Balance-=fee}}`, 0},
		{"reversed subtraction", `func withdraw(other *Account,quantity int){if other.Balance>=quantity{other.Balance=quantity-other.Balance}}`, 0},
		{"different comparison", `func withdraw(other *Account,quantity int){if other.Balance>quantity{other.Balance-=quantity}}`, 0},
		{"different operation", `func withdraw(other *Account,quantity int){if other.Balance>=quantity{other.Balance+=quantity}}`, 0},
		{"different subject", `func withdraw(other,second *Account,quantity int){if other.Balance>=quantity{second.Balance-=quantity}}`, 0},
		{"different field", `func withdraw(other *Account,quantity int){if other.Limit>=quantity{other.Limit-=quantity}}`, 0},
		{"constant versus parameter", `func withdraw(other *Account){if other.Balance>=10{other.Balance-=10}}`, 0},
		{"local alias input", `func withdraw(other *Account,quantity int){amount:=quantity;if other.Balance>=amount{other.Balance-=amount}}`, 0},
		{"shadowed input", `func withdraw(other *Account,quantity int){for quantity:=0;quantity<1;quantity++{if other.Balance>=quantity{other.Balance-=quantity}}}`, 0},
		{"effect after check", `func check(a *Account)bool{a.Balance=0;return true};func withdraw(other *Account,quantity int){if other.Balance>=quantity&&check(other){other.Balance-=quantity}}`, 0},
		{"effect in update", `func amount()int{return 1};func withdraw(other *Account,quantity int){if other.Balance>=quantity{other.Balance-=amount()}}`, 0},
		{"or check", `func withdraw(other *Account,quantity int){if other.Balance>=quantity||other.Limit>0{other.Balance-=quantity}}`, 0},
		{"compound context", `func withdraw(other *Account,quantity int){if other.Limit>0&&other.Balance>=quantity{other.Balance-=quantity}}`, 1},
		{"status assignment", `func launch(task *Job){if task.Status==Pending{task.Status=Running}}`, 1},
		{"different named constant same value", `func launch(task *Job){if task.Status==Waiting{task.Status=Running}}`, 0},
		{"literal versus named state", `func launch(task *Job){if task.Status=="pending"{task.Status=Running}}`, 0},
		{"different assigned constant", `func launch(task *Job){if task.Status==Pending{task.Status=Waiting}}`, 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			h := &testHarness{T: t}
			report := h.investigate(h.fixture(mutationPrelude+test.source), quiet())
			require.Len(t, report.GuardedUpdates, test.groups)
		})
	}
}
func TestGuardedMutationOperationSchemas(t *testing.T) {
	h := &testHarness{T: t}
	report := h.investigate(h.fixture(`package fixture
 type State struct{Count int;Text string;Flag bool}
 func a(x *State,n int,s string){
 if x.Count!=n{x.Count=n}
 if x.Text!=s{x.Text+=s}
 if x.Flag==false{x.Flag=true}
 if x.Count<50{x.Count++}
 }
 func b(y *State,other int,text string){
 if y.Count!=other{y.Count=other}
 if y.Text!=text{y.Text=y.Text+text}
 if false==y.Flag{y.Flag=true}
 if y.Count<50{y.Count+=1}
 }
 func c(z *State){if z.Count<50{z.Count=z.Count+1}}
 `), quiet())
	require.Len(t, report.GuardedUpdates, 4)
	counts := []int{}
	for _, group := range report.GuardedUpdates {
		counts = append(counts, len(group.sites))
	}
	require.ElementsMatch(t, []int{2, 2, 2, 3}, counts)
}
func TestGuardedMutationInputRelationships(t *testing.T) {
	h := &testHarness{T: t}
	report := h.investigate(h.fixture(`package fixture
 type Account struct{Balance,Limit int}
 func a(x *Account,amount,fee int){if x.Balance>=amount{x.Balance-=fee}}
 func b(y *Account,cost,tax int){if y.Balance>=cost{y.Balance-=tax}}
 func c(z *Account,cost int){if z.Balance>=cost{z.Balance-=cost}}
 func limit1(x *Account){if x.Balance>=x.Limit{x.Balance-=x.Limit}}
 func limit2(y *Account){if y.Balance>=y.Limit{y.Balance=y.Balance-y.Limit}}
 `), quiet())
	require.Len(t, report.GuardedUpdates, 2)
	for _, group := range report.GuardedUpdates {
		require.Len(t, group.sites, 2)
	}
	db := h.snapshot(report)
	rendered, _, err := RenderSnapshot(testDatabaseQueries(db), "report.sqlite")
	require.NoError(t, err)
	require.Contains(t, string(rendered), "mutation-input")
	require.Contains(t, string(rendered), "normalized-check")
	require.Contains(t, string(rendered), "normalized-write")
}

func TestGuardedMutationExpressionIdentity(t *testing.T) {
	h := &testHarness{T: t}
	report := h.investigate(h.fixture(`package fixture
 type Account struct{Balance int}
 const First=1
 const Second=1
 func a(x *Account,n int){if x.Balance-n>=0{x.Balance=-n}}
 func b(y *Account,amount int){if y.Balance-amount>=0{y.Balance=-amount}}
 func c(z *Account,n int){if n-z.Balance>=0{z.Balance=-n}}
 func first(x *Account){if x.Balance==int(First){x.Balance=0}}
 func second(x *Account){if x.Balance==int(Second){x.Balance=0}}
 `), quiet())
	require.Len(t, report.GuardedUpdates, 1)
	require.Len(t, report.GuardedUpdates[0].sites, 2)
}
