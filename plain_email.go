package messageapi

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/mail"
	"net/smtp"
	"strconv"
	"sync"

	"github.com/scorredoira/email"
)

func init() {
	RegisterEmail("plain", new(plainEmail))
}

type plainEmail struct {
	sync.Mutex

	addr string
	auth smtp.Auth
	from mail.Address
}

func (p *plainEmail) Load(m map[string]string) error {
	var port = 25
	var (
		host     string
		username string
		password string
		from     string
		ok       bool
	)

	if host, ok = m["host"]; !ok {
		return fmt.Errorf("no the host configuration")
	}
	if _port, ok := m["port"]; ok {
		p, err := strconv.ParseInt(_port, 10, 16)
		if err != nil {
			return err
		}
		port = int(p)
	}
	if username, ok = m["username"]; !ok {
		return fmt.Errorf("no the username configuration")
	}
	if password, ok = m["password"]; !ok {
		return fmt.Errorf("no the password configuration")
	}
	if from, ok = m["from"]; !ok {
		return fmt.Errorf("no the from configuration")
	}

	p.Lock()
	defer p.Unlock()

	p.addr = fmt.Sprintf("%s:%d", host, port)
	p.auth = smtp.PlainAuth("", username, password, host)
	p.from = mail.Address{Name: "From", Address: from}
	return nil
}

func (p *plainEmail) SendEmail(cxt context.Context, to []string, subject,
	content string, attachments map[string]io.Reader) error {
	msg := email.NewMessage(subject, content)
	msg.From = p.from
	msg.To = to

	if len(attachments) > 0 {
		for f, r := range attachments {
			if r == nil {
				if err := msg.Attach(f); err != nil {
					return err
				}
			} else {
				buf := bytes.NewBuffer(nil)
				if _, err := io.Copy(buf, r); err != nil && err != io.EOF {
					return err
				}
				msg.AttachBuffer(f, buf.Bytes(), false)
			}
		}
	}

	return email.Send(p.addr, p.auth, msg)
}
