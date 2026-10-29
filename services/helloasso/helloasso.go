package helloasso

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/boavizta/helloasso-renew-contribution/internal/domain"
)

const userAgent = "Boavizta-Renew-Contribution/1.0"

// TokenResponse represents the OAuth token response
type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

// Payment represents the payment data we're interested in
type Payment = domain.Payment

type Config struct {
	APIURL       string
	ClientID     string
	ClientSecret string
	OrgSlug      string
	FromDate     string
}

type Client struct {
	config Config
	http   *http.Client
}

func NewClient(config Config, client *http.Client) *Client {
	return &Client{config: config, http: client}
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
func (c *Client) getOAuthToken() (string, error) {
	slog.Debug("Preparing OAuth token request")
	data := url.Values{}
	data.Set("client_id", c.config.ClientID)
	data.Set("client_secret", c.config.ClientSecret)
	data.Set("grant_type", "client_credentials")

	req, err := http.NewRequest("POST", c.config.APIURL+"/oauth2/token", strings.NewReader(data.Encode()))
	if err != nil {
		slog.Error("Failed to create request", "error", err)
		return "", err
	}

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("User-Agent", userAgent)

	slog.Debug("Sending OAuth token request")
	resp, err := c.http.Do(req)
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
	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("HelloAsso returned an empty access token")
	}

	slog.Debug("OAuth token obtained successfully")
	return tokenResp.AccessToken, nil
}

// ItemResponse represents the API response for membership items
// Items are used to retrieve free memberships that do not generate payments.
type ItemResponse struct {
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
		User struct {
			FirstName string `json:"firstName"`
			LastName  string `json:"lastName"`
		} `json:"user"`
		Name   string `json:"name"`
		Amount int    `json:"amount"`
		Type   string `json:"type"`
		State  string `json:"state"`
	} `json:"data"`
	Pagination struct {
		PageSize          int    `json:"pageSize"`
		TotalCount        int    `json:"totalCount"`
		PageIndex         int    `json:"pageIndex"`
		TotalPages        int    `json:"totalPages"`
		ContinuationToken string `json:"continuationToken"`
	} `json:"pagination"`
}

// getPayments fetches payments from the HelloAsso API
func (c *Client) GetPayments() ([]Payment, error) {
	slog.Info("Getting OAuth token...")
	token, err := c.getOAuthToken()
	if err != nil {
		return nil, err
	}
	slog.Info("OAuth token obtained successfully")

	orgSlug := c.config.OrgSlug
	fromDate := c.config.FromDate

	slog.Info("Fetching payments for organization", "org", orgSlug, "from", fromDate)

	var allPayments []Payment
	pageIndex := 1

	for {
		slog.Info("Fetching page of payments", "page", pageIndex)
		apiURL := fmt.Sprintf("%s/v5/organizations/%s/payments?pageSize=100&from=%s&pageIndex=%d&states=Authorized&states=Registered",
			c.config.APIURL, url.PathEscape(orgSlug), url.QueryEscape(fromDate), pageIndex)

		req, err := http.NewRequest("GET", apiURL, nil)
		if err != nil {
			return nil, err
		}

		req.Header.Add("Authorization", "Bearer "+token)
		req.Header.Add("User-Agent", userAgent)

		resp, err := c.http.Do(req)
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
				Amount:         float64(item.Amount) / 100,
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

// GetFreeMembershipItems fetches free membership items from the HelloAsso API.
// Free memberships (e.g. "Personne Physique - Sans Cotisation" or "Individual - Free")
// do not generate payments, so they must be retrieved through the items endpoint.
func (c *Client) GetFreeMembershipItems() ([]Payment, error) {
	slog.Info("Getting OAuth token for free membership items...")
	token, err := c.getOAuthToken()
	if err != nil {
		return nil, err
	}
	slog.Info("OAuth token obtained successfully")

	orgSlug := c.config.OrgSlug
	fromDate := c.config.FromDate

	slog.Info("Fetching free membership items for organization", "org", orgSlug, "from", fromDate)

	var allItems []Payment
	pageIndex := 1

	for {
		slog.Info("Fetching page of membership items", "page", pageIndex)
		apiURL := fmt.Sprintf(
			"%s/v5/organizations/%s/items?pageSize=100&from=%s&pageIndex=%d&tierTypes=Membership&itemStates=Processed&itemStates=Registered",
			c.config.APIURL, url.PathEscape(orgSlug), url.QueryEscape(fromDate), pageIndex,
		)

		req, err := http.NewRequest("GET", apiURL, nil)
		if err != nil {
			return nil, err
		}

		req.Header.Add("Authorization", "Bearer "+token)
		req.Header.Add("User-Agent", userAgent)

		resp, err := c.http.Do(req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("failed to get membership items: %s, status code: %d", string(body), resp.StatusCode)
		}

		var itemResp ItemResponse
		if err := json.NewDecoder(resp.Body).Decode(&itemResp); err != nil {
			resp.Body.Close()
			return nil, err
		}
		resp.Body.Close()

		pageItems := len(itemResp.Data)
		slog.Info("Processing membership items from page", "page", pageIndex, "count", pageItems)

		for _, item := range itemResp.Data {
			if item.Amount != 0 {
				continue
			}

			firstName := item.Payer.FirstName
			if firstName == "" {
				firstName = item.User.FirstName
			}
			lastName := item.Payer.LastName
			if lastName == "" {
				lastName = item.User.LastName
			}

			payment := Payment{
				OrderFormSlug:  item.Order.FormSlug,
				OrderDate:      item.Order.Date,
				PayerEmail:     item.Payer.Email,
				PayerFirstName: firstName,
				PayerLastName:  lastName,
				Amount:         0,
			}
			allItems = append(allItems, payment)
		}

		slog.Info("Found membership items on page", "page", pageIndex, "count", pageItems, "total", len(allItems))

		if pageItems == 0 {
			break
		}

		pageIndex++
	}

	slog.Info("Finished fetching all free membership items", "total", len(allItems))
	return allItems, nil
}
