// Package application orchestrates membership renewal through consumer-owned ports.
package application

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/boavizta/helloasso-renew-contribution/internal/domain"
)

type Contributions interface {
	GetPayments() ([]domain.Payment, error)
	GetFreeMembershipItems() ([]domain.Payment, error)
}

type Members interface {
	GetMembers() ([]domain.Member, error)
	UpdateMember(domain.Member) error
}

type Mailer interface {
	SendEmail(domain.Email) error
}

type Renewal struct {
	contributions Contributions
	members       Members
	mailer        Mailer
	now           func() time.Time
	logger        *slog.Logger
	email         domain.EmailSettings
}

func NewRenewal(contributions Contributions, members Members, mailer Mailer, now func() time.Time, logger *slog.Logger, email domain.EmailSettings) *Renewal {
	return &Renewal{contributions: contributions, members: members, mailer: mailer, now: now, logger: logger, email: email}
}

// Run reads all inputs before performing writes. Individual delivery/update
// failures do not block other members, but are returned so the job reports failure.
func (r *Renewal) Run() error {
	payments, err := r.contributions.GetPayments()
	if err != nil {
		return fmt.Errorf("fetch payments: %w", err)
	}
	free, err := r.contributions.GetFreeMembershipItems()
	if err != nil {
		return fmt.Errorf("fetch free memberships: %w", err)
	}
	members, err := r.members.GetMembers()
	if err != nil {
		return fmt.Errorf("fetch members: %w", err)
	}
	now := r.now()
	changes := domain.Plan(members, append(payments, free...), now)
	r.logger.Info("Membership renewal planned", "members", len(members), "contributions", len(payments)+len(free), "updates", len(changes))
	var failures []error
	delivered := map[string]bool{}
	// Send once per recipient before persisting updates so every planned row
	// sharing that mailbox receives the same successful-delivery timestamp.
	for _, change := range changes {
		member := change.Member
		if change.Notify {
			if err := r.mailer.SendEmail(domain.RenewalEmail(member, r.email)); err != nil {
				failures = append(failures, fmt.Errorf("email member %d: %w", member.Id, err))
			} else {
				delivered[domain.RecipientKey(member.Email)] = true
			}
		}
	}
	for _, change := range changes {
		member := change.Member
		if delivered[domain.RecipientKey(member.Email)] {
			member.LastContributionEmailDate = now
			member.NumberContributionsEmail++
		}
		if err := r.members.UpdateMember(member); err != nil {
			failures = append(failures, fmt.Errorf("update member %d: %w", member.Id, err))
		}
	}
	r.report(members, append(payments, free...))
	return errors.Join(failures...)
}
