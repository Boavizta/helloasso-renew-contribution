package baserow_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/boavizta/helloasso-renew-contribution/internal/domain"
	"github.com/boavizta/helloasso-renew-contribution/services/baserow"
)

func TestGetMembersPaginationAndMapping(t *testing.T) {
	t.Parallel()
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Authorization") != "Token token" || r.Method != http.MethodGet {
			t.Errorf("request=%+v", r)
		}
		w.Header().Set("Content-Type", "application/json")
		if calls == 1 {
			if r.URL.Path != "/api/database/rows/table/42/" || r.URL.Query().Get("filter__Archived__boolean") != "false" || r.URL.Query().Get("user_field_names") != "true" {
				t.Errorf("query=%s", r.URL)
			}
			fmt.Fprintf(w, `{"next":"http://%s/page2","results":[{"id":7,"Surname":"Smith","First name":"Alice","E-mail":"a@test.org","AlternativeEmail1":"b@test.org","AlternativeEmail2":"c@test.org","Country":[{"value":"France"}],"Membership type":{"id":2521},"Preferred languages":[{"id":2591}],"Active MemberShip":true,"Last Payment Date":"2026-10-01","Last Contribution Email Date":"2026-09-01","Number of Contributions Email":2}]}`, r.Host)
		} else {
			fmt.Fprint(w, `{"next":null,"results":[{"id":8,"Country":{"value":"Spain"},"Last Payment Date":"invalid"},{"id":9,"Country":"Germany"},{}]}`)
		}
	}))
	defer server.Close()
	client := baserow.NewClient(baserow.Config{APIURL: server.URL, APIToken: "token", TableID: "42"}, server.Client())
	got, err := client.GetMembers()
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || len(got) != 4 {
		t.Fatalf("calls=%d members=%+v", calls, got)
	}
	m := got[0]
	if m.Id != 7 || m.Email != "a@test.org" || m.FirstName != "Alice" || m.Surname != "Smith" || m.AlternativeEmail1 != "b@test.org" || m.AlternativeEmail2 != "c@test.org" || !m.ActiveMembership || m.Country != "France" || m.MembershipType != domain.IndividualMembership || len(m.PreferredLanguages) != 1 || m.PreferredLanguages[0] != domain.FrenchLanguage || m.NumberContributionsEmail != 2 || m.LastPaymentDate.Format(time.DateOnly) != "2026-10-01" || m.LastContributionEmailDate.Format(time.DateOnly) != "2026-09-01" {
		t.Fatalf("member=%+v", m)
	}
	if got[1].Country != "Spain" || !got[1].LastPaymentDate.IsZero() || got[2].Country != "Germany" || got[3].Country != "" {
		t.Fatalf("members=%+v", got)
	}
}

func TestUpdateMember(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name    string
		status  int
		dry     bool
		wantErr bool
	}{
		{"success", 200, false, false}, {"API error", 500, false, true}, {"dry run", 500, true, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != http.MethodPatch || r.URL.Path != "/api/database/rows/table/42/7/" || r.Header.Get("Authorization") != "Token token" || r.URL.Query().Get("user_field_names") != "true" {
					t.Errorf("request=%+v", r)
				}
				var payload map[string]any
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Error(err)
				}
				if len(payload) != 4 || payload["Active MemberShip"] != true || payload["Last Payment Date"] != "2026-10-01" || payload["Last Contribution Email Date"] != nil || payload["Number of Contributions Email"] != float64(0) {
					t.Errorf("payload=%+v", payload)
				}
				w.WriteHeader(tt.status)
			}))
			defer server.Close()
			client := baserow.NewClient(baserow.Config{APIURL: server.URL, APIToken: "token", TableID: "42", DryRun: tt.dry}, server.Client())
			member := domain.Member{Id: 7, ActiveMembership: true, LastPaymentDate: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)}
			if err := client.UpdateMember(member); (err != nil) != tt.wantErr {
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

func TestGetMembersErrors(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name, body string
		status     int
	}{{"API error", "denied", 403}, {"invalid JSON", "{", 200}} {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tt.status); fmt.Fprint(w, tt.body) }))
			defer server.Close()
			client := baserow.NewClient(baserow.Config{APIURL: server.URL}, server.Client())
			if _, err := client.GetMembers(); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}
