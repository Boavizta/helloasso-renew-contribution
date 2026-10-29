package config_test

import (
	"strings"
	"testing"

	"github.com/boavizta/helloasso-renew-contribution/internal/config"
)

func environment() map[string]string {
	return map[string]string{
		"HELLOASSO_API_ID": "id", "HELLOASSO_API_SECRET": "secret", "HELLOASSO_FROM_DATE": "2024-01-01",
		"HELLOASSO_ORG_SLUG": "org", "BASEROW_API_TOKEN": "token", "BASEROW_MEMBER_TABLE_ID": "42", "BREVO_API_KEY": "key",
	}
}

func TestLoadDefaults(t *testing.T) {
	t.Parallel()
	env := environment()
	cfg, err := config.Load(func(key string) string { return env[key] })
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DryRun || cfg.HelloAsso.ClientID != "id" || cfg.HelloAsso.APIURL != "https://api.helloasso.com" || cfg.Baserow.TableID != "42" || cfg.Email.SenderEmail != "no-reply@boavizta.org" {
		t.Fatalf("config=%+v", cfg)
	}
}

func TestLoadRequired(t *testing.T) {
	t.Parallel()
	for name := range environment() {
		t.Run(name, func(t *testing.T) {
			env := environment()
			delete(env, name)
			_, err := config.Load(func(key string) string { return env[key] })
			if err == nil || !strings.Contains(err.Error(), name) {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestLoadInvalid(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct{ key, value string }{
		{"DRY_RUN", "tru"}, {"HELLOASSO_FROM_DATE", "2024-02-30"}, {"BASEROW_MEMBER_TABLE_ID", "0"},
		{"BASEROW_MEMBER_TABLE_ID", "abc"}, {"HELLOASSO_API_URL", "relative"}, {"BASEROW_API_URL", "ftp://test.org"},
		{"BREVO_API_URL", "https://test.org?token=secret"}, {"RENEWAL_FRENCH_LINK", "https://test.org/#fragment"},
		{"RENEWAL_ENGLISH_LINK", "https://user:secret@test.org"}, {"RENEWAL_SENDER_EMAIL", "invalid"},
	} {
		t.Run(tt.key+tt.value, func(t *testing.T) {
			env := environment()
			env[tt.key] = tt.value
			_, err := config.Load(func(key string) string { return env[key] })
			if err == nil || !strings.Contains(err.Error(), tt.key) {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestLoadOverrides(t *testing.T) {
	t.Parallel()
	env := environment()
	for key, value := range map[string]string{
		"DRY_RUN": "true", "HELLOASSO_API_URL": "http://localhost:1234/", "BASEROW_API_URL": "https://rows.test",
		"BREVO_API_URL": "https://mail.test", "RENEWAL_SENDER_NAME": "Team", "RENEWAL_SENDER_EMAIL": "team@test.org",
		"RENEWAL_FRENCH_LINK": "https://test.org/fr", "RENEWAL_ENGLISH_LINK": "https://test.org/en",
	} {
		env[key] = value
	}
	cfg, err := config.Load(func(key string) string { return env[key] })
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.DryRun || !cfg.Baserow.DryRun || !cfg.Brevo.DryRun || cfg.HelloAsso.APIURL != "http://localhost:1234" || cfg.Email.SenderName != "Team" || cfg.Email.FrenchLink != "https://test.org/fr" {
		t.Fatalf("config=%+v", cfg)
	}
}
