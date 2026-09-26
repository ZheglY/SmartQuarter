package notification

import "testing"

func TestImportantNotificationsRemainEnabled(t *testing.T) {
	for _, kind := range []string{"poll.created", "announcement.created", "initiative.created", "calendar.event_created", "issue.status_changed", "statement.generated", "house.join.created", "house.join.approved", "house.join.rejected", "house.registration.approved", "house.registration.rejected", "house.membership.deactivated", "house.chairman.assigned", "house.chairman.transfer_requested"} {
		if quietEvent(kind) {
			t.Errorf("important event suppressed: %s", kind)
		}
	}
}
