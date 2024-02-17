package mailer

import (
	"bytes"
	"embed"
	"errors"
	"text/template"
	"time"

	"github.com/go-mail/mail/v2"
)

//go:embed "templates"
var templateFS embed.FS

type Mailer struct {
	dialer *mail.Dialer
	sender string
}

func New(host string, port int, username, password, sender string) Mailer {
	dialer := mail.NewDialer(host, port, username, password)
	dialer.Timeout = 5 * time.Second
	return Mailer{
		dialer: dialer,
		sender: sender,
	}
}

func (m Mailer) Send(recipient, templateFile string, data any) error {
	tmpl, err := template.New("email").ParseFS(templateFS, "templates/"+templateFile)
	if err != nil {
		return err
	}
	subject := new(bytes.Buffer)
	subjectTemplateErr := tmpl.ExecuteTemplate(subject, "subject", data)
	if subjectTemplateErr != nil {
		return subjectTemplateErr
	}
	plainBody := new(bytes.Buffer)
	plainBodyTemplateErr := tmpl.ExecuteTemplate(plainBody, "plainBody", data)
	if plainBodyTemplateErr != nil {
		return plainBodyTemplateErr
	}
	htmlBody := new(bytes.Buffer)
	htmlBodyTemplateErr := tmpl.ExecuteTemplate(htmlBody, "htmlBody", data)
	if htmlBodyTemplateErr != nil {
		return htmlBodyTemplateErr
	}
	msg := mail.NewMessage()
	msg.SetHeader("To", recipient)
	msg.SetHeader("From", m.sender)
	msg.SetHeader("Subject", subject.String())
	msg.SetBody("text/plain", plainBody.String())
	msg.AddAlternative("text/html", htmlBody.String())
	for i := 1; i <= 3; i++ {
		sendErr := m.dialer.DialAndSend(msg)
		if sendErr == nil {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return errors.New("failed to send welcome email")
}
