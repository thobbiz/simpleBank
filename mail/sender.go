package mail

import (
	"fmt"

	"github.com/wneessen/go-mail"
)

const (
	smtpAuthAddress = "smtp.gmail.com"
)

type EmailSender interface {
	SendEmail(
		subject string,
		content string,
		to []string,
		cc []string,
		bcc []string,
		attachFiles []string,
	) error
}

type GmailSender struct {
	name              string
	fromEmailAddress  string
	fromEmailPassword string
}

func NewGmailSender(name, fromEmailAddress, fromEmailPassword string) EmailSender {
	return &GmailSender{
		name:              name,
		fromEmailAddress:  fromEmailAddress,
		fromEmailPassword: fromEmailPassword,
	}
}

func (sender *GmailSender) SendEmail(
	subject string,
	content string,
	to []string,
	cc []string,
	bcc []string,
	attachFiles []string,
) error {
	m := mail.NewMsg()
	if err := m.From(fmt.Sprintf("%s <%s>", sender.name, sender.fromEmailAddress)); err != nil {
		return fmt.Errorf("failed to set From address: %s", err)
	}
	if err := m.To(to...); err != nil {
		return fmt.Errorf("failed to set To address: %s", err)
	}
	m.Subject(subject)
	m.SetBodyString(mail.TypeTextHTML, content)
	m.Cc(cc...)
	m.Bcc(bcc...)

	for _, f := range attachFiles {
		m.AttachFile(f)
	}

	client, err := mail.NewClient(
		smtpAuthAddress,
		mail.WithPort(465),
		mail.WithSMTPAuth(mail.SMTPAuthPlain),
		mail.WithUsername(sender.fromEmailAddress),
		mail.WithPassword(sender.fromEmailPassword),
		mail.WithSSL(),
	)
	if err != nil {
		return fmt.Errorf("failed to create mail client: %s", err)
	}
	if err := client.DialAndSend(m); err != nil {
		return fmt.Errorf("failed to send mail: %s", err)
	}

	return nil
}
