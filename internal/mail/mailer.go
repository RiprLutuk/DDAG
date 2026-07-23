package mail

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/smtp"
	"os"
	"strconv"
)

// Mailer handles email sending via SMTP or Brevo API v3
type Mailer struct {
	Host       string
	Port       int
	User       string
	Password   string
	SenderName string
	SenderMail string
	BrevoAPIKey string // if set, uses API v3 instead of SMTP
}

func NewMailerFromEnv() *Mailer {
	port, _ := strconv.Atoi(os.Getenv("DDAG_SMTP_PORT"))
	if port == 0 {
		port = 587
	}
	return &Mailer{
		Host:        os.Getenv("DDAG_SMTP_HOST"),
		Port:        port,
		User:        os.Getenv("DDAG_SMTP_USER"),
		Password:    os.Getenv("DDAG_SMTP_PASSWORD"),
		SenderMail:  os.Getenv("DDAG_SMTP_SENDER_EMAIL"),
		SenderName:  os.Getenv("DDAG_SMTP_SENDER_NAME"),
		BrevoAPIKey: os.Getenv("DDAG_BREVO_API_KEY"),
	}
}

// SendHTML sends an HTML email. Uses Brevo API v3 if key is set, otherwise SMTP.
func (m *Mailer) SendHTML(to string, subject string, bodyHTML string) error {
	if m.BrevoAPIKey != "" {
		return m.sendViaAPIv3(to, subject, bodyHTML)
	}
	return m.sendViaSMTP(to, subject, bodyHTML)
}

// sendViaAPIv3 sends using Brevo REST API v3 — preferred, more reliable
func (m *Mailer) sendViaAPIv3(to string, subject string, bodyHTML string) error {
	payload := map[string]interface{}{
		"sender": map[string]string{
			"name":  m.SenderName,
			"email": m.SenderMail,
		},
		"to":          []map[string]string{{"email": to}},
		"subject":     subject,
		"htmlContent": bodyHTML,
	}

	b, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", "https://api.brevo.com/v3/smtp/email", bytes.NewBuffer(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-key", m.BrevoAPIKey)

	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return fmt.Errorf("brevo API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("brevo API error [%d]: %s", resp.StatusCode, string(body))
	}
	return nil
}

// sendViaSMTP sends via STARTTLS SMTP (Brevo relay port 587)
func (m *Mailer) sendViaSMTP(to string, subject string, bodyHTML string) error {
	if m.Host == "" || m.User == "" || m.Password == "" {
		return fmt.Errorf("SMTP configuration is incomplete")
	}

	header := fmt.Sprintf(
		"From: \"%s\" <%s>\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=\"UTF-8\"\r\n\r\n",
		m.SenderName, m.SenderMail, to, subject,
	)
	message := header + bodyHTML

	addr := fmt.Sprintf("%s:%d", m.Host, m.Port)
	auth := smtp.PlainAuth("", m.User, m.Password, m.Host)

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to connect SMTP server: %w", err)
	}
	defer conn.Close()

	c, err := smtp.NewClient(conn, m.Host)
	if err != nil {
		return fmt.Errorf("failed to create smtp client: %w", err)
	}
	defer c.Quit()

	if m.Port == 587 {
		if err = c.StartTLS(&tls.Config{ServerName: m.Host, InsecureSkipVerify: true}); err != nil {
			return fmt.Errorf("failed to start TLS: %w", err)
		}
	}
	if err = c.Auth(auth); err != nil {
		return fmt.Errorf("SMTP authentication failed: %w", err)
	}
	if err = c.Mail(m.SenderMail); err != nil {
		return fmt.Errorf("failed to set sender: %w", err)
	}
	if err = c.Rcpt(to); err != nil {
		return fmt.Errorf("failed to set recipient: %w", err)
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err = w.Write([]byte(message)); err != nil {
		return err
	}
	return w.Close()
}
