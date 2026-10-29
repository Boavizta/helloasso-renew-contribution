package baserow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/boavizta/helloasso-renew-contribution/internal/domain"
	"github.com/samber/lo"
)

// Member represents a member from the Baserow table with the required columns.
// Id is the Baserow row ID (auto-generated, lowercase "id" in API responses),
// used as the identifier in update/delete API calls.
type Member = domain.Member

type Config struct {
	APIURL   string
	APIToken string
	TableID  string
	DryRun   bool
}

type Client struct {
	config Config
	http   *http.Client
}

func NewClient(config Config, client *http.Client) *Client {
	return &Client{config: config, http: client}
}

// Baserow select IDs are adapter details, not domain vocabulary.
var membershipTypes = map[int]string{2521: domain.IndividualMembership, 2520: domain.OrganizationMembership}
var languages = map[int]string{2590: domain.EnglishLanguage, 2591: domain.FrenchLanguage, 2592: domain.SpanishLanguage}

// BaserowResponse represents the API response from Baserow.
type BaserowResponse struct {
	Count    int                      `json:"count"`
	Next     string                   `json:"next"`
	Previous any                      `json:"previous"`
	Results  []map[string]interface{} `json:"results"`
}

// GetMembers fetches all members from the Baserow API
func (c *Client) GetMembers() ([]Member, error) {
	slog.Info("Fetching members from Baserow")

	// Archived members are excluded server-side: they must never be read or updated by this program.
	apiURL := fmt.Sprintf("%s/api/database/rows/table/%s/?user_field_names=true&filter__Archived__boolean=false", c.config.APIURL, c.config.TableID)

	var members []Member

	// Loop to handle pagination
	for apiURL != "" {
		req, err := http.NewRequest("GET", apiURL, nil)
		if err != nil {
			slog.Error("Failed to create request", "error", err)
			return nil, err
		}

		req.Header.Add("Authorization", "Token "+c.config.APIToken)

		resp, err := c.http.Do(req)
		if err != nil {
			slog.Error("Failed to send request", "error", err)
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			slog.Error("Failed to get members", "status", resp.StatusCode, "response", string(body))
			return nil, fmt.Errorf("failed to get members: %s, status code: %d", string(body), resp.StatusCode)
		}

		var baserowResp BaserowResponse
		if err := json.NewDecoder(resp.Body).Decode(&baserowResp); err != nil {
			resp.Body.Close()
			slog.Error("Failed to decode response", "error", err)
			return nil, err
		}
		resp.Body.Close()

		// Process the results from this page
		for _, result := range baserowResp.Results {
			member := Member{
				Id:                       getIntValue(result, "id"),
				Surname:                  getStringValue(result, "Surname"),
				FirstName:                getStringValue(result, "First name"),
				Email:                    getStringValue(result, "E-mail"),
				AlternativeEmail1:        getStringValue(result, "AlternativeEmail1"),
				AlternativeEmail2:        getStringValue(result, "AlternativeEmail2"),
				Country:                  getLinkedValue(result, "Country"),
				ActiveMembership:         getBoolValue(result, "Active MemberShip"),
				NumberContributionsEmail: getIntValue(result, "Number of Contributions Email"),
				MembershipType:           membershipTypes[getSelectId(result, "Membership type")],
				PreferredLanguages: lo.FilterMap(getMultiSelectIds(result, "Preferred languages"), func(id int, _ int) (string, bool) {
					language, ok := languages[id]
					return language, ok
				}),
			}

			// Handle the date fields separately as they require parsing
			if dateStr, ok := result["Last Payment Date"].(string); ok && dateStr != "" {
				date, err := time.Parse("2006-01-02", dateStr)
				if err == nil {
					member.LastPaymentDate = date
				}
			}

			if dateStr, ok := result["Last Contribution Email Date"].(string); ok && dateStr != "" {
				date, err := time.Parse("2006-01-02", dateStr)
				if err == nil {
					member.LastContributionEmailDate = date
				}
			}

			members = append(members, member)
		}

		// Update URL for the next page or exit the loop if there's no next page
		apiURL = baserowResp.Next

		if apiURL != "" {
			slog.Info("Fetching next page of members", "url", apiURL)
		}
	}

	slog.Info("Successfully fetched all members from Baserow", "count", len(members))
	return members, nil
}

// Helper functions to safely extract values from the map
func getStringValue(data map[string]interface{}, key string) string {
	if val, ok := data[key].(string); ok {
		return val
	}
	return ""
}

func getLinkedValue(data map[string]interface{}, key string) string {
	// Handle string value
	if val, ok := data[key].(string); ok {
		return val
	}

	// Handle map value (single linked item)
	if val, ok := data[key].(map[string]interface{}); ok {
		if value, ok := val["value"].(string); ok {
			return value
		}
	}

	// Handle array value (multiple linked items)
	if val, ok := data[key].([]interface{}); ok && len(val) > 0 {
		// Return the first item's value
		if itemMap, ok := val[0].(map[string]interface{}); ok {
			if value, ok := itemMap["value"].(string); ok {
				return value
			}
		}
	}

	return ""
}

func getSelectId(data map[string]interface{}, key string) int {
	if val, ok := data[key].(map[string]interface{}); ok {
		if value, ok := val["id"].(float64); ok {
			return int(value)
		}
	}
	return 0
}

func getBoolValue(data map[string]interface{}, key string) bool {
	if val, ok := data[key].(bool); ok {
		return val
	}
	return false
}

func getIntValue(data map[string]interface{}, key string) int {
	if val, ok := data[key].(float64); ok {
		return int(val)
	}
	return 0
}

func getMultiSelectIds(data map[string]interface{}, key string) []int {
	var ids []int
	if val, ok := data[key].([]interface{}); ok {
		for _, item := range val {
			if itemMap, ok := item.(map[string]interface{}); ok {
				if id, ok := itemMap["id"].(float64); ok {
					ids = append(ids, int(id))
				}
			}
		}
	}
	return ids
}

// UpdateMember updates a member's information in the Baserow database
func (c *Client) UpdateMember(member Member) error {
	slog.Debug("Updating member in Baserow", "id", member.Id, "email", member.Email)

	apiURL := fmt.Sprintf("%s/api/database/rows/table/%s/%d/?user_field_names=true", c.config.APIURL, c.config.TableID, member.Id)

	// Prepare the update payload
	payload := map[string]interface{}{
		"Active MemberShip":             member.ActiveMembership,
		"Last Payment Date":             dateValue(member.LastPaymentDate),
		"Last Contribution Email Date":  dateValue(member.LastContributionEmailDate),
		"Number of Contributions Email": member.NumberContributionsEmail,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		slog.Error("Failed to marshal update payload", "error", err)
		return err
	}

	if c.config.DryRun {
		slog.Info("DRY_RUN: skipping Baserow update", "id", member.Id, "email", member.Email, "payload", string(payloadBytes))
		return nil
	}

	req, err := http.NewRequest("PATCH", apiURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		slog.Error("Failed to create update request", "error", err)
		return err
	}

	req.Header.Add("Authorization", "Token "+c.config.APIToken)
	req.Header.Add("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		slog.Error("Failed to send update request", "error", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		slog.Error("Failed to update member", "status", resp.StatusCode, "response", string(body))
		return fmt.Errorf("failed to update member: %s, status code: %d", string(body), resp.StatusCode)
	}

	slog.Info("Successfully updated member in Baserow", "id", member.Id, "email", member.Email)
	return nil
}

func dateValue(date time.Time) any {
	if date.IsZero() {
		return nil
	}
	return date.Format(time.DateOnly)
}
