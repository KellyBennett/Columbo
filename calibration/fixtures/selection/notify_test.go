package selection

import (
	"reflect"
	"testing"
)

func TestDelivery(t *testing.T) {
	sent = nil
	steps := []error{
		Notify(true, "555", "a"),
		Notify(false, "x@y", "b"),
		Notify(true, "", "c"),
		Alert("sms", "d"),
		Alert("mail", "e"),
		Alert("pager", "f"),
		Broadcast("sms", "777", "g"),
		Broadcast("fax", "z@y", "h"),
	}
	fails := 0
	for _, err := range steps {
		if err != nil {
			fails++
		}
	}
	if fails != 1 {
		t.Errorf("%d failures, want 1", fails)
	}
	want := []string{"sms:555:a", "email:x@y:b", "sms:0:d", "email:ops:e", "sms:777:g", "email:z@y:h"}
	if !reflect.DeepEqual(sent, want) {
		t.Errorf("sent %v, want %v", sent, want)
	}
}
