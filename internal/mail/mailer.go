package mail

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"os"
	"strconv"
)

type Mailer struct {
	Host       string
	Port       int
	User       string
	Password   string
	SenderName string
	SenderMail string
}

func NewMailerFromEnv() *Mailer {
	port, _ := strconv.Atoi(os.Getenv("DDAG_SMTP_PORT"))
	if port == 0 {
		port = 587
	}

	return &Mailer{
		Host:       os.Getenv("DDAG_SMTP_HOST"),
		Port:       port,
		User:       os.Getenv("DDAG_SMTP_USER"),
		Password:   os.Getenv("DDAG_SMTP_PASSWORD"),
		SenderMail: os.Getenv("DDAG_SMTP_SENDER_EMAIL"),
		SenderName: os.Getenv("DDAG_SMTP_SENDER_NAME"),
	}
}

// SendHTML sends an HTML format email using SMTP Login auth (which Brevo uses)
func (m *Mailer) SendHTML(to string, subject string, bodyHTML string) error {
	if m.Host == "" || m.User == "" || m.Password == "" {
		return fmt.Errorf("SMTP configuration is incomplete in environment variables")
	}

	// Prepare email headers & content
	header := make(map[string]string)
	header["From"] = fmt.Sprintf("\"%s\" <%s>", m.SenderName, m.SenderMail)
	header["To"] = to
	header["Subject"] = subject
	header["MIME-Version"] = "1.0"
	header["Content-Type"] = "text/html; charset=\"UTF-8\""

	message := ""
	for k, v := range header {
		message += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	message += "\r\n" + bodyHTML

	addr := fmt.Sprintf("%s:%d", m.Host, m.Port)

	// Brevo SMTP requires STARTTLS on port 587
	auth := smtp.PlainAuth("", m.User, m.Password, m.Host)

	// Connect to SMTP Server
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

	// Upgrade to TLS if using STARTTLS (port 587)
	if m.Port == 587 {
		tlsconfig := &tls.Config{
			ServerName:         m.Host,
			InsecureSkipVerify: true, // safe for standard relays
		}
		if err = c.StartTLS(tlsconfig); err != nil {
			return fmt.Errorf("failed to start TLS: %w", err)
		}
	}

	// Authenticate
	if err = c.Auth(auth); err != nil {
		return fmt.Errorf("SMTP authentication failed: %w", err)
	}

	// Set Sender
	if err = c.Mail(m.SenderMail); err != nil {
		return fmt.Errorf("failed to set sender: %w", err)
	}

	// Set Recipient
	if err = c.Rcpt(to); err != nil {
		return fmt.Errorf("failed to set recipient: %w", err)
	}

	// Send Body
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("failed to open data writer: %w", err)
	}
	_, err = w.Write([]byte(message))
	if err != nil {
		return fmt.Errorf("failed to write message body: %w", err)
	}
	err = w.Close()
	if err != nil {
		return fmt.Errorf("failed to close data writer: %w", err)
	}

	return nil
}
