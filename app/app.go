// Package app implements an interface to send the message based on HTTP.
//
// The package registers two urls by default: "/v1/email" and "/v1/sms".
// You can use them to send the email or the sms message. Both two apis support
// the POST method, not GET, which can be enabled by setting `Config.AllowGet`
// to true.
//
// For POST, the argument is in body, type of which is "application/json".
// Email needs "subject", "content", "to", "attachments", "provider", and SMS
// needs "phone", "content", "provider".
package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/golang/glog"
	"github.com/xgfone/messageapi"
)

// Config is used to configure the app.
type Config struct {
	// If true, allow to use the GET method to send the message.
	// The default is false.
	AllowGet bool

	// The name of the default sms provider, which is used when it is not given
	// in the request.
	DefaultSMSProvider string

	// The name of the default email provider, which is used when it is not given
	// in the request.
	DefaultEmailProvider string

	// The configuration of all the email providers. The key is the name of the
	// provider, and the value is its configuration information.
	Emails map[string]map[string]string

	// The configuration of all the sms providers. The key is the name of the
	// provider, and the value is its configuration information.
	SMSes map[string]map[string]string
}

// NewDefaultConfig returns a default configuration.
//
// DefaultEmailProvider is "plain" by default.
func NewDefaultConfig() *Config {
	return &Config{
		DefaultEmailProvider: "plain",
	}
}

// Start starts the app.
//
// If certFile and keyFile are not empty, it will start the app with TLS.
func Start(c *Config, addr, certFile, keyFile string) error {
	if err := resetConfig(c); err != nil {
		return err
	}

	glog.Infof("listening on %s", addr)

	if certFile == "" || keyFile == "" {
		return http.ListenAndServe(addr, nil)
	}
	return http.ListenAndServeTLS(addr, certFile, keyFile, nil)
}

var (
	// ErrEmpty is returned when the value is empty.
	ErrEmpty = fmt.Errorf("The value is empty")
)

const (
	defaultSMSProvider   = ""
	defaultEmailProvider = "plain"
)

var (
	config *Config

	emails map[string]messageapi.Email
	smses  map[string]messageapi.SMS
)

type request interface {
	SetProvider(p string)
	Validate() error
}

type emailRequest struct {
	Provider    string            `json:"provider"`
	Subject     string            `json:"subject"`
	Content     string            `json:"content"`
	To          string            `json:"to"`
	Attachments map[string]string `json:"attachments"`

	tos         []string
	attachments map[string]io.Reader
}

func (e *emailRequest) SetProvider(p string) {
	e.Provider = p
}

func (e *emailRequest) Validate() error {
	if e.To == "" || e.Subject == "" || e.Provider == "" {
		return ErrEmpty
	}

	e.tos = strings.Split(e.To, ",")
	var attachments map[string]io.Reader
	if len(e.Attachments) != 0 {
		attachments = make(map[string]io.Reader, len(e.Attachments))
		for f, c := range e.Attachments {
			attachments[f] = bytes.NewBufferString(c)
		}
	}
	e.attachments = attachments
	return nil
}

type smsRequest struct {
	Provider string `json:"provider"`
	Phone    string `json:"phone"`
	Content  string `json:"content"`
}

func (s *smsRequest) SetProvider(p string) {
	s.Provider = p
}

func (s *smsRequest) Validate() error {
	if s.Phone == "" || s.Provider == "" {
		return ErrEmpty
	}
	return nil
}

func init() {
	resetConfig(NewDefaultConfig())
	http.HandleFunc("/v1/email", sendEmail)
	http.HandleFunc("/v1/sms", sendSMS)
}

func resetConfig(conf *Config) error {
	_emails := make(map[string]messageapi.Email)
	for n, c := range conf.Emails {
		provider := messageapi.GetEmail(n)
		if provider == nil {
			return fmt.Errorf("have no the email provider[%s]", n)
		}

		if err := provider.Load(c); err != nil {
			return fmt.Errorf("Failed to load the email configuration, err=%s", err)
		}
		_emails[n] = provider
	}

	_smses := make(map[string]messageapi.SMS)
	for n, c := range conf.SMSes {
		provider := messageapi.GetSMS(n)
		if provider == nil {
			return fmt.Errorf("have no the sms provider[%s]", n)
		}

		if err := provider.Load(c); err != nil {
			return fmt.Errorf("Failed to load the sms configuration, err=%s", err)
		}
		_smses[n] = provider
	}

	emails = _emails
	smses = _smses
	config = conf
	return nil
}

func getEmail(name string) messageapi.Email {
	if e, ok := emails[name]; ok {
		return e
	}
	return nil
}

func getSMS(name string) messageapi.SMS {
	if e, ok := smses[name]; ok {
		return e
	}
	return nil
}

func sendEmail(w http.ResponseWriter, r *http.Request) {
	_args := handleRequestArgs(true, w, r)
	if _args == nil {
		return
	}

	args := _args.(*emailRequest)
	email := getEmail(args.Provider)
	if email == nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(fmt.Sprintf("have no the email provider[%s]", args.Provider)))
		return
	}

	err := email.SendEmail(context.TODO(), args.tos, args.Subject, args.Content, args.attachments)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		if _, err := w.Write([]byte(err.Error())); err != nil {
			glog.Error(err)
		}
		return
	}
}

func sendSMS(w http.ResponseWriter, r *http.Request) {
	_args := handleRequestArgs(false, w, r)
	if _args == nil {
		return
	}

	args := _args.(*smsRequest)
	sms := getSMS(args.Provider)
	if sms == nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(fmt.Sprintf("have no the sms provider[%s]", args.Provider)))
		return
	}

	err := sms.SendSMS(context.TODO(), args.Phone, args.Content)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		if _, err := w.Write([]byte(err.Error())); err != nil {
			glog.Error(err)
		}
		return

	}
}

func handleRequestArgs(isEmail bool, w http.ResponseWriter, r *http.Request) (args request) {
	var ok bool
	if isEmail {
		ok = len(emails) > 0
	} else {
		ok = len(smses) > 0
	}
	if !ok {
		w.WriteHeader(http.StatusNotImplemented)
		return
	}

	var provider string
	if r.Method == "POST" {
		buf := bytes.NewBuffer(nil)
		n, err := buf.ReadFrom(r.Body)
		if err != nil || n != r.ContentLength {
			glog.Errorf("cannot read the body, err=%s, len=%d, content_len=%d",
				err, n, r.ContentLength)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if isEmail {
			args = new(emailRequest)
		} else {
			args = new(smsRequest)
		}
		if err := json.Unmarshal(buf.Bytes(), args); err != nil {
			glog.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	} else if config.AllowGet && r.Method == "GET" {
		if err := r.ParseForm(); err != nil {
			glog.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if isEmail {
			_args := new(emailRequest)
			_args.Provider = r.FormValue("provider")
			_args.Subject = r.FormValue("subject")
			_args.Content = r.FormValue("content")
			_args.To = r.FormValue("to")
			provider = _args.Provider
			args = _args
		} else {
			_args := new(smsRequest)
			_args.Provider = r.FormValue("provider")
			_args.Phone = r.FormValue("phone")
			_args.Content = r.FormValue("content")
			provider = _args.Provider
			args = _args
		}
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if provider == "" {
		if config.DefaultEmailProvider != "" {
			args.SetProvider(config.DefaultEmailProvider)
		} else {
			args.SetProvider(defaultEmailProvider)
		}
	}

	if err := args.Validate(); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return nil
	}
	return
}
