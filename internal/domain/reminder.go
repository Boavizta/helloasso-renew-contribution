package domain

import (
	"time"

	"github.com/samber/lo"
)

// guardReminders considers all rows, including those without planned updates.
// Reserve one attempt per mailbox: even an ambiguous delivery failure must not
// cause another duplicate row to retry sending in the same run.
func guardReminders(members []Member, changes []Change, now time.Time) []Change {
	latestDates := lo.Reduce(members, func(dates map[string]time.Time, member Member, _ int) map[string]time.Time {
		key := RecipientKey(member.Email)
		if member.LastContributionEmailDate.After(dates[key]) {
			dates[key] = member.LastContributionEmailDate
		}
		return dates
	}, map[string]time.Time{})
	reserved := map[string]bool{}
	return lo.Map(changes, func(change Change, _ int) Change {
		key := RecipientKey(change.Member.Email)
		change.Notify = change.Notify && key != "" && !reserved[key] && latestDates[key].Before(now.AddDate(0, 0, -14))
		if change.Notify {
			reserved[key] = true
		}
		return change
	})
}
