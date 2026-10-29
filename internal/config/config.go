// Package config loads and validates environment settings before any API calls.
package config

import (
	"fmt"
	"net/mail"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/boavizta/helloasso-renew-contribution/internal/domain"
	"github.com/boavizta/helloasso-renew-contribution/services/baserow"
	"github.com/boavizta/helloasso-renew-contribution/services/brevo"
	"github.com/boavizta/helloasso-renew-contribution/services/helloasso"
)

type Config struct {
	HelloAsso helloasso.Config
	Baserow   baserow.Config
	Brevo     brevo.Config
	Email     domain.EmailSettings
	DryRun    bool
}

// Load accepts an environment lookup to keep configuration tests process-local.
// Optional settings retain the deployed service and sender defaults.
func Load(getenv func(string) string) (Config, error) {
	var cfg Config
	required := []string{"HELLOASSO_API_ID", "HELLOASSO_API_SECRET", "HELLOASSO_FROM_DATE", "HELLOASSO_ORG_SLUG", "BASEROW_API_TOKEN", "BASEROW_MEMBER_TABLE_ID", "BREVO_API_KEY"}
	for _, name := range required {
		if strings.TrimSpace(getenv(name)) == "" {
			return cfg, fmt.Errorf("%s environment variable must be set", name)
		}
	}
	if value := getenv("DRY_RUN"); value != "" {
		var err error
		cfg.DryRun, err = strconv.ParseBool(value)
		if err != nil {
			return cfg, fmt.Errorf("DRY_RUN must be a boolean")
		}
	}
	if _, err := time.Parse(time.DateOnly, getenv("HELLOASSO_FROM_DATE")); err != nil {
		return cfg, fmt.Errorf("HELLOASSO_FROM_DATE must use YYYY-MM-DD")
	}
	if id, err := strconv.Atoi(getenv("BASEROW_MEMBER_TABLE_ID")); err != nil || id <= 0 {
		return cfg, fmt.Errorf("BASEROW_MEMBER_TABLE_ID must be a positive integer")
	}
	optional := func(name, fallback string) string {
		if value := getenv(name); value != "" {
			return value
		}
		return fallback
	}
	cfg.HelloAsso = helloasso.Config{
		APIURL:   optional("HELLOASSO_API_URL", "https://api.helloasso.com"),
		ClientID: getenv("HELLOASSO_API_ID"), ClientSecret: getenv("HELLOASSO_API_SECRET"),
		OrgSlug: getenv("HELLOASSO_ORG_SLUG"), FromDate: getenv("HELLOASSO_FROM_DATE"),
	}
	cfg.Baserow = baserow.Config{
		APIURL:   optional("BASEROW_API_URL", "https://baserow.boavizta.org"),
		APIToken: getenv("BASEROW_API_TOKEN"), TableID: getenv("BASEROW_MEMBER_TABLE_ID"), DryRun: cfg.DryRun,
	}
	cfg.Brevo = brevo.Config{
		APIURL: optional("BREVO_API_URL", "https://api.sendinblue.com"),
		APIKey: getenv("BREVO_API_KEY"), DryRun: cfg.DryRun,
	}
	cfg.Email = domain.EmailSettings{
		SenderName:  optional("RENEWAL_SENDER_NAME", "Boavizta"),
		SenderEmail: optional("RENEWAL_SENDER_EMAIL", "no-reply@boavizta.org"),
		FrenchLink:  optional("RENEWAL_FRENCH_LINK", "https://www.helloasso.com/associations/boavizta/adhesions/cotisation-annuelle"),
		EnglishLink: optional("RENEWAL_ENGLISH_LINK", "https://www.helloasso.com/associations/boavizta/adhesions/annual-membership-fee"),
	}
	urls := map[string]*string{
		"HELLOASSO_API_URL": &cfg.HelloAsso.APIURL, "BASEROW_API_URL": &cfg.Baserow.APIURL,
		"BREVO_API_URL": &cfg.Brevo.APIURL, "RENEWAL_FRENCH_LINK": &cfg.Email.FrenchLink, "RENEWAL_ENGLISH_LINK": &cfg.Email.EnglishLink,
	}
	for name, value := range urls {
		u, err := url.Parse(*value)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
			return cfg, fmt.Errorf("%s must be an absolute HTTP(S) URL without credentials, query or fragment", name)
		}
		*value = strings.TrimRight(*value, "/")
	}
	if address, err := mail.ParseAddress(cfg.Email.SenderEmail); err != nil || address.Address != cfg.Email.SenderEmail {
		return cfg, fmt.Errorf("RENEWAL_SENDER_EMAIL must be an email address")
	}
	return cfg, nil
}
