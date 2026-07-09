package main

import (
	"fmt"
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

// extractDomain extracts the domain part from an email address.
// Returns empty string if the email format is invalid.
func extractDomain(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return ""
	}
	return strings.ToLower(parts[1])
}

// commonEmailProviders lists free/public email domains for which domain-based
// matching must NOT be used (anyone can register, the domain says nothing about
// organisational affiliation). Payments from these domains fall through to the
// standard email-based matching.
var commonEmailProviders = map[string]bool{
	// International
	"gmail.com":          true,
	"hotmail.com":        true,
	"hotmail.co.uk":      true,
	"outlook.com":        true,
	"outlook.fr":         true,
	"live.com":           true,
	"live.fr":            true,
	"msn.com":            true,
	"yahoo.com":          true,
	"yahoo.fr":           true,
	"yahoo.co.uk":        true,
	"aol.com":            true,
	"icloud.com":         true,
	"me.com":             true,
	"mac.com":            true,
	"protonmail.com":     true,
	"proton.me":          true,
	"tutanota.com":       true,
	"tuta.io":            true,
	"gmx.com":            true,
	"gmx.fr":             true,
	"mail.com":           true,
	"yandex.com":         true,
	"zoho.com":           true,
	"fastmail.com":       true,
	"hushmail.com":       true,
	"startmail.com":      true,
	// French ISPs
	"laposte.net":        true,
	"orange.fr":          true,
	"wanadoo.fr":         true,
	"free.fr":            true,
	"sfr.fr":             true,
	"numericable.fr":     true,
	"bbox.fr":            true,
	"neuf.fr":            true,
	// Alumni / school
	"gadz.org":           true,
	"m4x.org":            true,
	// Disposable / temporary email providers
	"mailinator.com":     true,
	"guerrillamail.com":  true,
	"10minutemail.com":   true,
	"tempmail.com":       true,
	"temp-mail.org":      true,
	"throwawaymail.com":  true,
	"yopmail.com":        true,
	"getnada.com":        true,
	"nada.email":         true,
	"fakeinbox.com":      true,
	"sharklasers.com":    true,
	"trashmail.com":      true,
	"trashmail.net":      true,
	"mintemail.com":      true,
	"mohmal.com":         true,
	"tempinbox.com":      true,
	"maildrop.cc":        true,
	"mailnesia.com":      true,
	"spamgourmet.com":    true,
	"dispostable.com":    true,
	"mailcatch.com":      true,
	"tempmailo.com":      true,
	"emailondeck.com":    true,
	"mytemp.email":       true,
	"burnermail.io":      true,
	"isposable.com":      true,
	"moakt.com":          true,
	"tmpmail.org":        true,
	"tmpmail.net":        true,
}

// isCommonEmailProvider reports whether the domain is a free/public email provider
// for which domain-based matching is not meaningful.
func isCommonEmailProvider(domain string) bool {
	return commonEmailProviders[domain]
}

const IndividualTypeId = 2521
const OrganizationTypeId = 2520

const EnglishId = 2590
const FrenchId = 2591
const SpanishId = 2592

