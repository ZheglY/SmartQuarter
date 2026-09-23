package http

import (
	"encoding/json"
	cpb "github.com/ZheglY/SmartQuarter/services/max-gateway/internal/gen/smartquarter/community/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"net/http"
)

type contactBody struct {
	Category         string `json:"category"`
	Title            string `json:"title"`
	OrganizationName string `json:"organization_name"`
	Phone            string `json:"phone"`
	AdditionalPhone  string `json:"additional_phone"`
	Email            string `json:"email"`
	Website          string `json:"website"`
	Description      string `json:"description"`
	Emergency        bool   `json:"emergency"`
	SortOrder        int32  `json:"sort_order"`
}

func (a *API) contactMutation(w http.ResponseWriter, r *http.Request) {
	if !a.communityAvailable(w, r) {
		return
	}
	var body contactBody
	if r.Method == "DELETE" {
		if !a.decode(w, r, &struct{}{}) {
			return
		}
	} else if !a.decode(w, r, &body) {
		return
	}
	if r.Method != "POST" && !a.pathID(w, r) {
		return
	}
	raw, e := json.Marshal(body)
	if e != nil {
		a.fail(w, r, 400, "INVALID_ARGUMENT", "invalid contact")
		return
	}
	input := new(cpb.ServiceContactInput)
	if protojson.Unmarshal(raw, input) != nil {
		a.fail(w, r, 400, "INVALID_ARGUMENT", "invalid contact")
		return
	}
	h := current(r).Session.ActiveHouseID
	var out *cpb.ServiceContact
	code := 200
	switch r.Method {
	case "POST":
		out, e = a.Community.CreateServiceContact(r.Context(), &cpb.CreateServiceContactRequest{HouseId: h, Contact: input})
		code = 201
	case "PATCH":
		out, e = a.Community.UpdateServiceContact(r.Context(), &cpb.UpdateServiceContactRequest{HouseId: h, Id: r.PathValue("id"), Contact: input})
	case "DELETE":
		out, e = a.Community.ArchiveServiceContact(r.Context(), &cpb.ArchiveServiceContactRequest{HouseId: h, Id: r.PathValue("id")})
	}
	a.houseResponse(w, r, code, out, e)
}
func (a *API) listContacts(w http.ResponseWriter, r *http.Request) {
	if !a.communityAvailable(w, r) {
		return
	}
	manager := current(r).Role == "CHAIRMAN" || current(r).Role == "ADMIN"
	out, e := a.Community.ListServiceContacts(r.Context(), &cpb.ListServiceContactsRequest{HouseId: current(r).Session.ActiveHouseID, IncludeArchived: manager && r.URL.Query().Get("include_archived") == "true"})
	if e != nil {
		a.rpcError(w, r, e)
		return
	}
	// Residents receive only public directory fields. Proto defaults must not
	// reintroduce the internal author/timestamp fields removed by Community.
	raw, e := (protojson.MarshalOptions{UseProtoNames: true, EmitDefaultValues: true}).Marshal(out)
	if e != nil {
		a.fail(w, r, 502, "INVALID_UPSTREAM_RESPONSE", "invalid contact response")
		return
	}
	var dto struct {
		Items []map[string]any `json:"items"`
	}
	if json.Unmarshal(raw, &dto) != nil {
		a.fail(w, r, 502, "INVALID_UPSTREAM_RESPONSE", "invalid contact response")
		return
	}
	if !manager {
		for _, item := range dto.Items {
			delete(item, "created_by")
			delete(item, "created_at")
			delete(item, "updated_at")
		}
	}
	write(w, 200, dto)
}
