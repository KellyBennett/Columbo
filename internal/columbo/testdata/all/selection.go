package fixture

type Sender interface {
	Send(string) error
	Close() error
}
type Email struct{}

func (Email) Send(string) error { return nil }
func (Email) Close() error      { return nil }

type SMS struct{}

func (*SMS) Send(string) error { return nil }
func (*SMS) Close() error      { return nil }
func Deliver(email bool) error {
	var sender Sender
	if email {
		sender = Email{}
	} else {
		sender = &SMS{}
	}
	selected := sender
	active := selected
	active.Send("hello")
	return active.Close()
}
