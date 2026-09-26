package http

import (
	cpb "github.com/ZheglY/SmartQuarter/services/max-gateway/internal/gen/smartquarter/community/v1"
	"github.com/ZheglY/SmartQuarter/services/max-gateway/internal/identity"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	"net/http"
	"time"
)

func (a *API) communityRoutes(register func(string, bool, http.HandlerFunc)) {
	for _, route := range []struct {
		pattern string
		manager bool
	}{
		{"GET /api/v1/polls", false}, {"GET /api/v1/polls/{id}", false}, {"POST /api/v1/polls", true}, {"POST /api/v1/polls/{id}/vote", false}, {"POST /api/v1/polls/{id}/close", true},
		{"GET /api/v1/calendar", false}, {"POST /api/v1/calendar", true}, {"PATCH /api/v1/calendar/{id}", true}, {"DELETE /api/v1/calendar/{id}", true},
		{"GET /api/v1/initiatives", false}, {"POST /api/v1/initiatives", false}, {"POST /api/v1/initiatives/{id}/support", false}, {"POST /api/v1/initiatives/{id}/close", true},
	} {
		handler := a.communityWorkflow
		if route.pattern[:3] != "GET" {
			handler = a.idempotent(handler)
		}
		register(route.pattern, route.manager, handler)
	}
}
func (a *API) communityWorkflow(w http.ResponseWriter, r *http.Request) {
	if !a.communityAvailable(w, r) {
		return
	}
	id := r.PathValue("id")
	if id != "" && !a.pathID(w, r) {
		return
	}
	house := current(r).Session.ActiveHouseID
	ctx := r.Context()
	var out proto.Message
	var err error
	code := 200
	invalid := func() { a.fail(w, r, 400, "INVALID_ARGUMENT", "invalid community request") }
	if r.Method == "GET" {
		n, token, ok := page(r)
		if !ok {
			invalid()
			return
		}
		switch r.Pattern {
		case "GET /api/v1/polls":
			req := &cpb.ListPollsRequest{HouseId: house, PageSize: n, PageToken: token}
			if st := r.URL.Query().Get("status"); st != "" {
				switch st {
				case "OPEN":
					req.Status = []cpb.PollStatus{cpb.PollStatus_POLL_STATUS_OPEN}
				case "CLOSED":
					req.Status = []cpb.PollStatus{cpb.PollStatus_POLL_STATUS_CLOSED}
				default:
					invalid()
					return
				}
			}
			out, err = a.Community.ListPolls(ctx, req)
		case "GET /api/v1/polls/{id}":
			out, err = a.Community.GetPoll(ctx, &cpb.GetPollRequest{HouseId: house, PollId: id})
		case "GET /api/v1/calendar":
			from, e1 := time.Parse(time.RFC3339, r.URL.Query().Get("from"))
			to, e2 := time.Parse(time.RFC3339, r.URL.Query().Get("to"))
			if e1 != nil || e2 != nil {
				invalid()
				return
			}
			out, err = a.Community.ListCalendarEvents(ctx, &cpb.ListCalendarEventsRequest{HouseId: house, From: timestamppb.New(from), To: timestamppb.New(to)})
		case "GET /api/v1/initiatives":
			out, err = a.Community.ListInitiatives(ctx, &cpb.ListInitiativesRequest{HouseId: house, PageSize: n, PageToken: token})
		}
	} else {
		switch r.Pattern {
		case "POST /api/v1/polls":
			var b struct {
				Question string    `json:"question"`
				Options  []string  `json:"options"`
				EndsAt   time.Time `json:"ends_at"`
			}
			if !a.decode(w, r, &b) {
				return
			}
			out, err = a.Community.CreatePoll(ctx, &cpb.CreatePollRequest{HouseId: house, Question: b.Question, Options: b.Options, EndsAt: timestamppb.New(b.EndsAt)})
			code = 201
		case "POST /api/v1/polls/{id}/vote":
			var b struct {
				OptionID string `json:"option_id"`
			}
			if !a.decode(w, r, &b) {
				return
			}
			if !identity.ValidID(b.OptionID) {
				invalid()
				return
			}
			out, err = a.Community.VotePoll(ctx, &cpb.VotePollRequest{HouseId: house, PollId: id, OptionId: b.OptionID})
		case "POST /api/v1/calendar", "PATCH /api/v1/calendar/{id}":
			var b struct {
				Title       string    `json:"title"`
				Description string    `json:"description"`
				StartsAt    time.Time `json:"starts_at"`
				EndsAt      time.Time `json:"ends_at"`
			}
			if !a.decode(w, r, &b) {
				return
			}
			if r.Method == "POST" {
				out, err = a.Community.CreateCalendarEvent(ctx, &cpb.CreateCalendarEventRequest{HouseId: house, Title: b.Title, Description: b.Description, StartsAt: timestamppb.New(b.StartsAt), EndsAt: timestamppb.New(b.EndsAt)})
				code = 201
			} else {
				out, err = a.Community.UpdateCalendarEvent(ctx, &cpb.UpdateCalendarEventRequest{HouseId: house, Id: id, Title: b.Title, Description: b.Description, StartsAt: timestamppb.New(b.StartsAt), EndsAt: timestamppb.New(b.EndsAt)})
			}
		case "POST /api/v1/initiatives":
			var b struct {
				Title       string `json:"title"`
				Description string `json:"description"`
			}
			if !a.decode(w, r, &b) {
				return
			}
			out, err = a.Community.CreateInitiative(ctx, &cpb.CreateInitiativeRequest{HouseId: house, Title: b.Title, Description: b.Description})
			code = 201
		default:
			var b struct{}
			if !a.decode(w, r, &b) {
				return
			}
			switch r.Pattern {
			case "POST /api/v1/polls/{id}/close":
				out, err = a.Community.ClosePoll(ctx, &cpb.GetPollRequest{HouseId: house, PollId: id})
			case "DELETE /api/v1/calendar/{id}":
				out, err = a.Community.DeleteCalendarEvent(ctx, &cpb.DeleteCalendarEventRequest{HouseId: house, Id: id})
			case "POST /api/v1/initiatives/{id}/support":
				out, err = a.Community.SupportInitiative(ctx, &cpb.SupportInitiativeRequest{HouseId: house, InitiativeId: id})
			case "POST /api/v1/initiatives/{id}/close":
				out, err = a.Community.CloseInitiative(ctx, &cpb.SupportInitiativeRequest{HouseId: house, InitiativeId: id})
			}
		}
	}
	a.houseResponse(w, r, code, out, err)
}
