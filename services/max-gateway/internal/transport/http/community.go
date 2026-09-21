package http

import (
	"net/http"
	"strings"

	cpb "github.com/ZheglY/SmartQuarter/services/max-gateway/internal/gen/smartquarter/community/v1"
)

func announcementDTO(v *cpb.Announcement) (any, error) {
	if v == nil || v.Status != cpb.AnnouncementStatus_ANNOUNCEMENT_STATUS_PUBLISHED {
		return nil, invalidUpstream
	}
	return map[string]any{"id": v.Id, "house_id": v.HouseId, "author_user_id": v.AuthorUserId, "title": v.Title, "body": v.Body, "status": "PUBLISHED", "published_at": timestamp(v.PublishedAt), "created_at": timestamp(v.CreatedAt)}, nil
}
func (a *API) communityAvailable(w http.ResponseWriter, r *http.Request) bool {
	if a.Community == nil {
		a.fail(w, r, 503, "COMMUNITY_UNAVAILABLE", "community not configured")
		return false
	}
	return true
}
func (a *API) createAnnouncement(w http.ResponseWriter, r *http.Request) {
	if !a.communityAvailable(w, r) {
		return
	}
	var req struct {
		Title string `json:"title"`
		Body  string `json:"body"`
	}
	if !a.decode(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Title) == "" || len(req.Title) > 200 || strings.TrimSpace(req.Body) == "" || len(req.Body) > 10000 {
		a.fail(w, r, 400, "INVALID_ARGUMENT", "invalid announcement")
		return
	}
	v, e := a.Community.CreateAnnouncement(r.Context(), &cpb.CreateAnnouncementRequest{HouseId: current(r).Session.ActiveHouseID, Title: req.Title, Body: req.Body})
	if e != nil {
		a.rpcError(w, r, e)
		return
	}
	a.mapped(w, r, 201, func() (any, error) { return announcementDTO(v) })
}
func (a *API) listAnnouncements(w http.ResponseWriter, r *http.Request) {
	if !a.communityAvailable(w, r) {
		return
	}
	n, token, ok := page(r)
	if !ok {
		a.fail(w, r, 400, "INVALID_ARGUMENT", "invalid pagination")
		return
	}
	v, e := a.Community.ListAnnouncements(r.Context(), &cpb.ListAnnouncementsRequest{HouseId: current(r).Session.ActiveHouseID, PageSize: n, PageToken: token})
	if e != nil {
		a.rpcError(w, r, e)
		return
	}
	a.mapped(w, r, 200, func() (any, error) {
		items := []any{}
		for _, i := range v.Items {
			d, e := announcementDTO(i)
			if e != nil {
				return nil, e
			}
			items = append(items, d)
		}
		return map[string]any{"items": items, "next_page_token": v.NextPageToken}, nil
	})
}
