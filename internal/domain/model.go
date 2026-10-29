// Package domain contains membership rules independent of infrastructure.
package domain

import "time"

type Member struct {
	Id                        int
	Surname                   string
	FirstName                 string
	Email                     string
	AlternativeEmail1         string
	AlternativeEmail2         string
	ActiveMembership          bool
	LastPaymentDate           time.Time
	LastContributionEmailDate time.Time
	NumberContributionsEmail  int
	MembershipType            string
	PreferredLanguages        []string
	Country                   string
}

const (
	IndividualMembership   = "individual"
	OrganizationMembership = "organization"
	FrenchLanguage         = "fr"
	EnglishLanguage        = "en"
	SpanishLanguage        = "es"
)

type Payment struct {
	OrderFormSlug  string
	OrderDate      time.Time
	PayerEmail     string
	PayerFirstName string
	PayerLastName  string
	Amount         float64
}

type Email struct {
	SenderName  string
	SenderEmail string
	ToEmail     string
	ToName      string
	Subject     string
	HtmlContent string
	TextContent string
}
