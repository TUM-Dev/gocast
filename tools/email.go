package tools

import (
	"net/smtp"
)

func SendPasswordMail(to string, link string) error {
	user := Cfg.MailUser
	password := Cfg.MailPassword
	server := Cfg.MailServer
	auth := smtp.PlainAuth("", user, password, server)

	err := smtp.SendMail(server+":587", auth, user, []string{to}, []byte("To: " + to + "\r\n" +
		"Subject: [TUM-Live] Setup your password.\r\n" +
		"\r\n" +
		"Hello! You can set your password for TUM-Live here: http://localhost:8080/setPassword/" + link + "\r\n"))
	return err

}
