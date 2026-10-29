package application_test

import (
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/boavizta/helloasso-renew-contribution/internal/application"
	"github.com/boavizta/helloasso-renew-contribution/internal/domain"
)

type contributions struct {
	paid, free       []domain.Payment
	paidErr, freeErr error
}

func (c *contributions) GetPayments() ([]domain.Payment, error)            { return c.paid, c.paidErr }
func (c *contributions) GetFreeMembershipItems() ([]domain.Payment, error) { return c.free, c.freeErr }

type members struct {
	items, updates    []domain.Member
	readErr, writeErr error
}

func (m *members) GetMembers() ([]domain.Member, error) { return m.items, m.readErr }
func (m *members) UpdateMember(member domain.Member) error {
	m.updates = append(m.updates, member)
	return m.writeErr
}

type mailer struct {
	sent []domain.Email
	err  error
}

func (m *mailer) SendEmail(email domain.Email) error {
	m.sent = append(m.sent, email)
	return m.err
}

func renewal(c *contributions, m *members, mail *mailer, now time.Time) *application.Renewal {
	return application.NewRenewal(c, m, mail, func() time.Time { return now }, slog.New(slog.NewTextHandler(io.Discard, nil)), domain.EmailSettings{})
}

func TestRunDeliveryBookkeeping(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)
	failure := errors.New("delivery failed")
	for _, tt := range []struct {
		name      string
		err       error
		wantCount int
	}{
		{"success", nil, 3},
		{"failure still deactivates", failure, 2},
	} {
		t.Run(tt.name, func(t *testing.T) {
			last := now.AddDate(0, -2, 0)
			m := &members{items: []domain.Member{{Id: 1, Email: "a@gmail.com", ActiveMembership: true, NumberContributionsEmail: 2, LastContributionEmailDate: last}}}
			p := domain.Payment{PayerEmail: "a@gmail.com", OrderDate: now.AddDate(0, -14, 0), Amount: 100, OrderFormSlug: "cotisation-annuelle"}
			c := &contributions{paid: []domain.Payment{p}}
			mail := &mailer{err: tt.err}
			err := renewal(c, m, mail, now).Run()
			if !errors.Is(err, tt.err) || len(m.updates) != 1 || len(mail.sent) != 1 {
				t.Fatalf("err=%v updates=%+v emails=%+v", err, m.updates, mail.sent)
			}
			got := m.updates[0]
			wantDate := now
			if tt.err != nil {
				wantDate = last
			}
			if got.ActiveMembership || got.NumberContributionsEmail != tt.wantCount || got.LastContributionEmailDate != wantDate || got.LastPaymentDate != p.OrderDate {
				t.Fatalf("update = %+v", got)
			}
		})
	}
}

func TestRunReadsBeforeWrites(t *testing.T) {
	t.Parallel()
	failure := errors.New("read failed")
	for _, stage := range []string{"payments", "free", "members"} {
		t.Run(stage, func(t *testing.T) {
			c, m, mail := &contributions{}, &members{items: []domain.Member{{Id: 1, ActiveMembership: true}}}, &mailer{}
			switch stage {
			case "payments":
				c.paidErr = failure
			case "free":
				c.freeErr = failure
			case "members":
				m.readErr = failure
			}
			if err := renewal(c, m, mail, time.Now()).Run(); !errors.Is(err, failure) {
				t.Fatalf("err = %v", err)
			}
			if len(m.updates) != 0 || len(mail.sent) != 0 {
				t.Fatal("side effects after read failure")
			}
		})
	}
}

func TestRunContinuesAfterWriteErrors(t *testing.T) {
	t.Parallel()
	failure := errors.New("write failed")
	m := &members{items: []domain.Member{{Id: 1, ActiveMembership: true}, {Id: 2, ActiveMembership: true}}, writeErr: failure}
	if err := renewal(&contributions{}, m, &mailer{}, time.Now()).Run(); !errors.Is(err, failure) || len(m.updates) != 2 {
		t.Fatalf("err=%v updates=%+v", err, m.updates)
	}
}

func TestRunMergesFreeMemberships(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)
	m := &members{items: []domain.Member{{Id: 1, Email: "a@gmail.com"}}}
	c := &contributions{free: []domain.Payment{{PayerEmail: "a@gmail.com", OrderDate: now, OrderFormSlug: "annual-membership-fee"}}}
	mail := &mailer{}
	if err := renewal(c, m, mail, now).Run(); err != nil {
		t.Fatal(err)
	}
	if len(m.updates) != 1 || !m.updates[0].ActiveMembership || len(mail.sent) != 0 {
		t.Fatalf("updates=%+v emails=%+v", m.updates, mail.sent)
	}
}

func TestRunDuplicateRecipientDelivery(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	failure := errors.New("operation failed")
	for _, tt := range []struct {
		name              string
		sendErr, writeErr error
	}{
		{"successful delivery", nil, nil},
		{"failed delivery is not retried for duplicate", failure, nil},
		{"failed persistence does not cause duplicate delivery", nil, failure},
	} {
		t.Run(tt.name, func(t *testing.T) {
			last := now.AddDate(0, 0, -30)
			m := &members{items: []domain.Member{
				{Id: 1, Email: "person@gmail.com", NumberContributionsEmail: 2, LastContributionEmailDate: last},
				{Id: 2, Email: "PERSON@GMAIL.COM", AlternativeEmail1: "person@gmail.com", NumberContributionsEmail: 5, LastContributionEmailDate: last},
			}, writeErr: tt.writeErr}
			c := &contributions{paid: []domain.Payment{{PayerEmail: "person@gmail.com", OrderDate: now.AddDate(0, -14, 0), Amount: 100, OrderFormSlug: "cotisation-annuelle"}}}
			mail := &mailer{err: tt.sendErr}
			err := renewal(c, m, mail, now).Run()
			if (err != nil) != (tt.sendErr != nil || tt.writeErr != nil) {
				t.Fatalf("err=%v", err)
			}
			if len(mail.sent) != 1 || len(m.updates) != 2 {
				t.Fatalf("emails=%d updates=%+v", len(mail.sent), m.updates)
			}
			for i, update := range m.updates {
				wantDate, wantCount := last, m.items[i].NumberContributionsEmail
				if tt.sendErr == nil {
					wantDate, wantCount = now, wantCount+1
				}
				if update.LastContributionEmailDate != wantDate || update.NumberContributionsEmail != wantCount || update.ActiveMembership {
					t.Fatalf("update=%+v", update)
				}
			}
			if tt.sendErr == nil && tt.writeErr == nil {
				m.items, m.updates = m.updates, nil
				if err := renewal(c, m, mail, now).Run(); err != nil {
					t.Fatal(err)
				}
				if len(mail.sent) != 1 {
					t.Fatal("second run sent another reminder")
				}
			}
		})
	}
}
