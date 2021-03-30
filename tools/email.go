package tools

import (
	"encoding/base64"
	"fmt"
	"net/smtp"
	"strings"
)

func SendPasswordMail(to string, link string) error {
	err := SendMail(Cfg.MailServer, Cfg.MailUser, "Setup your TUM-Live Account",
		fmt.Sprintf("Hello!<br>\n"+
			"You can set a password for your TUM-Live account here: <a href=\"https://live.mm.rbg.tum.de/setPassword/%v\">https://live.mm.rbg.tum.de/setPassword/%v</a>.</br>\n" +
			"If you have any further questions please reach out to <a href=\"rbg@in.tum.de\">rbg@in.tum.de</a>", link, link),
		[]string{to})
	return err
}

func SendMail(addr, from, subject, body string, to []string) error {
	r := strings.NewReplacer("\r\n", "", "\r", "", "\n", "", "%0a", "", "%0d", "")

	c, err := smtp.Dial(addr)
	if err != nil {
		return err
	}
	defer c.Close()
	if err = c.Mail(r.Replace(from)); err != nil {
		return err
	}
	for i := range to {
		to[i] = r.Replace(to[i])
		if err = c.Rcpt(to[i]); err != nil {
			return err
		}
	}

	w, err := c.Data()
	if err != nil {
		return err
	}

	msg := "To: " + strings.Join(to, ",") + "\r\n" +
		"From: " + from + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"Content-Type: text/html; charset=\"UTF-8\"\r\n" +
		"Content-Transfer-Encoding: base64\r\n" +
		"\r\n" + base64.StdEncoding.EncodeToString([]byte(body))

	_, err = w.Write([]byte(msg))
	if err != nil {
		return err
	}
	err = w.Close()
	if err != nil {
		return err
	}
	return c.Quit()
}
