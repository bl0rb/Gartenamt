package services

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"mime"
	"mime/multipart"
	"net"
	"net/smtp"
	"net/textproto"
	"strings"
	"time"

	"kleingarten-verwaltung/models"
)

type MailAttachment struct {
	Filename    string
	ContentType string
	Data        []byte
}

type MailRequest struct {
	To          []string
	Subject     string
	HTMLBody    string
	TextBody    string
	Attachments []MailAttachment
}

type EmailService struct{}

func NewEmailService() *EmailService {
	return &EmailService{}
}

func (s *EmailService) SendMail(settings *models.OrganizationSettings, request MailRequest) error {
	fromAddress := settings.SMTPFromAddr
	if fromAddress == "" {
		fromAddress = settings.Email
	}
	if fromAddress == "" {
		return fmt.Errorf("keine Absenderadresse konfiguriert")
	}
	if settings.SMTPHost == "" || settings.SMTPPort == 0 {
		return fmt.Errorf("SMTP-Konfiguration unvollständig")
	}

	message, err := buildMIMEMessage(settings, fromAddress, request)
	if err != nil {
		return err
	}

	addr := net.JoinHostPort(settings.SMTPHost, fmt.Sprintf("%d", settings.SMTPPort))
	tlsConfig := &tls.Config{ServerName: settings.SMTPHost, MinVersion: tls.VersionTLS12}

	mode := strings.ToLower(strings.TrimSpace(settings.SMTPTLSMode))
	if mode == "" {
		mode = "tls"
	}

	var client *smtp.Client
	if mode == "tls" || mode == "ssl" {
		conn, err := tls.Dial("tcp", addr, tlsConfig)
		if err != nil {
			return err
		}
		client, err = smtp.NewClient(conn, settings.SMTPHost)
		if err != nil {
			return err
		}
	} else {
		conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
		if err != nil {
			return err
		}
		client, err = smtp.NewClient(conn, settings.SMTPHost)
		if err != nil {
			return err
		}
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(tlsConfig); err != nil {
				return err
			}
		}
	}
	defer client.Close()

	if settings.SMTPUsername != "" {
		auth := smtp.PlainAuth("", settings.SMTPUsername, settings.SMTPPassword, settings.SMTPHost)
		if err := client.Auth(auth); err != nil {
			return err
		}
	}

	if err := client.Mail(fromAddress); err != nil {
		return err
	}
	for _, recipient := range request.To {
		if err := client.Rcpt(recipient); err != nil {
			return err
		}
	}

	writer, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := writer.Write(message); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}

	return client.Quit()
}

func buildMIMEMessage(settings *models.OrganizationSettings, fromAddress string, request MailRequest) ([]byte, error) {
	var message bytes.Buffer
	mixedWriter := multipart.NewWriter(&message)
	alternativeBoundary := fmt.Sprintf("alt-%d", time.Now().UnixNano())

	fromHeader := fromAddress
	if settings.SMTPFromName != "" {
		fromHeader = fmt.Sprintf("%s <%s>", mime.QEncoding.Encode("utf-8", settings.SMTPFromName), fromAddress)
	}

	headers := []string{
		"MIME-Version: 1.0",
		fmt.Sprintf("From: %s", fromHeader),
		fmt.Sprintf("To: %s", strings.Join(request.To, ", ")),
		fmt.Sprintf("Subject: %s", mime.QEncoding.Encode("utf-8", request.Subject)),
		fmt.Sprintf("Content-Type: multipart/mixed; boundary=%s", mixedWriter.Boundary()),
		"",
	}
	message.WriteString(strings.Join(headers, "\r\n"))

	altHeader := textproto.MIMEHeader{}
	altHeader.Set("Content-Type", fmt.Sprintf("multipart/alternative; boundary=%s", alternativeBoundary))
	altPart, err := mixedWriter.CreatePart(altHeader)
	if err != nil {
		return nil, err
	}

	altWriter := multipart.NewWriter(altPart)
	_ = altWriter.SetBoundary(alternativeBoundary)

	textHeader := textproto.MIMEHeader{}
	textHeader.Set("Content-Type", "text/plain; charset=utf-8")
	textHeader.Set("Content-Transfer-Encoding", "8bit")
	textPart, err := altWriter.CreatePart(textHeader)
	if err != nil {
		return nil, err
	}
	if _, err := textPart.Write([]byte(request.TextBody)); err != nil {
		return nil, err
	}

	htmlHeader := textproto.MIMEHeader{}
	htmlHeader.Set("Content-Type", "text/html; charset=utf-8")
	htmlHeader.Set("Content-Transfer-Encoding", "8bit")
	htmlPart, err := altWriter.CreatePart(htmlHeader)
	if err != nil {
		return nil, err
	}
	if _, err := htmlPart.Write([]byte(request.HTMLBody)); err != nil {
		return nil, err
	}

	if err := altWriter.Close(); err != nil {
		return nil, err
	}

	for _, attachment := range request.Attachments {
		attachmentHeader := textproto.MIMEHeader{}
		attachmentHeader.Set("Content-Type", fmt.Sprintf("%s; name=%q", attachment.ContentType, attachment.Filename))
		attachmentHeader.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", attachment.Filename))
		attachmentHeader.Set("Content-Transfer-Encoding", "base64")

		attachmentPart, err := mixedWriter.CreatePart(attachmentHeader)
		if err != nil {
			return nil, err
		}

		encoded := make([]byte, base64.StdEncoding.EncodedLen(len(attachment.Data)))
		base64.StdEncoding.Encode(encoded, attachment.Data)
		for len(encoded) > 76 {
			if _, err := attachmentPart.Write(encoded[:76]); err != nil {
				return nil, err
			}
			if _, err := attachmentPart.Write([]byte("\r\n")); err != nil {
				return nil, err
			}
			encoded = encoded[76:]
		}
		if len(encoded) > 0 {
			if _, err := attachmentPart.Write(encoded); err != nil {
				return nil, err
			}
			if _, err := attachmentPart.Write([]byte("\r\n")); err != nil {
				return nil, err
			}
		}
	}

	if err := mixedWriter.Close(); err != nil {
		return nil, err
	}

	return message.Bytes(), nil
}
