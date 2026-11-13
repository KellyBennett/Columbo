package selection

import "errors"

type Sender interface {
	Send(msg string) error
}

type Email struct{ Addr string }

type SMS struct{ Number string }

var sent []string

func (e *Email) Send(msg string) error {
	sent = append(sent, "email:"+e.Addr+":"+msg)
	return nil
}

func (s *SMS) Send(msg string) error {
	if s.Number == "" {
		return errors.New("no number")
	}
	sent = append(sent, "sms:"+s.Number+":"+msg)
	return nil
}

func NewSender(sms bool, to string) Sender {
	if sms {
		return &SMS{Number: to}
	}
	return &Email{Addr: to}
}

func NewChannel(kind string) Sender {
	switch kind {
	case "sms":
		return &SMS{Number: "0"}
	case "email", "mail":
		return &Email{Addr: "ops"}
	}
	return nil
}

func Notify(sms bool, to, msg string) error {
	s := NewSender(sms, to)
	return s.Send(msg)
}

func Alert(kind, msg string) error {
	ch := NewChannel(kind)
	if ch == nil {
		return nil
	}
	return ch.Send(msg)
}

func Broadcast(kind, to, msg string) error {
	var s Sender
	switch kind {
	case "sms":
		s = &SMS{Number: to}
	default:
		s = &Email{Addr: to}
	}
	return s.Send(msg)
}
