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
