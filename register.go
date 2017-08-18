// Package messageapi is the interface to send the message by the email or the sms.
package messageapi

import (
	"context"
	"fmt"
	"io"
)

// Config is the interface to load the configuration information.
//
// The provider receives the configuration and should cache it.
// When the configuration changes, this function will be called.
// So it may be called more than once, and must be thread-safe.
//
// The configuration passed by the interface Load should be invariable
// each time to call the send interface, such as SendSMS or SendEmail.
// Or please use the context in SendSMS, or SendEmail.
//
// Notice: When failed to load the configuration, you should not use
// the corresponding plugin, or disable it for the moment until it's ok.
type Config interface {
	Load(map[string]string) error
}

// SMS is the interface which the SMS provider implements.
type SMS interface {
	Config
	SendSMS(cxt context.Context, phone, content string) error
}

// Email is the interface which the email provider implements.
type Email interface {
	Config
	SendEmail(cxt context.Context, to []string, subject, content string,
		attachments map[string]io.Reader) error
}

var (
	smses  = make(map[string]SMS)
	emails = make(map[string]Email)
)

// RegisterSMS registers a SMS provider implementation.
//
// Notice: The plugin is a single instance in the global.
func RegisterSMS(name string, sms SMS) {
	if _, ok := smses[name]; ok {
		panic(fmt.Errorf("%s has been registered", name))
	}
	smses[name] = sms
}

// RegisterEmail registers a Email provider implementation.
//
// Notice: The plugin is a single instance in the global.
func RegisterEmail(name string, email Email) {
	if _, ok := emails[name]; ok {
		panic(fmt.Errorf("%s has been registered", name))
	}
	emails[name] = email
}

// GetSMS returns a named SMS provider.
//
// Return nil if there is no the sms provider named name.
func GetSMS(name string) SMS {
	if s, ok := smses[name]; ok {
		return s
	}
	return nil
}

// GetEmail returns a named Email provider.
//
// Return nil if there is no the email provider named name.
func GetEmail(name string) Email {
	if s, ok := emails[name]; ok {
		return s
	}
	return nil
}

// GetAllEmails returns all the email providers.
func GetAllEmails() map[string]Email {
	return emails
}

// GetAllSMSs returns all the sms providers.
func GetAllSMSs() map[string]SMS {
	return smses
}
