package helloasso

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// TokenResponse represents the OAuth token response
type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

// Payment represents the payment data we're interested in
type Payment struct {
	OrderFormSlug  string    `json:"orderFormSlug"`
	OrderDate      time.Time `json:"orderDate"`
	PayerEmail     string    `json:"payerEmail"`
	PayerFirstName string    `json:"payerFirstName"`
	PayerLastName  string    `json:"payerLastName"`
}

// PaymentResponse represents the API response for payments
type PaymentResponse struct {
	Data []struct {
		Order struct {
			ID       int       `json:"id"`
			Date     time.Time `json:"date"`
			FormSlug string    `json:"formSlug"`
			FormType string    `json:"formType"`
		} `json:"order"`
		Payer struct {
			Email     string `json:"email"`
			Country   string `json:"country"`
			FirstName string `json:"firstName"`
			LastName  string `json:"lastName"`
		} `json:"payer"`
		Items []struct {
			ID     int    `json:"id"`
			Amount int    `json:"amount"`
			Type   string `json:"type"`
			State  string `json:"state"`
		} `json:"items"`
		ID     int       `json:"id"`
		Amount int       `json:"amount"`
		Date   time.Time `json:"date"`
		State  string    `json:"state"`
	} `json:"data"`
	Pagination struct {
		PageSize          int    `json:"pageSize"`
		TotalCount        int    `json:"totalCount"`
		PageIndex         int    `json:"pageIndex"`
		TotalPages        int    `json:"totalPages"`
		ContinuationToken string `json:"continuationToken"`
	} `json:"pagination"`
}

// getOAuthToken gets an OAuth token from the HelloAsso API
func getOAuthToken() (string, error) {
	clientID := os.Getenv("HELLOASSO_API_ID")
	clientSecret := os.Getenv("HELLOASSO_API_SECRET")

	if clientID == "" || clientSecret == "" {
		slog.Error("Missing environment variables", "variables", "HELLOASSO_API_ID and HELLOASSO_API_SECRET")
		return "", fmt.Errorf("HELLOASSO_API_ID and HELLOASSO_API_SECRET environment variables must be set")
	}

	slog.Debug("Preparing OAuth token request")
	data := url.Values{}
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)
	data.Set("grant_type", "client_credentials")

	req, err := http.NewRequest("POST", "https://api.helloasso.com/oauth2/token", strings.NewReader(data.Encode()))
	if err != nil {
		slog.Error("Failed to create request", "error", err)
		return "", err
	}

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	slog.Debug("Sending OAuth token request")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		slog.Error("Failed to send request", "error", err)
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		slog.Error("Failed to get token", "status", resp.StatusCode, "response", string(body))
		return "", fmt.Errorf("failed to get token: %s, status code: %d", string(body), resp.StatusCode)
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		slog.Error("Failed to decode token response", "error", err)
		return "", err
	}

	slog.Debug("OAuth token obtained successfully")
	return tokenResp.AccessToken, nil
}

// getPayments fetches payments from the HelloAsso API
func GetPayments() ([]Payment, error) {
	slog.Info("Getting OAuth token...")
	token, err := getOAuthToken()
	if err != nil {
		return nil, err
	}
	slog.Info("OAuth token obtained successfully")

	orgSlug := os.Getenv("HELLOASSO_ORG_SLUG")
	fromDate := os.Getenv("HELLOASSO_FROM_DATE")

	if orgSlug == "" || fromDate == "" {
		return nil, fmt.Errorf("HELLOASSO_ORG_SLUG and HELLOASSO_FROM_DATE environment variables must be set")
	}

	slog.Info("Fetching payments for organization", "org", orgSlug, "from", fromDate)

	var allPayments []Payment
	pageIndex := 1

	for {
		slog.Info("Fetching page of payments", "page", pageIndex)
		apiURL := fmt.Sprintf("https://api.helloasso.com/v5/organizations/%s/payments?pageSize=100&from=%s&pageIndex=%d&states=Authorized&states=Registered",
			orgSlug, fromDate, pageIndex)

		req, err := http.NewRequest("GET", apiURL, nil)
		if err != nil {
			return nil, err
		}

		req.Header.Add("Authorization", "Bearer "+token)

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("failed to get payments: %s, status code: %d", string(body), resp.StatusCode)
		}

		var paymentResp PaymentResponse
		if err := json.NewDecoder(resp.Body).Decode(&paymentResp); err != nil {
			resp.Body.Close()
			return nil, err
		}
		resp.Body.Close()

		// Extract the fields we care about
		pagePayments := len(paymentResp.Data)
		slog.Info("Processing payments from page", "page", pageIndex, "count", pagePayments)

		for _, item := range paymentResp.Data {
			payment := Payment{
				OrderFormSlug:  item.Order.FormSlug,
				OrderDate:      item.Order.Date,
				PayerEmail:     item.Payer.Email,
				PayerFirstName: item.Payer.FirstName,
				PayerLastName:  item.Payer.LastName,
			}
			allPayments = append(allPayments, payment)
		}

		slog.Info("Found payments on page", "page", pageIndex, "count", pagePayments, "total", len(allPayments))

		// Check if we've processed all pages
		// If no data was returned or we've reached the end, break
		if pagePayments == 0 {
			break
		}

		// Move to the next page
		pageIndex++
	}

	slog.Info("Finished fetching all payments", "total", len(allPayments))
	return allPayments, nil
}
