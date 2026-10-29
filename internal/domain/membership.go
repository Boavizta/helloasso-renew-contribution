package domain

import (
	"slices"
	"strings"
	"time"

	"github.com/samber/lo"
)

// Change describes a desired member update. Notify requests a renewal email;
// delivery bookkeeping is applied by the use case only after a successful send.
type Change struct {
	Member Member
	Notify bool
}

func emailDomain(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) != 2 || parts[0] == "" {
		return ""
	}
	return strings.ToLower(parts[1])
}

var publicDomains = lo.SliceToMap(strings.Fields(`
gmail.com hotmail.com hotmail.co.uk outlook.com outlook.fr live.com live.fr msn.com
yahoo.com yahoo.fr yahoo.co.uk aol.com icloud.com me.com mac.com protonmail.com
proton.me tutanota.com tuta.io gmx.com gmx.fr mail.com yandex.com zoho.com fastmail.com
hushmail.com startmail.com laposte.net orange.fr wanadoo.fr free.fr sfr.fr
numericable.fr bbox.fr neuf.fr gadz.org m4x.org mailinator.com guerrillamail.com
10minutemail.com tempmail.com temp-mail.org throwawaymail.com yopmail.com getnada.com
nada.email fakeinbox.com sharklasers.com trashmail.com trashmail.net mintemail.com
mohmal.com tempinbox.com maildrop.cc mailnesia.com spamgourmet.com dispostable.com
mailcatch.com tempmailo.com emailondeck.com mytemp.email burnermail.io isposable.com
moakt.com tmpmail.org tmpmail.net`), func(s string) (string, bool) { return s, true })

func corporateDomain(email string) string {
	domain := emailDomain(email)
	if publicDomains[domain] {
		return ""
	}
	return domain
}

func (m Member) emails() []string {
	return lo.Filter([]string{m.Email, m.AlternativeEmail1, m.AlternativeEmail2}, func(s string, _ int) bool { return s != "" })
}

func (p Payment) expired(now time.Time) bool {
	months := -12
	if p.Amount == 0 {
		months = -13
	}
	return p.OrderDate.Before(now.AddDate(0, months, 0))
}

// LatestPayments keeps the latest contribution per exact payer email, accepting
// only the two membership forms. Equal dates keep the first contribution.
func LatestPayments(payments []Payment) []Payment {
	filtered := lo.Filter(payments, func(p Payment, _ int) bool {
		return p.OrderFormSlug == "cotisation-annuelle" || p.OrderFormSlug == "annual-membership-fee"
	})
	byEmail := lo.GroupBy(filtered, func(p Payment) string { return p.PayerEmail })
	// Preserve input order rather than depending on map iteration order.
	return lo.Map(lo.UniqBy(filtered, func(p Payment) string { return p.PayerEmail }), func(p Payment, _ int) Payment {
		return latest(byEmail[p.PayerEmail])
	})
}

func latest(payments []Payment) Payment {
	return lo.MaxBy(payments, func(a, b Payment) bool { return a.OrderDate.After(b.OrderDate) })
}

// Plan calculates at most one update per member from a consistent snapshot.
// Corporate payments activate inactive colleagues without renewal emails. Exact
// email matching governs paid renewals (12 months) and free expiry (13 months).
func Plan(members []Member, payments []Payment, now time.Time) []Change {
	payments = LatestPayments(payments)
	changes := lo.FilterMap(members, func(member Member, _ int) (Change, bool) {
		original := member
		emails := member.emails()
		domainPayments := lo.Filter(payments, func(p Payment, _ int) bool {
			domain := corporateDomain(p.PayerEmail)
			return domain != "" && slices.ContainsFunc(emails, func(email string) bool { return emailDomain(email) == domain })
		})
		if !member.ActiveMembership && len(domainPayments) > 0 {
			payment := latest(domainPayments)
			if !payment.expired(now) {
				member.ActiveMembership = true
				member.LastPaymentDate = payment.OrderDate
				member.NumberContributionsEmail = 0
				return Change{Member: member}, true
			}
		}

		matched := lo.Filter(payments, func(p Payment, _ int) bool { return slices.Contains(emails, p.PayerEmail) })
		if len(matched) > 0 {
			payment := latest(matched)
			member.LastPaymentDate = payment.OrderDate
			member.ActiveMembership = !payment.expired(now)
			if member.ActiveMembership {
				member.NumberContributionsEmail = 0
				changed := !original.ActiveMembership || original.LastPaymentDate.Format(time.DateOnly) != payment.OrderDate.Format(time.DateOnly)
				return Change{Member: member}, changed
			}
			return Change{Member: member, Notify: payment.Amount != 0}, true
		}

		// The historical safety net uses a 13-month grace period for members
		// without an exact payment. Evaluate it after matching, not against
		// the pre-renewal snapshot.
		if !member.ActiveMembership {
			return Change{}, false
		}
		recentDomain := slices.ContainsFunc(domainPayments, func(p Payment) bool { return !p.OrderDate.Before(now.AddDate(0, -13, 0)) })
		if !recentDomain || member.LastPaymentDate.IsZero() || member.LastPaymentDate.Before(now.AddDate(0, -13, 0)) {
			member.ActiveMembership = false
			return Change{Member: member}, true
		}
		return Change{}, false
	})
	return guardReminders(members, changes, now)
}
