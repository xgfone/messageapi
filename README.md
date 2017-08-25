messageapi
==========

The API to send the message by the email or the sms.

## How to write the plugin?

For the api interface, see the [doc](http://godoc.org/?q=github.com/xgfone/messageapi).

For the invariable arguments each time to call the send interface, you should receive it by the interface mentod of `Load`; or use `context.Context`, such as `context.WithValue`.

### For Email

1. Implement the interface `Email`, that's, the two methods:
```go
Load(map[string]string) error
SendEmail(context.Context, []string, string, string, map[string]io.Reader) error
```
2. Register the plugin with a name by the function `RegisterEmail`:
```go
RegisterEmail(pluginName, EmailPlugin)
```

By default, the api implements and registers the `plain` provider, which needs to `Load` the configuration options: `host`, `port`, `username`, `password`, `from`.

### For SMS

1. Implement the interface `SMS`, that's, the two methods:
```go
Load(map[string]string) error
SendSMS(cxt context.Context, phone, content string) error
```
2. Register the plugin with a name by the function `RegisterSMS`:
```go
RegisterSMS(pluginName, SMSPlugin)
```

## How to use?

1. Get the provider with the name by `GetSMS`, or `GetEmail`.
2. Load the configuration with the provider.
3. Send the email or sms message.

### Send the email
```go
package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/xgfone/messageapi"
)

func main() {
	// Get the email provider.
	email := messageapi.GetEmail("plain")
	if email == nil {
		fmt.Println("no email provider")
		return
	}

	// Load the configuration only once.
	config := map[string]string{
		"host":     "mail.example.com",
		"port":     "25",
		"username": "username",
		"password": "password",
		"from":     "username@example.com",
	}
	if err := email.Load(config); err != nil {
		fmt.Println(err)
		return
	}

	// Send the email without the attachment.
	to := "username@example.com"
	subject := "test"
	content := "test email send"
	if err := email.SendEmail(context.TODO(), []string{to}, subject, content, nil); err != nil {
		fmt.Println(err)
	}

	f, err := os.Open("test.txt")
	if err != nil {
		fmt.Println(err)
		return
	}

	// Send the email with the attachment for the first way.
	attachment1 := map[string]io.Reader{
		"test1.txt": f,
	}
	if err := email.SendEmail(context.TODO(), []string{to}, subject, content, attachment1); err != nil {
		fmt.Println(err)
	}

	// Send the email with the attachment for the second way.
	// The key is the path of the file, and the provider will open and read it.
	// The name of the attachment is same as filepath.Base(key).
	attachment2 := map[string]io.Reader{
		"test.txt": nil,
	}
	if err := email.SendEmail(context.TODO(), []string{to}, subject, content, attachment2); err != nil {
		fmt.Println(err)
	}
}
```

### Send the sms

It is the similar to `email`.

## App based on HTTP

You can use `github.com/xgfone/messageapi/app` to implement an app to send the Email or SMS based on HTTP. The example is as follow.

```go
package main

import (
	"flag"

	"github.com/golang/glog"
	"github.com/xgfone/messageapi/app"
)

func main() {
	flag.Parse()
	c := app.NewDefaultConfig("")
	c.AllowGet = true // Allow to use the GET method to send the message
	c.Emails = map[string]map[string]string{
		"plain": map[string]string{
			"host":     "mail.example.com",
			"port":     "25",
			"username": "username",
			"password": "password",
			"from":     "username@example.com",
		},
	}
	glog.Error(app.Start(c, ":8080", "", ""))
}
```
