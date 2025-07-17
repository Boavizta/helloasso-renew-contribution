package main

import (
	"log/slog"
	"os"
	"strings"
	"time"
	"unicode"

	"github.com/boavizta/helloasso-renew-contribution/services/baserow"
	"github.com/boavizta/helloasso-renew-contribution/services/brevo"
	"github.com/boavizta/helloasso-renew-contribution/services/helloasso"
	"github.com/samber/lo"
)

// toCamelCase converts a string to camel case format
func toCamelCase(s string) string {
	if s == "" {
		return s
	}

	words := strings.Fields(s)
	for i, word := range words {
		if len(word) > 0 {
			runes := []rune(word)
			runes[0] = unicode.ToUpper(runes[0])
			for j := 1; j < len(runes); j++ {
				runes[j] = unicode.ToLower(runes[j])
			}
			words[i] = string(runes)
		}
	}

	return strings.Join(words, " ")
}

const IndividualTypeId = 2521
const OrganizationTypeId = 2520

const EnglishId = 2590
const FrenchId = 2591
const SpanishId = 2592

// Members to check & update - merge lastPayments with members using email
type MemberPaymentPair struct {
	Member  baserow.Member
	Payment helloasso.Payment
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	logger.Info("Starting HelloAsso payment fetcher")

	payments, err := helloasso.GetPayments()
	if err != nil {
		logger.Error("Error fetching payments", "error", err)
		os.Exit(1)
	}

	logger.Info("Successfully fetched payments", "count", len(payments))

	// Extract and print distinct Form slugs using samber/lo
	// Uncomment if needed:
	// slugs := lo.Map(payments, func(payment helloasso.Payment, _ int) string {
	// 	return payment.OrderFormSlug
	// })

	// Filter payments to keep only those with form slugs "cotisation-annuelle" or "annual-membership-fee"
	filteredPayments := lo.Filter(payments, func(payment helloasso.Payment, _ int) bool {
		return payment.OrderFormSlug == "cotisation-annuelle" || payment.OrderFormSlug == "annual-membership-fee"
	})

	logger.Info("Filtered payments with form slugs 'cotisation-annuelle' or 'annual-membership-fee'", "count", len(filteredPayments))

	// Group payments by email and keep only the most recent one for each email
	uniquePayments := lo.Values(
		lo.MapValues(
			lo.GroupBy(filteredPayments, func(payment helloasso.Payment) string {
				return payment.PayerEmail
			}),
			func(payments []helloasso.Payment, _ string) helloasso.Payment {
				return lo.MaxBy(payments, func(p1, p2 helloasso.Payment) bool {
					return p1.OrderDate.After(p2.OrderDate)
				})
			},
		),
	)

	logger.Info("Unique emails with most recent payment data", "count", len(uniquePayments))

	// 1. compare & update with baserow - https://baserow.io/docs/apis%2Frest-api
	// Fetch members from Baserow
	logger.Info("Fetching members from Baserow")
	members, err := baserow.GetMembers()
	if err != nil {
		logger.Error("Error fetching members from Baserow", "error", err)
		os.Exit(1)
	}
	logger.Info("Successfully fetched members from Baserow", "count", len(members))

	// Prepare Data

	// Create a map of members by email for easier lookup using lo
	// Include primary email and alternative emails if they exist
	membersByEmail := lo.Reduce(members, func(acc map[string]baserow.Member, member baserow.Member, _ int) map[string]baserow.Member {
		// Add primary email
		acc[member.Email] = member

		// Add alternative emails if they exist
		if member.AlternativeEmail1 != "" {
			acc[member.AlternativeEmail1] = member
		}
		if member.AlternativeEmail2 != "" {
			acc[member.AlternativeEmail2] = member
		}

		return acc
	}, map[string]baserow.Member{})

	// Create a map of payments by email for easier lookup using lo.KeyBy
	paymentsByEmail := lo.KeyBy(uniquePayments, func(payment helloasso.Payment) string {
		return payment.PayerEmail
	})

	// Use lo.FilterMap to create membersWithPayment
	membersWithPayment := lo.FilterMap(uniquePayments, func(payment helloasso.Payment, _ int) (MemberPaymentPair, bool) {
		member, exists := membersByEmail[payment.PayerEmail]
		if !exists {
			return MemberPaymentPair{}, false
		}
		return MemberPaymentPair{
			Member:  member,
			Payment: payment,
		}, true
	})

	// Filter uniquePayments to keep only those with order dates older than 1 year
	oneYearAgo := time.Now().AddDate(-1, 0, 0)

	// Filter membersWithPayment to create the two slices based on payment dates
	// Use lo.Filter to get members with payments older than 1 year
	membersToUpdatePaymentNeeded := lo.Filter(membersWithPayment, func(pair MemberPaymentPair, _ int) bool {
		return pair.Payment.OrderDate.Before(oneYearAgo)
	})

	// Use lo.Filter to get members with recent payments that need status update
	membersToUpdateStatusUpdate := lo.Filter(membersWithPayment, func(pair MemberPaymentPair, _ int) bool {
		return !pair.Payment.OrderDate.Before(oneYearAgo) &&
			(pair.Member.ActiveMembership == false || pair.Member.LastPaymentDate.Format("2006-01-02") != pair.Payment.OrderDate.Format("2006-01-02"))
	})

	logger.Info("Members with payment needed", "count", len(membersToUpdatePaymentNeeded))

	// Use lo.ForEach to update members with payment needed
	lo.ForEach(membersToUpdatePaymentNeeded, func(pair MemberPaymentPair, _ int) {
		sendEmailAndUpdate(pair, logger)
	})

	logger.Info("Finished updating members with payment needed in Baserow")

	// update marked member valid payment date + upate payment date
	logger.Info("Members status to update", "count", len(membersToUpdateStatusUpdate))

	// Update all Members status with specific fields
	logger.Info("Updating all members status in Baserow")

	// Use lo.ForEach to update members status
	lo.ForEach(membersToUpdateStatusUpdate, func(pair MemberPaymentPair, _ int) {
		updateValidMembers(pair, err, logger)
	})

	logger.Info("Finished updating members status in Baserow")

	/// ### Stats
	generateStats(members, paymentsByEmail, logger, uniquePayments, membersByEmail)

}

