package domain_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/boavizta/helloasso-renew-contribution/internal/domain"
)

var now = time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)

func payment(email string, months int, amount float64) domain.Payment {
	return domain.Payment{PayerEmail: email, OrderDate: now.AddDate(0, months, 0), Amount: amount, OrderFormSlug: "annual-membership-fee"}
}

func TestPlan(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		member     domain.Member
		payments   []domain.Payment
		wantChange bool
		wantActive bool
		wantNotify bool
	}{
		{"paid renewal", domain.Member{Email: "a@gmail.com"}, []domain.Payment{payment("a@gmail.com", -1, 100)}, true, true, false},
		{"paid exact boundary", domain.Member{Email: "a@gmail.com"}, []domain.Payment{payment("a@gmail.com", -12, 100)}, true, true, false},
		{"paid expired", domain.Member{Email: "a@gmail.com", ActiveMembership: true}, []domain.Payment{payment("a@gmail.com", -13, 100)}, true, false, true},
		{"free grace", domain.Member{Email: "a@gmail.com"}, []domain.Payment{payment("a@gmail.com", -12, 0)}, true, true, false},
		{"free exact boundary", domain.Member{Email: "a@gmail.com"}, []domain.Payment{payment("a@gmail.com", -13, 0)}, true, true, false},
		{"free expired without email", domain.Member{Email: "a@gmail.com", ActiveMembership: true}, []domain.Payment{payment("a@gmail.com", -14, 0)}, true, false, false},
		{"first alternative", domain.Member{Email: "new@gmail.com", AlternativeEmail1: "old@gmail.com"}, []domain.Payment{payment("old@gmail.com", -1, 100)}, true, true, false},
		{"second alternative", domain.Member{Email: "new@gmail.com", AlternativeEmail2: "old@gmail.com"}, []domain.Payment{payment("old@gmail.com", -1, 100)}, true, true, false},
		{"corporate colleague", domain.Member{Email: "colleague@company.test"}, []domain.Payment{payment("payer@company.test", -1, 1000)}, true, true, false},
		{"corporate domain case", domain.Member{Email: "colleague@COMPANY.TEST"}, []domain.Payment{payment("payer@company.test", -1, 1000)}, true, true, false},
		{"corporate alternative", domain.Member{Email: "a@gmail.com", AlternativeEmail2: "a@company.test"}, []domain.Payment{payment("payer@company.test", -1, 1000)}, true, true, false},
		{"corporate stale cannot activate", domain.Member{Email: "a@company.test"}, []domain.Payment{payment("payer@company.test", -14, 1000)}, false, false, false},
		{"corporate paid grace not activation", domain.Member{Email: "a@company.test"}, []domain.Payment{payment("payer@company.test", -13, 1000)}, false, false, false},
		{"corporate free grace", domain.Member{Email: "a@company.test"}, []domain.Payment{payment("payer@company.test", -13, 0)}, true, true, false},
		{"public domain cannot activate colleague", domain.Member{Email: "colleague@gmail.com"}, []domain.Payment{payment("payer@gmail.com", -1, 100)}, false, false, false},
		{"disposable domain", domain.Member{Email: "colleague@yopmail.com"}, []domain.Payment{payment("payer@yopmail.com", -1, 100)}, false, false, false},
		{"malformed email", domain.Member{Email: "invalid"}, []domain.Payment{payment("also-invalid", -1, 100)}, false, false, false},
		{"no contribution deactivates", domain.Member{Email: "a@gmail.com", ActiveMembership: true, LastPaymentDate: now}, nil, true, false, false},
		{"inactive no contribution unchanged", domain.Member{Email: "a@gmail.com"}, nil, false, false, false},
		{"already current unchanged", domain.Member{Email: "a@gmail.com", ActiveMembership: true, LastPaymentDate: now.AddDate(0, -1, 0)}, []domain.Payment{payment("a@gmail.com", -1, 100)}, false, true, false},
		{"recent email rate limited", domain.Member{Email: "a@gmail.com", LastContributionEmailDate: now.AddDate(0, 0, -13)}, []domain.Payment{payment("a@gmail.com", -14, 100)}, true, false, false},
		{"email exact boundary rate limited", domain.Member{Email: "a@gmail.com", LastContributionEmailDate: now.AddDate(0, 0, -14)}, []domain.Payment{payment("a@gmail.com", -14, 100)}, true, false, false},
		{"email older than boundary", domain.Member{Email: "a@gmail.com", LastContributionEmailDate: now.AddDate(0, 0, -14).Add(-time.Nanosecond)}, []domain.Payment{payment("a@gmail.com", -14, 100)}, true, false, true},
		{"safety net missing date", domain.Member{Email: "a@company.test", ActiveMembership: true}, []domain.Payment{payment("payer@company.test", -1, 100)}, true, false, false},
		{"safety net old date", domain.Member{Email: "a@company.test", ActiveMembership: true, LastPaymentDate: now.AddDate(0, -14, 0)}, []domain.Payment{payment("payer@company.test", -1, 100)}, true, false, false},
		{"corporate active protected", domain.Member{Email: "a@company.test", ActiveMembership: true, LastPaymentDate: now.AddDate(0, -1, 0)}, []domain.Payment{payment("payer@company.test", -1, 100)}, false, true, false},
		{"latest alternative wins", domain.Member{Email: "a@gmail.com", AlternativeEmail1: "b@gmail.com"}, []domain.Payment{payment("a@gmail.com", -14, 100), payment("b@gmail.com", -1, 100)}, true, true, false},
		{"unrelated form ignored", domain.Member{Email: "a@gmail.com"}, []domain.Payment{{PayerEmail: "a@gmail.com", OrderDate: now, OrderFormSlug: "donation"}}, false, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			members := []domain.Member{tt.member}
			got := domain.Plan(members, tt.payments, now)
			if len(got) != 0 && !tt.wantChange || len(got) == 0 && tt.wantChange {
				t.Fatalf("changes = %+v, want change %v", got, tt.wantChange)
			}
			if len(got) > 0 && (len(got) != 1 || got[0].Member.ActiveMembership != tt.wantActive || got[0].Notify != tt.wantNotify) {
				t.Fatalf("change = %+v, want active=%v notify=%v", got, tt.wantActive, tt.wantNotify)
			}
			if !reflect.DeepEqual(members[0], tt.member) {
				t.Fatal("Plan mutated its input")
			}
		})
	}
}

