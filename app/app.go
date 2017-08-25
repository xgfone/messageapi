// Package app implements an interface to send the message based on HTTP.
//
// The package registers two urls by default: "/v1/email" and "/v1/sms".
// You can use them to send the email or the sms messagr. Both two apis support
// the POST method, not GET, which can be enabled by setting `Config.AllowGet`
// to trur.
//
// For POST, the arguments are in body, type of which is "application/json".
// Email needs "subject", "content", "to", "attachments", "provider", and SMS
// needs "phone", "content", "provider". Thereinto, "content", "attachments",
// and "provider" are optional. In most cases, there is no need to use "provider".
// You maybe only use it when there are more than one provider and you want to
// use the specific onr.
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
	"github.com/xgfone/go-tools/validation"
	"github.com/xgfone/messageapi"
)

const (
	defaultSMSProvider   = ""
	defaultEmailProvider = "plain"
)

var (
	configLocker *sync.Mutex
	config       *Config
)

func init() {
	configLocker = new(sync.Mutex)
	ResetConfig(NewDefaultConfig(""))
	http.HandleFunc("/v1/email", sendEmail)
	http.HandleFunc("/v1/sms", sendSMS)
	http.HandleFunc("/v1/config", resetConfig)
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
	configLocker.Lock()
	_config := config
	configLocker.Unlock()

	if r.Method == "GET" {
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

		if _config.key != "" {
			if !validation.VerifyMapValueType(_conf, "key", "string") {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte("have no key, or the key type is not a string"))
				return
			}
			if _config.key != _conf["key"].(string) {
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte("The key is invalid"))
				return
			}
		}

		conf, err := parseConfig(_conf)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(err.Error()))
			return
		}

		if err := ResetConfig(conf); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(err.Error()))
		}
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

type request struct {
	Provider    string            `json:"provider"`
	Phone       string            `json:"phone"`
	Subject     string            `json:"subject"`
	Content     string            `json:"content"`
	To          string            `json:"to"`
	Attachments map[string]string `json:"attachments"`

	tos         []string
	attachments map[string]io.Reader
}

func (r *request) ValidateEmail() error {
	if r.Provider == "" {
		return fmt.Errorf("the provider is empty")
	} else if r.To == "" {
		return fmt.Errorf("the to is empty")
	} else if r.Subject == "" {
		return fmt.Errorf("the subject is empty")
	}

	r.tos = strings.Split(r.To, ",")
	var attachments map[string]io.Reader
	if len(r.Attachments) != 0 {
		attachments = make(map[string]io.Reader, len(r.Attachments))
		for f, c := range r.Attachments {
			attachments[f] = bytes.NewBufferString(c)
		}
	}
	r.attachments = attachments
	return nil
}

func (r *request) ValidateSMS() error {
	if r.Provider == "" {
		return fmt.Errorf("the provider is empty")
	} else if r.Phone == "" {
		return fmt.Errorf("the phone is empty")
	}

	return nil
}

func sendEmail(w http.ResponseWriter, r *http.Request) {
	args := handleRequestArgs(true, w, r)
	if args == nil {
		return
	}

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
	}
}

func sendSMS(w http.ResponseWriter, r *http.Request) {
	args := handleRequestArgs(false, w, r)
	if args == nil {
		return
	}

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
	}
}

func handleRequestArgs(isEmail bool, w http.ResponseWriter, r *http.Request) (args *request) {
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

	if r.Method == "POST" {
		buf := bytes.NewBuffer(nil)
		if n, err := buf.ReadFrom(r.Body); err != nil || n != r.ContentLength {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(fmt.Sprintf("cannot read the body, err=%s", err)))
			return
		}
		args = new(request)

		if err := json.Unmarshal(buf.Bytes(), args); err != nil {
			glog.Errorf("the path %s from %s: %s", r.URL.Path, r.RemoteAddr, err)
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(err.Error()))
			return nil
		}
	} else if _config.AllowGet && r.Method == "GET" {
		if err := r.ParseForm(); err != nil {
			glog.Errorf("the path %s from %s: %s", r.URL.Path, r.RemoteAddr, err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		args := new(request)
		args.Provider = r.FormValue("provider")
		args.Subject = r.FormValue("subject")
		args.Content = r.FormValue("content")
		args.To = r.FormValue("to")
		args.Phone = r.FormValue("phone")
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if args.Provider == "" {
		if isEmail {
			if _config.DefaultEmailProvider != "" {
				args.Provider = _config.DefaultEmailProvider
			} else {
				args.Provider = defaultEmailProvider
			}
		} else {
			if _config.DefaultSMSProvider != "" {
				args.Provider = _config.DefaultSMSProvider
			} else {
				args.Provider = defaultSMSProvider
			}
		}
	}

	var err error
	if isEmail {
		err = args.ValidateEmail()
	} else {
		err = args.ValidateSMS()
	}
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return nil
	}

	return
}
