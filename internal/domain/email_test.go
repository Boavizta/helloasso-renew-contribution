package domain_test

import (
	"strings"
	"testing"

	"github.com/boavizta/helloasso-renew-contribution/internal/domain"
)

func TestRenewalEmail(t *testing.T) {
	t.Parallel()
	settings := domain.EmailSettings{SenderName: "Team", SenderEmail: "sender@test.org", FrenchLink: "https://test.org/fr", EnglishLink: "https://test.org/en"}
	for _, tt := range []struct {
		name      string
		country   string
		languages []string
		greeting  string
		link      string
	}{
		{"default English", "", nil, "Dear", settings.EnglishLink},
		{"French country", "France", nil, "Cher(e)", settings.FrenchLink},
		{"French preference", "Spain", []string{domain.EnglishLanguage, domain.FrenchLanguage}, "Cher(e)", settings.FrenchLink},
	} {
		t.Run(tt.name, func(t *testing.T) {
			member := domain.Member{Email: "a@test.org", FirstName: "  ÉLODIE   MARIE ", Surname: "Smith", Country: tt.country, PreferredLanguages: tt.languages}
			got := domain.RenewalEmail(member, settings)
			if got.ToName != "Élodie Marie Smith" || got.ToEmail != member.Email || got.SenderEmail != settings.SenderEmail || got.SenderName != settings.SenderName || got.Subject == "" {
				t.Fatalf("incorrect envelope: %+v", got)
			}
			for _, content := range []string{got.HtmlContent, got.TextContent} {
				if !strings.Contains(content, tt.greeting) || !strings.Contains(content, tt.link) || !strings.Contains(content, "Élodie Marie") {
					t.Fatalf("incorrect content: %s", content)
				}
			}
		})
	}
}

func TestRenewalEmailEscapesHTML(t *testing.T) {
	t.Parallel()
	got := domain.RenewalEmail(domain.Member{FirstName: "<SCRIPT>"}, domain.EmailSettings{})
	if strings.Contains(got.HtmlContent, "<script>") || !strings.Contains(got.HtmlContent, "&lt;script&gt;") {
		t.Fatalf("unsafe HTML: %s", got.HtmlContent)
	}
	if empty := domain.RenewalEmail(domain.Member{}, domain.EmailSettings{}); !strings.Contains(empty.TextContent, "Dear ,") {
		t.Fatal("empty name should render without panic")
	}
}