func TestLatestPayments(t *testing.T) {
	t.Parallel()
	old := payment("a@test.org", -14, 100)
	recent := payment("a@test.org", -1, 1000)
	french := payment("b@test.org", -1, 0)
	french.OrderFormSlug = "cotisation-annuelle"
	ignored := payment("c@test.org", 0, 100)
	ignored.OrderFormSlug = "donation"
	got := domain.LatestPayments([]domain.Payment{old, ignored, french, recent, recent})
	if !reflect.DeepEqual(got, []domain.Payment{recent, french}) {
		t.Fatalf("payments = %+v", got)
	}
}

func TestRenewalIsNotUndoneByStaleSnapshot(t *testing.T) {
	t.Parallel()
	for _, email := range []string{"a@gmail.com", "a@company.test"} {
		member := domain.Member{Id: 1, Email: email, ActiveMembership: true, LastPaymentDate: now.AddDate(0, -24, 0), NumberContributionsEmail: 3}
		p := payment(email, -1, 100)
		changes := domain.Plan([]domain.Member{member}, []domain.Payment{p}, now)
		if len(changes) != 1 || !changes[0].Member.ActiveMembership || changes[0].Member.LastPaymentDate != p.OrderDate || changes[0].Member.NumberContributionsEmail != 0 {
			t.Fatalf("renewal overwritten: %+v", changes)
		}
		if again := domain.Plan([]domain.Member{changes[0].Member}, []domain.Payment{p}, now); len(again) != 0 {
			t.Fatalf("renewal is not idempotent: %+v", again)
		}
	}
}

