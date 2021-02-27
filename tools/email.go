package tools

import (
	"net/smtp"
	"os"
)

func SendPasswordMail(to string, link string) error {
	user := os.Getenv("MAILUSER")
	password := os.Getenv("MAILPASSWORD")
	server :=os.Getenv("MAILSERVER")
	auth := smtp.PlainAuth("", user, password, server)

	msg := []byte("To: " + to + "\r\n" +
		"Subject: [TUM-Live] Setup your password.\r\n" +
		"\r\n" +
		"Hello! You can set your password for TUM-Live here: http://localhost:8080/setPassword/" + link + "\r\n")
	err := smtp.SendMail(server+":587", auth, user, []string{to}, msg)
	return err

}
