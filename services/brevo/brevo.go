package brevo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
)

// EmailData represents the data needed to send an email
type EmailData struct {
	SenderName  string
	SenderEmail string
	ToEmail     string
	ToName      string
	Subject     string
	HtmlContent string
	TextContent string
}

// SendEmailRequest represents the request body for the Brevo API
type SendEmailRequest struct {
	Sender      Sender      `json:"sender"`
	To          []Recipient `json:"to"`
	Subject     string      `json:"subject"`
	HtmlContent string      `json:"htmlContent"`
	TextContent string      `json:"textContent"`
}

// Sender represents the email sender
type Sender struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// Recipient represents an email recipient
type Recipient struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

// SendEmail sends an email using the Brevo API
func SendEmail(data EmailData) error {
	apiKey := os.Getenv("BREVO_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("BREVO_API_KEY environment variable must be set")
	}

	slog.Info("Preparing to send email", "to", data.ToEmail)

	// Prepare the request body
	reqBody := SendEmailRequest{
		Sender: Sender{
			Name:  data.SenderName,
			Email: data.SenderEmail,
		},
		To: []Recipient{
			{
				Email: data.ToEmail,
				Name:  data.ToName,
			},
		},
		Subject:     data.Subject,
		HtmlContent: data.HtmlContent,
		TextContent: data.TextContent,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		slog.Error("Failed to marshal request body", "error", err)
		return err
	}

	// Create the HTTP request
	req, err := http.NewRequest("POST", "https://api.sendinblue.com/v3/smtp/email", bytes.NewBuffer(jsonData))
	if err != nil {
		slog.Error("Failed to create request", "error", err)
		return err
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-key", apiKey)

	// Send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		slog.Error("Failed to send request", "error", err)
		return err
	}
	defer resp.Body.Close()

	// Check the response
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		slog.Error("Failed to send email", "status", resp.StatusCode, "response", string(body))
		return fmt.Errorf("failed to send email: %s, status code: %d", string(body), resp.StatusCode)
	}

	slog.Info("Email sent successfully", "to", data.ToEmail)
	return nil
}
