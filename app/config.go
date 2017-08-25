package app

import (
	"fmt"

	"github.com/xgfone/go-tools/validation"
	"github.com/xgfone/messageapi"
)

// Config is used to configure the app.
type Config struct {
	// If true, allow to use the GET method to send the message.
	// The default is false.
	AllowGet bool `json:"allow_get"`

	// if true, don't report an error when not support the given provider.
	IgnoreNotSupportedProvider bool `json:"ignore_not_supported_provider"`

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

	key    string
	emails map[string]messageapi.Email
	smses  map[string]messageapi.SMS
}

// NewDefaultConfig returns a default configuration.
//
// If the key is not empty, it must be given and matched when resetting the
// configuration by the HTTP API; or the configuration is not allowed to be reset.
//
// DefaultEmailProvider is "plain" by default.
func NewDefaultConfig(key string) *Config {
	return &Config{
		key:                  key,
		DefaultEmailProvider: "plain",
	}
}

// ResetConfig resets the global default configuration.
//
// Only use this function when you don't call Start and implement it youself.
//
// Notice: You can call this function to change the configuration at any time.
// And it's necessary to give the whole configuration options When resetting
// the configuration.
func ResetConfig(conf *Config) error {
	if conf == nil {
		return nil
	}

	_emails := make(map[string]messageapi.Email)
	for n, c := range conf.Emails {
		provider := messageapi.GetEmail(n)
		if provider == nil {
			if conf.IgnoreNotSupportedProvider {
				continue
			}
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
			if conf.IgnoreNotSupportedProvider {
				continue
			}
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

func parseConfig(_conf map[string]interface{}) (conf *Config, err error) {
	conf = new(Config)

	// Parse the option of allow_get.
	if _v, ok := _conf["allow_get"]; ok {
		if !validation.VerifyType(_v, "bool") {
			return nil, fmt.Errorf("the type of allow_get is not bool")
		}
		conf.AllowGet = _v.(bool)
	}

	// Parse the option of ignore_not_supported_provider.
	if _v, ok := _conf["ignore_not_supported_provider"]; ok {
		if !validation.VerifyType(_v, "bool") {
			return nil, fmt.Errorf("the type of ignore_not_supported_provider is not bool")
		}
		conf.IgnoreNotSupportedProvider = _v.(bool)
	}

	// Parse the option of default_email_provider.
	if _v, ok := _conf["default_email_provider"]; ok {
		if !validation.VerifyType(_v, "string") {
			return nil, fmt.Errorf("the type of default_email_provider is not string")
		}
		conf.DefaultEmailProvider = _v.(string)
	}

	// Parse the option of default_sms_provider.
	if _v, ok := _conf["default_sms_provider"]; ok {
		if !validation.VerifyType(_v, "string") {
			return nil, fmt.Errorf("the type of default_sms_provider is not string")
		}
		conf.DefaultSMSProvider = _v.(string)
	}

	// Parse the option of emails.
	if _v, ok := _conf["emails"]; ok {
		if !validation.VerifyType(_v, "string2interface") {
			return nil, fmt.Errorf("the type of emails is not json")
		}
		m := _v.(map[string]interface{})
		conf.Emails = make(map[string]map[string]string)

		for key, value := range m {
			if !validation.VerifyType(value, "string2interface") {
				return nil, fmt.Errorf("the type of the email provider[%s] config is not json", key)
			}
			v := value.(map[string]interface{})
			if _v, ok := toStringMap(v); ok {
				conf.Emails[key] = _v
			} else {
				return nil, fmt.Errorf("the type of the value of email is wrong")
			}
		}
	}

	// Parse the option of smses.
	if _v, ok := _conf["smses"]; ok {
		if !validation.VerifyType(_v, "string2interface") {
			return nil, fmt.Errorf("the type of smses is not json")
		}
		m := _v.(map[string]interface{})
		conf.SMSes = make(map[string]map[string]string)

		for key, value := range m {
			if !validation.VerifyType(value, "string2interface") {
				return nil, fmt.Errorf("the type of the sms provider[%s] config is not json", key)
			}
			v := value.(map[string]interface{})
			if _v, ok := toStringMap(v); ok {
				conf.SMSes[key] = _v
			} else {
				return nil, fmt.Errorf("the type of the value of sms is wrong")
			}
		}
	}

	return
}
