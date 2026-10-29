package application

import (
	"github.com/boavizta/helloasso-renew-contribution/internal/domain"
	"github.com/samber/lo"
)

// report retains the reconciliation diagnostics as structured logs rather than
// mixing CSV output with the job's log stream.
func (r *Renewal) report(members []domain.Member, payments []domain.Payment) {
	payments = domain.LatestPayments(payments)
	byEmail := lo.KeyBy(payments, func(p domain.Payment) string { return p.PayerEmail })
	withoutPayment := lo.Filter(members, func(m domain.Member, _ int) bool { _, ok := byEmail[m.Email]; return !ok })
	r.logger.Info("Members without payment entry", "count", len(withoutPayment))
	lo.ForEach(withoutPayment, func(m domain.Member, _ int) {
		r.logger.Info("Member without payment entry", "email", m.Email, "name", m.FirstName+" "+m.Surname)
	})
	for _, membershipType := range []string{domain.IndividualMembership, domain.OrganizationMembership} {
		count := lo.CountBy(withoutPayment, func(m domain.Member) bool { return m.MembershipType == membershipType })
		r.logger.Info("Members without payment entry by type", "type", membershipType, "count", count)
	}
	memberEmails := lo.Reduce(members, func(acc map[string]bool, m domain.Member, _ int) map[string]bool {
		acc[m.Email] = true
		if m.AlternativeEmail1 != "" {
			acc[m.AlternativeEmail1] = true
		}
		if m.AlternativeEmail2 != "" {
			acc[m.AlternativeEmail2] = true
		}
		return acc
	}, map[string]bool{})
	withoutMember := lo.Filter(payments, func(p domain.Payment, _ int) bool { return !memberEmails[p.PayerEmail] })
	r.logger.Info("Payment entries without member", "count", len(withoutMember))
	lo.ForEach(withoutMember, func(p domain.Payment, _ int) {
		r.logger.Info("Payment entry without member", "email", p.PayerEmail, "name", p.PayerFirstName+" "+p.PayerLastName)
	})
	for _, amount := range []float64{100, 1000} {
		tiers := lo.Filter(payments, func(p domain.Payment, _ int) bool { return p.Amount == amount })
		r.logger.Info("Payments by tier", "amount", amount, "count", len(tiers))
		lo.ForEach(tiers, func(p domain.Payment, _ int) {
			r.logger.Info("Tier payment", "email", p.PayerEmail, "name", p.PayerFirstName+" "+p.PayerLastName, "date", p.OrderDate, "amount", amount)
		})
	}
}
