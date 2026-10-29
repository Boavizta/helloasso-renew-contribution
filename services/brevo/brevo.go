package brevo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/boavizta/helloasso-renew-contribution/internal/domain"
)

// EmailData represents the data needed to send an email
type EmailData = domain.Email

type Config struct {
	APIURL string
	APIKey string
	DryRun bool
}

type Client struct {
	config Config
	http   *http.Client
}

func NewClient(config Config, client *http.Client) *Client {
	return &Client{config: config, http: client}
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
func (c *Client) SendEmail(data EmailData) error {
	slog.Info("Preparing to send email", "to", data.ToEmail)

	if c.config.DryRun {
		slog.Info("DRY_RUN: skipping email send", "to", data.ToEmail, "subject", data.Subject)
		return nil
	}

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
	req, err := http.NewRequest("POST", c.config.APIURL+"/v3/smtp/email", bytes.NewBuffer(jsonData))
	if err != nil {
		slog.Error("Failed to create request", "error", err)
		return err
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-key", c.config.APIKey)

	// Send the request
	resp, err := c.http.Do(req)
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
