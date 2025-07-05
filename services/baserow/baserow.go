package baserow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"
)

// Member represents a member from the Baserow table with the required columns
type Member struct {
	Id                        int       `json:"Id"`
	Surname                   string    `json:"Surname"`
	FirstName                 string    `json:"First name"`
	Email                     string    `json:"E-mail"`
	ActiveMembership          bool      `json:"Active MemberShip"`
	LastPaymentDate           time.Time `json:"Last Payment Date"`
	LastContributionEmailDate time.Time `json:"Last Contribution Email Date"`
	NumberContributionsEmail  int       `json:"Number of Contributions Email"`
	MembershipType            int       `json:"Membership Type"`
	PreferredLanguages        []int     `json:"Preferred languages"`
}

// BaserowResponse represents the API response from Baserow
type BaserowResponse struct {
	Count    int                      `json:"count"`
	Next     string                   `json:"next"`
	Previous any                      `json:"previous"`
	Results  []map[string]interface{} `json:"results"`
}

// GetMembers fetches all members from the Baserow API
func GetMembers() ([]Member, error) {
	slog.Info("Fetching members from Baserow")

	apiToken := os.Getenv("BASEROW_API_TOKEN")
	if apiToken == "" {
		return nil, fmt.Errorf("BASEROW_API_TOKEN environment variable must be set")
	}

	tableID := os.Getenv("BASEROW_MEMBER_TABLE_ID")
	if tableID == "" {
		return nil, fmt.Errorf("BASEROW_MEMBER_TABLE_ID environment variable must be set")
	}
	apiURL := fmt.Sprintf("https://baserow.boavizta.org/api/database/rows/table/%s/?user_field_names=true", tableID)

	client := &http.Client{}
	var members []Member

	// Loop to handle pagination
	for apiURL != "" {
		req, err := http.NewRequest("GET", apiURL, nil)
		if err != nil {
			slog.Error("Failed to create request", "error", err)
			return nil, err
		}

		req.Header.Add("Authorization", "Token "+apiToken)

		resp, err := client.Do(req)
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
				Id:                       getIntValue(result, "Id"),
				Surname:                  getStringValue(result, "Surname"),
				FirstName:                getStringValue(result, "First name"),
				Email:                    getStringValue(result, "E-mail"),
				ActiveMembership:         getBoolValue(result, "Active MemberShip"),
				NumberContributionsEmail: getIntValue(result, "Number of Contributions Email"),
				MembershipType:           getSelectId(result, "Membership type"),
				PreferredLanguages:       getMultiSelectIds(result, "Preferred languages"),
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
func UpdateMember(member Member) error {
	slog.Debug("Updating member in Baserow", "id", member.Id, "email", member.Email)

	apiToken := os.Getenv("BASEROW_API_TOKEN")
	if apiToken == "" {
		return fmt.Errorf("BASEROW_API_TOKEN environment variable must be set")
	}

	tableID := os.Getenv("BASEROW_MEMBER_TABLE_ID")
	if tableID == "" {
		return fmt.Errorf("BASEROW_MEMBER_TABLE_ID environment variable must be set")

	}
	apiURL := fmt.Sprintf("https://baserow.boavizta.org/api/database/rows/table/%s/%d/?user_field_names=true", tableID, member.Id)

	// Prepare the update payload
	payload := map[string]interface{}{
		"Active MemberShip":             member.ActiveMembership,
		"Last Payment Date":             member.LastPaymentDate.Format("2006-01-02"),
		"Last Contribution Email Date":  member.LastContributionEmailDate.Format("2006-01-02"),
		"Number of Contributions Email": member.NumberContributionsEmail,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		slog.Error("Failed to marshal update payload", "error", err)
		return err
	}

	client := &http.Client{}
	req, err := http.NewRequest("PATCH", apiURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		slog.Error("Failed to create update request", "error", err)
		return err
	}

	req.Header.Add("Authorization", "Token "+apiToken)
	req.Header.Add("Content-Type", "application/json")

	resp, err := client.Do(req)
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
