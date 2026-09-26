package notification

import "testing"

func TestEventAction(t *testing.T) {
	id, house := "11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222"
	for _, kind := range []string{"poll.created", "initiative.created", "announcement.created", "calendar.event_created", "issue.created"} {
		e := Event{Type: kind, Producer: "community-service"}
		e.Payload.HouseID = house
		prefix, label := "", ""
		switch kind {
		case "poll.created":
			e.Payload.PollID = id
			prefix, label = "poll", "Перейти к опросу"
		case "initiative.created":
			e.Payload.InitiativeID = id
			prefix, label = "initiative", "Перейти к инициативе"
		case "announcement.created":
			e.Payload.AnnouncementID = id
			prefix, label = "announcement", "Прочитать объявление"
		case "calendar.event_created":
			e.Payload.CalendarEventID = id
			prefix, label = "calendar", "Перейти к событию"
		case "issue.created":
			e.Producer = "issue-service"
			e.Payload.IssueID = id
			prefix, label = "issue", "Перейти к заявке"
		}
		gotLabel, payload := eventAction(e)
		if gotLabel != label || payload != prefix+"_"+id+"_"+house {
			t.Fatalf("%s: %q %q", kind, gotLabel, payload)
		}
	}
	e := Event{Type: "poll.created"}
	e.Payload.PollID = "https://evil.example"
	if _, p := eventAction(e); p != "poll" {
		t.Fatal(p)
	}
}