// MemberPaymentPair merges a member with their payment for processing
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

	// Fetch members from Baserow
	logger.Info("Fetching members from Baserow")
	members, err := baserow.GetMembers()
	if err != nil {
		logger.Error("Error fetching members from Baserow", "error", err)
		os.Exit(1)
	}
	logger.Info("Successfully fetched members from Baserow", "count", len(members))

	// Create a map of members by email for easier lookup
	// Include primary email and alternative emails if they exist
	membersByEmail := lo.Reduce(members, func(acc map[string]baserow.Member, member baserow.Member, _ int) map[string]baserow.Member {
		acc[member.Email] = member
		if member.AlternativeEmail1 != "" {
			acc[member.AlternativeEmail1] = member
		}
		if member.AlternativeEmail2 != "" {
			acc[member.AlternativeEmail2] = member
		}
		return acc
	}, map[string]baserow.Member{})

	// --- Domain-based matching: update INACTIVE members (no email) ---
	// Group all payments by domain, keep most recent per domain
	paymentsByDomain := lo.Values(
		lo.MapValues(
			lo.GroupBy(uniquePayments, func(payment helloasso.Payment) string {
				return extractDomain(payment.PayerEmail)
			}),
			func(payments []helloasso.Payment, _ string) helloasso.Payment {
				return lo.MaxBy(payments, func(p1, p2 helloasso.Payment) bool {
					return p1.OrderDate.After(p2.OrderDate)
				})
			},
		),
	)

	// Track which members were updated via domain matching to skip them in email phase
	domainUpdatedIds := map[int]bool{}

	lo.ForEach(paymentsByDomain, func(payment helloasso.Payment, _ int) {
		domain := extractDomain(payment.PayerEmail)
		if domain == "" {
			return
		}

		// Skip common/free email providers — domain matching is not meaningful
		// for them, these payments fall through to the email-based matching.
		if isCommonEmailProvider(domain) {
			logger.Info("Skipping domain matching for common email provider",
				"domain", domain,
				"payer", payment.PayerEmail,
			)
			return
		}

		domainMembers := lo.Filter(members, func(member baserow.Member, _ int) bool {
			return extractDomain(member.Email) == domain ||
				(member.AlternativeEmail1 != "" && extractDomain(member.AlternativeEmail1) == domain) ||
				(member.AlternativeEmail2 != "" && extractDomain(member.AlternativeEmail2) == domain)
		})

		// Only update members that are NOT already active
		inactiveMembers := lo.Filter(domainMembers, func(member baserow.Member, _ int) bool {
			return !member.ActiveMembership
		})

		if len(inactiveMembers) == 0 {
			return
		}

		logger.Info("Domain payment - updating inactive members",
			"domain", domain,
			"payer", payment.PayerEmail,
			"amount", payment.Amount,
			"inactive", len(inactiveMembers),
		)

		lo.ForEach(inactiveMembers, func(member baserow.Member, _ int) {
			member.ActiveMembership = true
			member.LastPaymentDate = payment.OrderDate
			member.NumberContributionsEmail = 0

			if updateErr := baserow.UpdateMember(member); updateErr != nil {
				logger.Error("Error updating member in Baserow",
					"error", updateErr,
					"member", member.Email,
					"domain", domain,
				)
			} else {
				domainUpdatedIds[member.Id] = true
			}
		})
	})

	logger.Info("Finished domain-based updates", "count", len(domainUpdatedIds))

	// --- Email-based matching: handle active members (old mechanism) ---
	// Create a map of payments by email for easier lookup
	paymentsByEmail := lo.KeyBy(uniquePayments, func(payment helloasso.Payment) string {
		return payment.PayerEmail
	})

	// Match payments to members by email, excluding domain-updated members
	membersWithPayment := lo.FilterMap(uniquePayments, func(payment helloasso.Payment, _ int) (MemberPaymentPair, bool) {
		member, exists := membersByEmail[payment.PayerEmail]
		if !exists {
			return MemberPaymentPair{}, false
		}
		// Skip members already updated via domain matching
		if domainUpdatedIds[member.Id] {
			return MemberPaymentPair{}, false
		}
		return MemberPaymentPair{
			Member:  member,
			Payment: payment,
		}, true
	})

	twelveMonthsAgo := time.Now().AddDate(0, -12, 0)
	thirteenMonthsAgo := time.Now().AddDate(0, -13, 0)

	// Members with payments older than 12 months → send renewal email
	membersToUpdatePaymentNeeded := lo.Filter(membersWithPayment, func(pair MemberPaymentPair, _ int) bool {
		return pair.Payment.OrderDate.Before(twelveMonthsAgo)
	})

	// Members with recent payments (≤12 months) that need status update
	membersToUpdateStatusUpdate := lo.Filter(membersWithPayment, func(pair MemberPaymentPair, _ int) bool {
		return !pair.Payment.OrderDate.Before(twelveMonthsAgo) &&
			(pair.Member.ActiveMembership == false || pair.Member.LastPaymentDate.Format("2006-01-02") != pair.Payment.OrderDate.Format("2006-01-02"))
	})

	logger.Info("Members with payment needed", "count", len(membersToUpdatePaymentNeeded))

	lo.ForEach(membersToUpdatePaymentNeeded, func(pair MemberPaymentPair, _ int) {
		sendEmailAndUpdate(pair, logger)
	})

	logger.Info("Finished updating members with payment needed in Baserow")

	logger.Info("Members status to update", "count", len(membersToUpdateStatusUpdate))
	logger.Info("Updating all members status in Baserow")

	lo.ForEach(membersToUpdateStatusUpdate, func(pair MemberPaymentPair, _ int) {
		updateValidMembers(pair, err, logger)
	})

	logger.Info("Finished updating members status in Baserow")

	// --- Deactivate members with no recent payment (within 13 months) ---

	// Collect all member IDs that were already processed in earlier steps
	processedMemberIds := make(map[int]bool)
	for id := range domainUpdatedIds {
		processedMemberIds[id] = true
	}
	for _, pair := range membersWithPayment {
		processedMemberIds[pair.Member.Id] = true
	}

	// Build sets of recent payment indicators (within last 13 months)
	recentPaymentEmails := make(map[string]bool)
	recentPaymentDomains := make(map[string]bool)
	for _, payment := range uniquePayments {
		if payment.OrderDate.Before(thirteenMonthsAgo) {
			continue
		}
		recentPaymentEmails[payment.PayerEmail] = true

		domain := extractDomain(payment.PayerEmail)
		if domain != "" && !isCommonEmailProvider(domain) {
			recentPaymentDomains[domain] = true
		}
	}

	membersToDeactivate := lo.Filter(members, func(member baserow.Member, _ int) bool {
		if processedMemberIds[member.Id] {
			return false
		}
		if !member.ActiveMembership {
			return false
		}

		// Check if member has a recent payment by email
		if recentPaymentEmails[member.Email] ||
			(member.AlternativeEmail1 != "" && recentPaymentEmails[member.AlternativeEmail1]) ||
			(member.AlternativeEmail2 != "" && recentPaymentEmails[member.AlternativeEmail2]) {
			return false
		}

		// Check if any of member's email domains have a recent payment
		for _, email := range []string{member.Email, member.AlternativeEmail1, member.AlternativeEmail2} {
			if email == "" {
				continue
			}
			domain := extractDomain(email)
			if domain != "" && recentPaymentDomains[domain] {
				return false
			}
		}

		return true
	})

	logger.Info("Members to deactivate (no recent payment in 13 months)", "count", len(membersToDeactivate))

	lo.ForEach(membersToDeactivate, func(member baserow.Member, _ int) {
		member.ActiveMembership = false

		if updateErr := baserow.UpdateMember(member); updateErr != nil {
			logger.Error("Error deactivating member in Baserow",
				"error", updateErr,
				"member", member.Email,
			)
		} else {
			logger.Info("Deactivated member (no recent payment)",
				"member", member.Email,
				"id", member.Id,
			)
		}
	})

	logger.Info("Finished deactivating members with no recent payment")

	/// ### Stats
	generateStats(members, paymentsByEmail, logger, uniquePayments, membersByEmail)
}