func updateValidMembers(pair MemberPaymentPair, err error, logger *slog.Logger) {
	// Update the member with the required fields
	member := pair.Member
	payment := pair.Payment

	// Set the required fields
	member.ActiveMembership = true
	member.LastPaymentDate = payment.OrderDate
	member.NumberContributionsEmail = 0

	// Update the member in Baserow
	err = baserow.UpdateMember(member)
	if err != nil {
		logger.Error("Error updating member in Baserow", "error", err, "member", member.Email)
	}
}

func generateStats(members []baserow.Member, paymentsByEmail map[string]helloasso.Payment, logger *slog.Logger, uniquePayments []helloasso.Payment, membersByEmail map[string]baserow.Member) {
	// Generate membersWithoutPaymentEntry - members who don't have a payment entry
	membersWithoutPaymentEntry := lo.Filter(members, func(member baserow.Member, _ int) bool {
		_, exists := paymentsByEmail[member.Email]
		return !exists
	})

	logger.Info("Members without payment entry", "count", len(membersWithoutPaymentEntry))

	// Generate membersWithoutPaymentEntryIndividual - individual members without payment entry
	membersWithoutPaymentEntryIndividual := lo.Filter(membersWithoutPaymentEntry, func(member baserow.Member, _ int) bool {

		return member.MembershipType == IndividualTypeId
	})

	logger.Info("Individual members without payment entry", "count", len(membersWithoutPaymentEntryIndividual))

	// Generate membersWithoutPaymentEntryOrganization - organization members without payment entry
	membersWithoutPaymentEntryOrganization := lo.Filter(membersWithoutPaymentEntry, func(member baserow.Member, _ int) bool {
		return member.MembershipType == OrganizationTypeId
	})

	logger.Info("Organization members without payment entry", "count", len(membersWithoutPaymentEntryOrganization))

	// Generate paymentEntryWithoutMember - payment entries that don't have a corresponding member
	paymentEntryWithoutMember := lo.Filter(uniquePayments, func(payment helloasso.Payment, _ int) bool {
		_, exists := membersByEmail[payment.PayerEmail]
		return !exists
	})

	logger.Info("Payment entries without member", "count", len(paymentEntryWithoutMember))
}