func TestExpiryBoundaryInstants(t *testing.T) {
	t.Parallel()
	for _, amount := range []float64{0, 100} {
		months := -12
		if amount == 0 {
			months = -13
		}
		for _, offset := range []time.Duration{-time.Nanosecond, 0, time.Nanosecond} {
			p := payment("a@gmail.com", months, amount)
			p.OrderDate = p.OrderDate.Add(offset)
			changes := domain.Plan([]domain.Member{{Email: p.PayerEmail}}, []domain.Payment{p}, now)
			if len(changes) != 1 || changes[0].Member.ActiveMembership != (offset >= 0) {
				t.Fatalf("amount=%v offset=%v changes=%+v", amount, offset, changes)
			}
		}
	}
}

func TestLatestPaymentsEqualDatesKeepFirst(t *testing.T) {
	t.Parallel()
	first := payment("a@gmail.com", -1, 100)
	second := first
	second.Amount = 0
	got := domain.LatestPayments([]domain.Payment{first, second})
	if len(got) != 1 || got[0] != first {
		t.Fatalf("payments=%+v", got)
	}
}

func TestRemindersUseMostRecentRecipientHistory(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name       string
		last       time.Time
		wantEmails int
	}{
		{"recent duplicate", now.AddDate(0, 0, -3), 0},
		{"exact fourteen days", now.AddDate(0, 0, -14), 0},
		{"just over fourteen days", now.AddDate(0, 0, -14).Add(-time.Nanosecond), 1},
		{"future date is conservative", now.AddDate(0, 0, 1), 0},
		{"no history sends once", time.Time{}, 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			members := []domain.Member{
				{Id: 296, Email: "person@gmail.com"},
				{Id: 317, Email: " PERSON@GMAIL.COM ", AlternativeEmail1: "person@gmail.com"},
				{Id: 327, Email: "person@gmail.com", LastContributionEmailDate: tt.last},
			}
			for _, input := range [][]domain.Member{members, {members[2], members[1], members[0]}} {
				changes := domain.Plan(input, []domain.Payment{payment("person@gmail.com", -14, 100)}, now)
				if len(changes) != 3 {
					t.Fatalf("expected all rows updated, got %+v", changes)
				}
				notifications := 0
				for _, change := range changes {
					if change.Member.ActiveMembership {
						t.Fatal("guard must not prevent deactivation")
					}
					if change.Notify {
						notifications++
					}
				}
				if notifications != tt.wantEmails {
					t.Fatalf("notifications=%d, want %d", notifications, tt.wantEmails)
				}
			}
			if members[2].LastContributionEmailDate != tt.last {
				t.Fatal("planning mutated history")
			}
		})
	}
}

func TestReminderHistoryIncludesRowsWithoutPlannedUpdates(t *testing.T) {
	t.Parallel()
	members := []domain.Member{
		{Id: 1, Email: "person@gmail.com"},
		// This row has no matching contribution; its reminder history still applies.
		{Id: 2, Email: "PERSON@GMAIL.COM", LastContributionEmailDate: now.AddDate(0, 0, -3)},
	}
	changes := domain.Plan(members, []domain.Payment{payment("person@gmail.com", -14, 100)}, now)
	if len(changes) != 1 || changes[0].Notify {
		t.Fatalf("changes=%+v", changes)
	}
}

func TestReminderGuardKeepsDifferentRecipientsIndependent(t *testing.T) {
	t.Parallel()
	members := []domain.Member{{Id: 1, Email: "a@gmail.com"}, {Id: 2, Email: "b@gmail.com"}}
	changes := domain.Plan(members, []domain.Payment{payment("a@gmail.com", -14, 100), payment("b@gmail.com", -14, 100)}, now)
	if len(changes) != 2 || !changes[0].Notify || !changes[1].Notify {
		t.Fatalf("changes=%+v", changes)
	}
}

func TestReminderGuardRejectsEmptyRecipient(t *testing.T) {
	t.Parallel()
	member := domain.Member{AlternativeEmail1: "a@gmail.com"}
	changes := domain.Plan([]domain.Member{member}, []domain.Payment{payment("a@gmail.com", -14, 100)}, now)
	if len(changes) != 1 || changes[0].Notify {
		t.Fatalf("changes=%+v", changes)
	}
}