func updateValidMembers(pair MemberPaymentPair, err error, logger *slog.Logger) {
	member := pair.Member
	payment := pair.Payment

	member.ActiveMembership = true
	member.LastPaymentDate = payment.OrderDate
	member.NumberContributionsEmail = 0

	err = baserow.UpdateMember(member)
	if err != nil {
		logger.Error("Error updating member in Baserow", "error", err, "member", member.Email)
	}
}

func generateStats(members []baserow.Member, paymentsByEmail map[string]helloasso.Payment, logger *slog.Logger, uniquePayments []helloasso.Payment, membersByEmail map[string]baserow.Member) {
	// Members without payment entry
	membersWithoutPaymentEntry := lo.Filter(members, func(member baserow.Member, _ int) bool {
		_, exists := paymentsByEmail[member.Email]
		return !exists
	})

	logger.Info("Members without payment entry", "count", len(membersWithoutPaymentEntry))

	logger.Info("Listing all members without payment entry:")
	for _, member := range membersWithoutPaymentEntry {
		fmt.Printf("%s,%s\n", member.Email, member.FirstName+" "+member.Surname)
	}

	// Individual members without payment entry
	membersWithoutPaymentEntryIndividual := lo.Filter(membersWithoutPaymentEntry, func(member baserow.Member, _ int) bool {
		return member.MembershipType == IndividualTypeId
	})

	logger.Info("Individual members without payment entry", "count", len(membersWithoutPaymentEntryIndividual))

	// Organization members without payment entry
	membersWithoutPaymentEntryOrganization := lo.Filter(membersWithoutPaymentEntry, func(member baserow.Member, _ int) bool {
		return member.MembershipType == OrganizationTypeId
	})

	logger.Info("Organization members without payment entry", "count", len(membersWithoutPaymentEntryOrganization))

	// Payment entries without member
	paymentEntryWithoutMember := lo.Filter(uniquePayments, func(payment helloasso.Payment, _ int) bool {
		_, exists := membersByEmail[payment.PayerEmail]
		return !exists
	})

	logger.Info("Payment entries without member", "count", len(paymentEntryWithoutMember))

	logger.Info("Listing all payment entries without member:")
	for _, payment := range paymentEntryWithoutMember {
		fmt.Printf("%s,%s\n", payment.PayerEmail, payment.PayerFirstName+" "+payment.PayerLastName)
	}

	// SME payments (100€)
	smePayments := lo.Filter(uniquePayments, func(payment helloasso.Payment, _ int) bool {
		return payment.Amount == 100
	})

	logger.Info("SME payments (100€)", "count", len(smePayments))
	logger.Info("Listing all SME payments:")
	for _, payment := range smePayments {
		fmt.Printf("%s,%s,%s\n", payment.PayerEmail, payment.PayerFirstName+" "+payment.PayerLastName, payment.OrderDate.Format("2006-01-02"))
	}

	// Enterprise payments (1000€)
	enterprisePayments := lo.Filter(uniquePayments, func(payment helloasso.Payment, _ int) bool {
		return payment.Amount == 1000
	})

	logger.Info("Enterprise payments (1000€)", "count", len(enterprisePayments))
	logger.Info("Listing all Enterprise payments:")
	for _, payment := range enterprisePayments {
		fmt.Printf("%s,%s,%s\n", payment.PayerEmail, payment.PayerFirstName+" "+payment.PayerLastName, payment.OrderDate.Format("2006-01-02"))
	}
}

