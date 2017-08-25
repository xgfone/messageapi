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
	// c.Emails = map[string]map[string]string{
	// 	"plain": map[string]string{
	// 		"host":     "mail.example.com",
	// 		"port":     "25",
	// 		"username": "username",
	// 		"password": "password",
	// 		"from":     "username@example.com",
	// 	},
	// }
	c.Emails = map[string]map[string]string{
		"plain": map[string]string{
			"host":     "mail.grandcloud.cn",
			"port":     "25",
			"username": "xiegaofeng",
			"password": "XieGaoFeng",
			"from":     "xiegaofeng@grandcloud.cn",
		},
	}
	glog.Error(app.Start(c, ":8080", "", ""))
}
