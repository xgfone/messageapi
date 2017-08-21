// Package app implements an interface to send the message based on HTTP.
//
// The package registers two urls by default: "/v1/email" and "/v1/sms".
// You can use them to send the email or the sms message. Both two apis support
// the POST method, not GET, which can be enabled by setting `Config.AllowGet`
// to true.
//
// For POST, the arguments are in body, type of which is "application/json".
// Email needs "subject", "content", "to", "attachments", "provider", and SMS
// needs "phone", "content", "provider". Thereinto, "content", "attachments",
// and "provider" are optional. In most cases, there is no need to use "provider".
// You maybe only use it when there are more than one provider and you want to
// use the specific one.
//
// For GET, the arguments above are in the url query, but not "attachments".
//
// Besides, the package also registers a url by default: "/v1/config". You can
// visit it to get the configuration information by "GET", or modify it by "POST".
// The format is json.
package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/golang/glog"
	"github.com/xgfone/messageapi"
)

// Config is used to configure the app.
type Config struct {
	// If true, allow to use the GET method to send the message.
	// The default is false.
	AllowGet bool `json:"allow_get"`

	// The name of the default sms provider, which is used when it is not given
	// in the request. It's best to give a default provider.
	DefaultSMSProvider string `json:"default_sms_provider,omitempty"`

	// The name of the default email provider, which is used when it is not given
	// in the request. It's best to give a default provider.
	DefaultEmailProvider string `json:"default_email_provider,omitempty"`

	// The configuration of all the email providers. The key is the name of the
	// provider, and the value is its configuration information.
	Emails map[string]map[string]string `json:"emails,omitempty"`

	// The configuration of all the sms providers. The key is the name of the
	// provider, and the value is its configuration information.
	SMSes map[string]map[string]string `json:"smses,omitempty"`

	emails map[string]messageapi.Email
	smses  map[string]messageapi.SMS
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
	if err := ResetConfig(c); err != nil {
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
	configLocker *sync.Mutex
	config       *Config
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
	configLocker = new(sync.Mutex)
	ResetConfig(NewDefaultConfig())
	http.HandleFunc("/v1/email", sendEmail)
	http.HandleFunc("/v1/sms", sendSMS)
	http.HandleFunc("/v1/config", resetConfig)
}

// ResetConfig resets the global default configuration.
//
// Only use this function when you don't call Start and implement it youself.
//
// Notice: You can call this function to change the configuration at any time.
func ResetConfig(conf *Config) error {
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

	conf.emails = _emails
	conf.smses = _smses
	configLocker.Lock()
	config = conf
	configLocker.Unlock()
	return nil
}

func getEmail(name string) messageapi.Email {
	configLocker.Lock()
	_config := config
	configLocker.Unlock()
	if e, ok := _config.emails[name]; ok {
		return e
	}
	return nil
}

func getSMS(name string) messageapi.SMS {
	configLocker.Lock()
	_config := config
	configLocker.Unlock()
	if e, ok := _config.smses[name]; ok {
		return e
	}
	return nil
}

func resetConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		configLocker.Lock()
		_config := config
		configLocker.Unlock()

		if content, err := json.Marshal(_config); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.Write(content)
		}
	} else if r.Method == "POST" {
		buf := bytes.NewBuffer(nil)
		if _, err := buf.ReadFrom(r.Body); err != nil {
			glog.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		_conf := make(map[string]interface{})
		if err := json.Unmarshal(buf.Bytes(), &_conf); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(err.Error()))
			return
		}

		conf := new(Config)
		if _v, ok := _conf["allow_get"]; ok {
			if v, ok := _v.(bool); ok {
				conf.AllowGet = v
			} else {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("the type of allow_get is wrong"))
				return
			}
		}
		if _v, ok := _conf["default_email_provider"]; ok {
			if v, ok := _v.(string); ok {
				conf.DefaultEmailProvider = v
			} else {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("the type of default_email_provider is wrong"))
				return
			}
		}
		if _v, ok := _conf["default_sms_provider"]; ok {
			if v, ok := _v.(string); ok {
				conf.DefaultSMSProvider = v
			} else {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("the type of default_sms_provider is wrong"))
				return
			}
		}
		if _v, ok := _conf["emails"]; ok {
			m, ok := _v.(map[string]interface{})
			if !ok {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("the type of emails is wrong"))
				return
			}

			conf.Emails = make(map[string]map[string]string)
			for key, value := range m {
				v, ok := value.(map[string]interface{})
				if !ok {
					w.WriteHeader(http.StatusBadRequest)
					w.Write([]byte("the type of the value of emails is wrong"))
					return
				}

				if _v, ok := toStringMap(v); ok {
					conf.Emails[key] = _v
				} else {
					w.WriteHeader(http.StatusBadRequest)
					w.Write([]byte("the type of the value of emails is wrong"))
					return
				}
			}
		}
		if _v, ok := _conf["smses"]; ok {
			m, ok := _v.(map[string]interface{})
			if !ok {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("the type of smses is wrong"))
				return
			}

			conf.SMSes = make(map[string]map[string]string)
			for key, value := range m {
				v, ok := value.(map[string]interface{})
				if !ok {
					w.WriteHeader(http.StatusBadRequest)
					w.Write([]byte("the type of the value of smses is wrong"))
					return
				}

				if _v, ok := toStringMap(v); ok {
					conf.SMSes[key] = _v
				} else {
					w.WriteHeader(http.StatusBadRequest)
					w.Write([]byte("the type of the value of emails is wrong"))
					return
				}
			}
		}

		if err := ResetConfig(conf); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(err.Error()))
		}
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
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
	configLocker.Lock()
	_config := config
	configLocker.Unlock()

	var ok bool
	if isEmail {
		ok = len(_config.emails) > 0
	} else {
		ok = len(_config.smses) > 0
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
	} else if _config.AllowGet && r.Method == "GET" {
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
		if _config.DefaultEmailProvider != "" {
			args.SetProvider(_config.DefaultEmailProvider)
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

func toStringMap(v map[string]interface{}) (map[string]string, bool) {
	if len(v) == 0 {
		return nil, true
	}

	vs := make(map[string]string, len(v))
	for _k, _v := range v {
		s, ok := _v.(string)
		if !ok {
			return nil, false
		}
		vs[_k] = s
	}
	return vs, true
}