func sendEmailAndUpdate(pair MemberPaymentPair, logger *slog.Logger) {
	member := pair.Member
	payment := pair.Payment

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

	contributionLink := "https://www.helloasso.com/associations/boavizta/adhesions/annual-membership-fee"
	if isFrench {
		contributionLink = "https://www.helloasso.com/associations/boavizta/adhesions/cotisation-annuelle"
	}

	var subject, htmlContent, textContent string

	if isFrench {
		subject = "Prêt pour une nouvelle année avec Boavizta ? Il est temps de renouveler votre adhésion"
		htmlContent = "<html><body>" +
			"<p>Cher(e) " + toCamelCase(member.FirstName) + ",</p>" +
			"<p>Alors que votre adhésion à Boavizta touche à sa fin, nous tenons à vous remercier d'avoir été avec nous cette année !</p>" +
			"<p>Boavizta existe grâce aux incroyables contributions de ses membres, des personnes comme vous qui nous aident à créer et partager des communs pour promouvoir des pratiques numériques respectueuses des limites planétaires. Votre implication fait vraiment la différence.</p>" +
			"<p>Nous sommes enthousiastes à l'idée de ce qui nous attend en 2026 et nous serons heureux de vous voir rester impliqué(e) dans notre communauté.</p>" +
			"<p>👉 Pour renouveler votre adhésion, <a href=\"" + contributionLink + "\">cliquez simplement ici</a>.</p>" +
			"<p>Merci encore de faire partie de Boavizta !</p>" +
			"<p>Cordialement,<br>L'équipe Boavizta</p>" +
			"</body></html>"
		textContent = "Cher(e) " + toCamelCase(member.FirstName) + ",\n\n" +
			"Alors que votre adhésion à Boavizta touche à sa fin, nous tenons à vous remercier d'avoir été avec nous cette année !\n\n" +
			"Boavizta existe grâce aux incroyables contributions de ses membres, des personnes comme vous qui nous aident à créer et partager des communs pour promouvoir des pratiques numériques respectueuses des limites planétaires. Votre implication fait vraiment la différence.\n\n" +
			"Nous sommes enthousiastes à l'idée de ce qui nous attend en 2026 et nous serons heureux de vous voir rester impliqué(e) dans notre communauté.\n\n" +
			"👉 Pour renouveler votre adhésion, cliquez simplement ici : " + contributionLink + "\n\n" +
			"Merci encore de faire partie de Boavizta !\n\n" +
			"Cordialement,\nL'équipe Boavizta"
	} else {
		subject = "Ready for another year with Boavizta? It's time to renew your membership"
		htmlContent = "<html><body>" +
			"<p>Dear " + toCamelCase(member.FirstName) + ",</p>" +
			"<p>As your membership with Boavizta comes to an end, we want to say thank you for being with us this past year!</p>" +
			"<p>Boavizta exists thanks to the incredible contributions of its members, people like you who help us create and share commons to promote digital practices that respect planetary boundaries. Your involvement really makes a difference.</p>" +
			"<p>We're excited about what's coming in 2026 and we will be happy to see you stay involved in our community.</p>" +
			"<p>👉 To renew your membership, simply <a href=\"" + contributionLink + "\">click here</a>.</p>" +
			"<p>Thanks again for being part of Boavizta!</p>" +
			"<p>Warm regards,<br>Boavizta Team</p>" +
			"</body></html>"
		textContent = "Dear " + toCamelCase(member.FirstName) + ",\n\n" +
			"As your membership with Boavizta comes to an end, we want to say thank you for being with us this past year!\n\n" +
			"Boavizta exists thanks to the incredible contributions of its members, people like you who help us create and share commons to promote digital practices that respect planetary boundaries. Your involvement really makes a difference.\n\n" +
			"We're excited about what's coming in 2026 and we will be happy to see you stay involved in our community.\n\n" +
			"👉 To renew your membership, simply click here: " + contributionLink + "\n\n" +
			"Thanks again for being part of Boavizta!\n\n" +
			"Warm regards,\nBoavizta Team"
	}

	emailData := brevo.EmailData{
		SenderName:  "Boavizta",
		SenderEmail: "no-reply@boavizta.org",
		ToEmail:     member.Email,
		ToName:      toCamelCase(member.FirstName) + " " + member.Surname,
		Subject:     subject,
		HtmlContent: htmlContent,
		TextContent: textContent,
	}

	// Send renewal email only if last one was more than 14 days ago
	if member.LastContributionEmailDate.Before(time.Now().AddDate(0, 0, -14)) {
		if err := brevo.SendEmail(emailData); err != nil {
			logger.Error("Error sending email notification", "error", err, "member", member.Email)
		} else {
			member.LastContributionEmailDate = time.Now()
			member.NumberContributionsEmail++
		}
	}

	// Always update Baserow (deactivation + payment date), even if email was rate-limited
	if err := baserow.UpdateMember(member); err != nil {
		logger.Error("Error updating member in Baserow", "error", err, "member", member.Email)
	}
}
