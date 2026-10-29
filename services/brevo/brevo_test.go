package brevo_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/boavizta/helloasso-renew-contribution/internal/domain"
	"github.com/boavizta/helloasso-renew-contribution/services/brevo"
)

func TestSendEmail(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name    string
		status  int
		dry     bool
		wantErr bool
	}{
		{"success", http.StatusCreated, false, false},
		{"API error", http.StatusBadRequest, false, true},
		{"dry run", http.StatusInternalServerError, true, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			data := domain.Email{SenderName: "Team", SenderEmail: "sender@test.org", ToEmail: "to@test.org", ToName: "Name", Subject: "Renewal", HtmlContent: "<p>Hello</p>", TextContent: "Hello"}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != http.MethodPost || r.URL.Path != "/v3/smtp/email" || r.Header.Get("api-key") != "key" || r.Header.Get("Content-Type") != "application/json" {
					t.Errorf("request=%+v", r)
				}
				var body brevo.SendEmailRequest
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if body.Sender.Email != data.SenderEmail || len(body.To) != 1 || body.To[0].Email != data.ToEmail || body.Subject != data.Subject || body.HtmlContent != data.HtmlContent || body.TextContent != data.TextContent {
					t.Errorf("body=%+v", body)
				}
				w.WriteHeader(tt.status)
			}))
			defer server.Close()
			client := brevo.NewClient(brevo.Config{APIURL: server.URL, APIKey: "key", DryRun: tt.dry}, server.Client())
			if err := client.SendEmail(data); (err != nil) != tt.wantErr {
				t.Fatalf("err=%v", err)
			}
			wantCalls := 1
			if tt.dry {
				wantCalls = 0
			}
			if calls != wantCalls {
				t.Fatalf("calls=%d", calls)
			}
		})
	}
}

func TestSendEmailTransportFailure(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	server.Close()
	client := brevo.NewClient(brevo.Config{APIURL: server.URL}, server.Client())
	if err := client.SendEmail(domain.Email{}); err == nil {
		t.Fatal("expected transport error")
	}
}
