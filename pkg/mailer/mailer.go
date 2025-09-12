package mailer

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"time"

	"github.com/jofosuware/go/shopit/config"
	mail "github.com/xhit/go-simple-mail/v2"
)

//go:embed "templates"
var emailTemplateFS embed.FS

type Mailer interface {
	SendMail(from, to, subject, tmpl string, data interface{}) error
}

type Mail struct {
	Config *config.Config
}

func NewMail(cfg *config.Config) *Mail {
	return &Mail{
		Config: cfg,
	}
}

func (m *Mail) SendMail(from, to, subject, tmpl string, data interface{}) error {
	templateToRender := fmt.Sprintf("templates/%s.html.tmpl", tmpl)

	t, err := template.New("email-html").ParseFS(emailTemplateFS, templateToRender)
	if err != nil {
		return err
	}

	var tpl bytes.Buffer
	if err = t.ExecuteTemplate(&tpl, "body", data); err != nil {
		return err
	}

	formattedMessage := tpl.String()

	templateToRender = fmt.Sprintf("templates/%s.plain.tmpl", tmpl)
	t, err = template.New("email-plain").ParseFS(emailTemplateFS, templateToRender)
	if err != nil {
		return err
	}

	if err = t.ExecuteTemplate(&tpl, "body", data); err != nil {
		return err
	}

	plainMessage := tpl.String()

	// send the mail
	server := mail.NewSMTPClient()
	server.Host = m.Config.SMTP.Host
	server.Port = m.Config.SMTP.Port
	server.Username = m.Config.SMTP.Username
	server.Password = m.Config.SMTP.Password
	server.Encryption = mail.EncryptionTLS
	server.KeepAlive = false
	server.ConnectTimeout = 10 * time.Second
	server.SendTimeout = 10 * time.Second

	smtpClient, err := server.Connect()
	if err != nil {
		return err
	}

	email := mail.NewMSG()
	email.SetFrom(from).
		AddTo(to).
		SetSubject(subject)

	email.SetBody(mail.TextHTML, formattedMessage)
	email.AddAlternative(mail.TextPlain, plainMessage)

	err = email.Send(smtpClient)
	if err != nil {
		return err
	}

	return nil
}
