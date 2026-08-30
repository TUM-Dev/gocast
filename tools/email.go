package tools

import (
	"bytes"
	"context"
	"fmt"
	"net/smtp"
	"os/exec"
	"strings"
	"time"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
)

type Mailer struct {
	Dao               dao.DaoWrapper
	MaxMailsPerMinute int
}

func NewMailer(dao dao.DaoWrapper, maxMailsPerMinute int) *Mailer {
	if maxMailsPerMinute == 0 {
		maxMailsPerMinute = 10
	}
	return &Mailer{Dao: dao, MaxMailsPerMinute: maxMailsPerMinute}
}

func (m *Mailer) Run() {
	lastRun := time.Now().Add(-time.Minute)
	for {
		if time.Since(lastRun) < time.Minute {
			time.Sleep(time.Until(lastRun.Add(time.Minute)))
		}
		lastRun = time.Now()
		emails, err := m.Dao.EmailDao.GetDue(context.Background(), m.MaxMailsPerMinute)
		if err != nil {
			logger.Error("error getting due emails", "err", err)
			continue
		}
		for _, email := range emails {
			err := m.sendMail(Cfg.Mail.Server, email.From, email.Subject, email.Body, []string{email.To})
			if err != nil {
				email.LastTry = time.Now()
				email.Retries++
				email.Errors = appendSendError(email.Errors, err)
			} else {
				email.Success = true
			}
			err = m.Dao.EmailDao.Save(context.Background(), &email)
			if err != nil {
				logger.Error("error saving email", "err", err)
			}

			sleepDur := time.Duration(1000 * (60 / m.MaxMailsPerMinute))
			time.Sleep(sleepDur)
		}
	}
}

// maxStoredErrors bounds what one email's failures add up to. How big an error is
// belongs to whatever produced it, not to this package.
const maxStoredErrors = 4000

// truncationMark keeps a cut first line from reading as a mangled error.
const truncationMark = "[earlier attempts dropped]\n"

// appendSendError records a failed attempt, keeping the most recent within the bound.
func appendSendError(existing string, err error) string {
	combined := existing + fmt.Sprintf("%v\n", err)
	if len(combined) <= maxStoredErrors {
		return combined
	}

	tail := combined[len(combined)-maxStoredErrors:]
	// Resuming after a newline keeps the first line whole, and cannot leave the cut
	// inside a multi-byte character.
	if i := strings.IndexByte(tail, '\n'); i >= 0 {
		tail = tail[i+1:]
	} else {
		// One error longer than the bound, so there is no line break to cut at.
		tail = strings.ToValidUTF8(tail, "")
	}

	return truncationMark + tail
}

func (m *Mailer) sendMail(addr, from, subject, body string, to []string) error {
	logger.Info("sending mail", "to", to, "subject", subject, "body", body)
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
	defer func() {
		_ = c.Close()
	}()
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

// SendAccountInvite queues the mail inviting someone to set a password on a new
// account. The register link is single-use, so calling this twice invalidates the
// first mail's.
func SendAccountInvite(ctx context.Context, daoWrapper dao.DaoWrapper, email string) error {
	user, err := daoWrapper.UsersDao.GetUserByEmail(ctx, email)
	if err != nil {
		return fmt.Errorf("no user with that address: %w", err)
	}

	registerLink, err := daoWrapper.UsersDao.CreateRegisterLink(ctx, user)
	if err != nil {
		return fmt.Errorf("creating the register link: %w", err)
	}

	body := fmt.Sprintf("Hello!\n"+
		"You have been invited to use TUM-Live. You can set a password for your account here: https://live.rbg.tum.de/setPassword/%v\n"+
		"After setting a password you can log in with the email this message was sent to. Please note that this is not your TUMOnline account.\n"+
		"If you have any further questions please reach out to "+Cfg.Mail.Sender, registerLink.RegisterSecret)

	if err := daoWrapper.EmailDao.Create(ctx, &model.Email{
		From:    Cfg.Mail.Sender,
		To:      email,
		Subject: "Setup your TUM-Live account",
		Body:    body,
	}); err != nil {
		return fmt.Errorf("queueing the invitation: %w", err)
	}

	return nil
}
