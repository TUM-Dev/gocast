package tools

import (
	"bytes"
	"context"
	"fmt"
	"github.com/joschahenningsen/TUM-Live/dao"
	"log"
	"net/smtp"
	"os/exec"
	"strings"
	"time"
)

type Mailer struct {
	Dao               dao.DaoWrapper
	MaxMailsPerMinute int
}

func NewMailer(dao dao.DaoWrapper, maxMailsPerMinute int) *Mailer {
	return &Mailer{Dao: dao, MaxMailsPerMinute: maxMailsPerMinute}
}

func (m *Mailer) Run() {
	lastRun := time.Now().Add(-time.Minute)
	for {
		if time.Since(lastRun) < time.Minute {
			time.Sleep(time.Until(lastRun.Add(time.Minute)))
		}
		emails, err := m.Dao.EmailDao.GetDue(context.Background(), m.MaxMailsPerMinute)
		if err != nil {
			log.Printf("error getting due emails: %v", err)
			continue
		}
		for _, email := range emails {
			err := m.sendMail(Cfg.Mail.Server, email.From, email.Subject, email.Body, []string{email.To})
			if err != nil {
				email.LastTry = time.Now()
				email.Retries++
			} else {
				email.Success = true
			}
			err = m.Dao.EmailDao.Save(context.Background(), &email)
			if err != nil {
				log.Printf("error saving email: %v", err)
			}

			time.Sleep(time.Duration(1000 * (60 / m.MaxMailsPerMinute)))
		}
	}
}

func (m *Mailer) sendMail(addr, from, subject, body string, to []string) error {
	log.Printf("sending mail to %v, subject: %s body:\n%s", to, subject, body)
	r := strings.NewReplacer("\r\n", "", "\r", "", "\n", "", "%0a", "", "%0d", "")

	signed, err := openssl([]byte(body), "smime", "-text", "-sign", "-signer", Cfg.Mail.SMIMECert, "-inkey", Cfg.Mail.SMIMEKey)
	if err != nil {
		fmt.Printf("can't encrypt: %v", err)
	}
	msg := "To: " + strings.Join(to, ",") + "\r\n" +
		"From: " + from + "\r\n" +
		"Subject: " + subject + "\r\n" +
		strings.ReplaceAll(string(signed), "Content-Type: text/plain", "Content-Type: text/plain; charset=UTF-8")
	// todo: Charset
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

func openssl(stdin []byte, args ...string) ([]byte, error) {
	cmd := exec.Command("openssl", args...)

	in := bytes.NewReader(stdin)
	out := &bytes.Buffer{}
	errs := &bytes.Buffer{}

	cmd.Stdin, cmd.Stdout, cmd.Stderr = in, out, errs

	if err := cmd.Run(); err != nil {
		if len(errs.Bytes()) > 0 {
			return nil, fmt.Errorf("error running %s (%s):\n %v", cmd.Args, err, errs.String())
		}
		return nil, err
	}

	return out.Bytes(), nil
}