func sendEmailAndUpdate(pair MemberPaymentPair, logger *slog.Logger) {
	// Update the member with the required fields
	member := pair.Member
	payment := pair.Payment

	// Set the required fields
	member.ActiveMembership = false
	member.LastPaymentDate = payment.OrderDate

	// Determine language preference
	isFrench := false
	for _, langId := range member.PreferredLanguages {
		if langId == FrenchId {
			isFrench = true
			break
		}
	}
	if !isFrench && member.Country == "France" {
		isFrench = true
	}

	// Set the appropriate contribution link based on language
	contributionLink := "https://www.helloasso.com/associations/boavizta/adhesions/annual-membership-fee"
	if isFrench {
		contributionLink = "https://www.helloasso.com/associations/boavizta/adhesions/cotisation-annuelle"
	}

	var subject, htmlContent, textContent string

	if isFrench {
		// French version
		subject = "Il est temps de renouveler votre adhésion à Boavizta"
		htmlContent = "<html><body>" +
			"<p>Cher(e) " + toCamelCase(member.FirstName) + ",</p>" +
			"<p>Nous espérons que vous allez bien.</p>" +
			"<p>Votre adhésion à l'association Boavizta arrive à échéance. Nous vous invitons à la renouveler pour continuer à soutenir nos actions en faveur de la mesure et la réduction de l'impact environnemental du numérique.</p>" +
			"<p>Pour renouveler votre adhésion, veuillez cliquer sur le lien suivant : <a href=\"" + contributionLink + "\">" + contributionLink + "</a></p>" +
			"<p>Nous vous remercions pour votre soutien continu.</p>" +
			"<p>Cordialement,<br>L'équipe Boavizta</p>" +
			"</body></html>"
		textContent = "Cher(e) " + toCamelCase(member.FirstName) + ",\n\n" +
			"Nous espérons que vous allez bien.\n\n" +
			"Votre adhésion à l'association Boavizta arrive à échéance. Nous vous invitons à la renouveler pour continuer à soutenir nos actions en faveur de la mesure et la réduction de l'impact environnemental du numérique.\n\n" +
			"Pour renouveler votre adhésion, veuillez cliquer sur le lien suivant : " + contributionLink + "\n\n" +
			"Nous vous remercions pour votre soutien continu.\n\n" +
			"Cordialement,\nL'équipe Boavizta"
	} else {
		// English version
		subject = "It's time to renew your Boavizta membership"
		htmlContent = "<html><body>" +
			"<p>Dear " + toCamelCase(member.FirstName) + ",</p>" +
			"<p>We hope this message finds you well.</p>" +
			"<p>Your membership with Boavizta association is coming to an end. We invite you to renew it to continue supporting our efforts in measuring and reducing the environmental impact of digital technology.</p>" +
			"<p>To renew your membership, please click on the following link: <a href=\"" + contributionLink + "\">" + contributionLink + "</a></p>" +
			"<p>Thank you for your continued support.</p>" +
			"<p>Best regards,<br>The Boavizta Team</p>" +
			"</body></html>"
		textContent = "Dear " + toCamelCase(member.FirstName) + ",\n\n" +
			"We hope this message finds you well.\n\n" +
			"Your membership with Boavizta association is coming to an end. We invite you to renew it to continue supporting our efforts in measuring and reducing the environmental impact of digital technology.\n\n" +
			"To renew your membership, please click on the following link: " + contributionLink + "\n\n" +
			"Thank you for your continued support.\n\n" +
			"Best regards,\nThe Boavizta Team"
	}

	// Send email notification via Brevo API
	emailData := brevo.EmailData{
		SenderName:  "Boavizta",
		SenderEmail: "no-reply@boavizta.org",
		ToEmail:     member.Email,
		ToName:      toCamelCase(member.FirstName) + " " + member.Surname,
		Subject:     subject,
		HtmlContent: htmlContent,
		TextContent: textContent,
	}

	var err error

	// Filter to send no email between 2 weeks
	if member.LastContributionEmailDate.Before(time.Now().AddDate(0, 0, -14)) {
		if member.Email == "youen@boavizta.org" || member.AlternativeEmail1 == "youen@boavizta.org" || member.AlternativeEmail2 == "tresorier@boavizta.org" {
			err = brevo.SendEmail(emailData)
			if err != nil {
				logger.Error("Error sending email notification", "error", err, "member", member.Email)
			} else {
				// mark sent
				member.LastContributionEmailDate = time.Now()
				member.NumberContributionsEmail++
			}
		} else {
			slog.Info("Skipping email notification", "member", member.Email, "subject", emailData.Subject, "body", emailData.HtmlContent, "bodytxt", emailData.TextContent)
		}

		// Update the member in Baserow
		err = baserow.UpdateMember(member)
		if err != nil {
			logger.Error("Error updating member in Baserow", "error", err, "member", member.Email)
		}
	}
}
