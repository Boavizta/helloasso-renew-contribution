package helloasso_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/boavizta/helloasso-renew-contribution/services/helloasso"
)

func TestContributionsPaginationAndMapping(t *testing.T) {
	t.Parallel()
	for _, free := range []bool{false, true} {
		name := "payments"
		if free {
			name = "items"
		}
		t.Run(name, func(t *testing.T) {
			pages, tokens := 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("User-Agent") != "Boavizta-Renew-Contribution/1.0" {
					t.Error("missing user agent")
				}
				if r.URL.Path == "/oauth2/token" {
					tokens++
					if err := r.ParseForm(); err != nil {
						t.Error(err)
					}
					if r.Method != http.MethodPost || r.Form.Get("client_id") != "id" || r.Form.Get("client_secret") != "secret" || r.Form.Get("grant_type") != "client_credentials" {
						t.Errorf("OAuth request=%+v", r.Form)
					}
					fmt.Fprint(w, `{"access_token":"token"}`)
					return
				}
				pages++
				q := r.URL.Query()
				if r.Method != http.MethodGet || r.URL.Path != "/v5/organizations/org/"+name || r.Header.Get("Authorization") != "Bearer token" || q.Get("from") != "2024-01-01" || q.Get("pageIndex") != fmt.Sprint(pages) || q.Get("pageSize") != "100" {
					t.Errorf("request=%+v", r)
				}
				if pages > 1 {
					fmt.Fprint(w, `{"data":[]}`)
					return
				}
				if free {
					if q.Get("tierTypes") != "Membership" || len(q["itemStates"]) != 2 {
						t.Errorf("query=%s", q)
					}
					fmt.Fprint(w, `{"data":[{"order":{"date":"2026-10-01T00:00:00Z","formSlug":"annual-membership-fee"},"payer":{"email":"free@test.org"},"user":{"firstName":"Alice","lastName":"Smith"},"amount":0},{"order":{"date":"2026-10-01T00:00:00Z","formSlug":"cotisation-annuelle"},"payer":{"email":"other@test.org","firstName":"Bob","lastName":"Jones"},"user":{"firstName":"Ignored","lastName":"Ignored"},"amount":0},{"amount":10000}]}`)
				} else {
					if len(q["states"]) != 2 {
						t.Errorf("query=%s", q)
					}
					fmt.Fprint(w, `{"data":[{"order":{"date":"2026-10-01T00:00:00Z","formSlug":"cotisation-annuelle"},"payer":{"email":"paid@test.org","firstName":"Alice","lastName":"Smith"},"amount":12345}]}`)
				}
			}))
			defer server.Close()
			client := helloasso.NewClient(helloasso.Config{APIURL: server.URL, ClientID: "id", ClientSecret: "secret", OrgSlug: "org", FromDate: "2024-01-01"}, server.Client())
			var got []helloasso.Payment
			var err error
			if free {
				got, err = client.GetFreeMembershipItems()
			} else {
				got, err = client.GetPayments()
			}
			if err != nil {
				t.Fatal(err)
			}
			if tokens != 1 || pages != 2 {
				t.Fatalf("tokens=%d pages=%d", tokens, pages)
			}
			if len(got) == 0 {
				t.Fatal("missing contributions")
			}
			p := got[0]
			if p.PayerFirstName != "Alice" || p.PayerLastName != "Smith" || p.OrderDate.Format(time.DateOnly) != "2026-10-01" {
				t.Fatalf("payment=%+v", p)
			}
			if !free && (len(got) != 1 || p.Amount != 123.45 || p.PayerEmail != "paid@test.org" || p.OrderFormSlug != "cotisation-annuelle") {
				t.Fatalf("payments=%+v", got)
			}
			if free && (len(got) != 2 || p.Amount != 0 || p.PayerEmail != "free@test.org" || got[1].PayerFirstName != "Bob" || got[1].PayerLastName != "Jones") {
				t.Fatalf("free=%+v", got)
			}
		})
	}
}

func TestContributionErrors(t *testing.T) {
	t.Parallel()
	for _, free := range []bool{false, true} {
		for _, tt := range []struct {
			name        string
			tokenBody   string
			tokenStatus int
			dataBody    string
			dataStatus  int
		}{
			{"OAuth denied", "denied", 401, "", 200},
			{"OAuth invalid JSON", "{", 200, "", 200},
			{"OAuth empty token", `{}`, 200, "", 200},
			{"API error", `{"access_token":"token"}`, 200, "denied", 403},
			{"invalid JSON", `{"access_token":"token"}`, 200, "{", 200},
		} {
			t.Run(fmt.Sprintf("%v/%s", free, tt.name), func(t *testing.T) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/oauth2/token" {
						w.WriteHeader(tt.tokenStatus)
						fmt.Fprint(w, tt.tokenBody)
						return
					}
					w.WriteHeader(tt.dataStatus)
					fmt.Fprint(w, tt.dataBody)
				}))
				defer server.Close()
				client := helloasso.NewClient(helloasso.Config{APIURL: server.URL}, server.Client())
				var err error
				if free {
					_, err = client.GetFreeMembershipItems()
				} else {
					_, err = client.GetPayments()
				}
				if err == nil {
					t.Fatal("expected error")
				}
			})
		}
	}
}
