package notification

import "testing"

func TestProducerCannotSpoofHouseDecision(t *testing.T) {
	e := Event{Producer: "identity-service", Type: "house.registration.approved"}
	e.Payload.RecipientUserID = "11111111-1111-4111-8111-111111111111"
	if !validEvent(e) {
		t.Fatal("valid direct decision rejected")
	}
	e.Producer = "community-service"
	if validEvent(e) {
		t.Fatal("community impersonated Identity")
	}
	e.Producer = "identity-service"
	e.Type = "announcement.created"
	if validEvent(e) {
		t.Fatal("Identity impersonated Community")
	}
	e.Producer = "community-service"
	e.Payload.HouseID = "22222222-2222-4222-8222-222222222222"
	if !validEvent(e) {
		t.Fatal("valid announcement rejected")
	}
	e.Payload.HouseID = ""
	if validEvent(e) {
		t.Fatal("broadcast without scope accepted")
	}
	e.Producer = "unknown"
	if validEvent(e) {
		t.Fatal("unknown producer accepted")
	}
}
