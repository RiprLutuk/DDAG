package main

import (
	"fmt"
	"log"
	"github.com/ddag/ddag/internal/mail"
	"github.com/joho/godotenv"
)

func main() {
	// Load env from configs
	err := godotenv.Load("configs/.env")
	if err != nil {
		log.Println("No configs/.env file found, using system environments")
	}

	mailer := mail.NewMailerFromEnv()

	to := "rizqy.pra85@gmail.com"
	subject := "DDAG SMTP Verification Test"
	body := `
		<div style="font-family: sans-serif; padding: 20px; border: 1px solid #eee; border-radius: 8px; max-width: 500px;">
			<h2 style="color: #f59e0b; margin-top: 0;">DDAG Mailer Active</h2>
			<p>Halo Her,</p>
			<p>Email ini dikirim otomatis oleh system DDAG menggunakan <strong>Brevo SMTP Relay</strong>.</p>
			<hr style="border: 0; border-top: 1px solid #eee; margin: 20px 0;">
			<p style="font-size: 12px; color: #888;">Dynamic Database API Gateway · tangerang, Indonesia</p>
		</div>
	`

	fmt.Printf("Sending test email to %s via %s...\n", to, mailer.Host)
	err = mailer.SendHTML(to, subject, body)
	if err != nil {
		log.Fatalf("❌ Error sending email: %v\n", err)
	}

	fmt.Println("✅ Success! Test email has been sent successfully. Check your inbox/spam folder!")
}
