package main

import (
	"log/slog"
	"os"
	"time"

	"github.com/boavizta/helloasso-renew-contribution/services/helloasso"
	"github.com/samber/lo"
)

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

	// Filter uniquePayments to keep only those with order dates older than 1 year
	oneYearAgo := time.Now().AddDate(-1, 0, 0)
	oldPayments := lo.Filter(uniquePayments, func(payment helloasso.Payment, _ int) bool {
		return payment.OrderDate.Before(oneYearAgo)
	})

	logger.Info("Payments with order dates older than 1 year", "count", len(oldPayments))

	// Filter uniquePayments to keep only those with order dates equal to or after 1 year ago
	lastPayments := lo.Filter(uniquePayments, func(payment helloasso.Payment, _ int) bool {
		return payment.OrderDate.After(oneYearAgo) || payment.OrderDate.Equal(oneYearAgo)
	})

	logger.Info("Payments with order dates equal to or after 1 year ago", "count", len(lastPayments))

	//TODO 1. compare & update with baserow
	//TODO 2. update marked member valid payment date + upate payment date
	//TODO 3. mark member with need renew and last payment date and identify for emailing
	//TODO 4. Email with brevo
	//TODO 5. mark member email sent

}
