package notification

import (
	"github.com/ZheglY/SmartQuarter/services/max-gateway/internal/identity"
	"strconv"
	"strings"
	"time"
)

// Payload is an application route token, never an arbitrary URL.
func eventAction(e Event) (string, string) {
	label, kind, id := "Открыть дом", "houses", ""
	switch {
	case e.Producer == "issue-service":
		label, kind, id = "Перейти к заявке", "issue", e.Payload.IssueID
	case strings.HasPrefix(e.Type, "poll."):
		label, kind, id = "Перейти к опросу", "poll", e.Payload.PollID
	case e.Type == "initiative.created":
		label, kind, id = "Перейти к инициативе", "initiative", e.Payload.InitiativeID
	case e.Type == "announcement.created":
		label, kind, id = "Прочитать объявление", "announcement", e.Payload.AnnouncementID
	case e.Type == "calendar.event_created":
		label, kind, id = "Перейти к событию", "calendar", e.Payload.CalendarEventID
	case strings.HasPrefix(e.Type, "service_contact."):
		return "Контакты служб", "contacts"
	case e.Type == "house.join.created":
		return "Посмотреть заявки", "join_review"
	case strings.HasPrefix(e.Type, "house.chairman.transfer_"):
		return "Посмотреть предложение", "transfer"
	case strings.HasPrefix(e.Type, "house.registration.") || strings.HasPrefix(e.Type, "house.join."):
		return "Мои заявки", "my_requests"
	}
	if !identity.ValidID(id) {
		return label, kind
	}
	payload := kind + "_" + id
	if identity.ValidID(e.Payload.HouseID) {
		payload += "_" + e.Payload.HouseID
	}
	if kind == "calendar" {
		if date, err := time.Parse(time.RFC3339, e.Payload.StartsAt); err == nil {
			payload += "_" + strconv.FormatInt(date.Unix(), 10)
		}
	}
	return label, payload
}
