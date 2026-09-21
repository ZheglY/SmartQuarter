package http

import (
	"net/http"
	"strconv"
	"strings"

	pb "github.com/ZheglY/SmartQuarter/services/max-gateway/internal/gen/smartquarter/issue/v1"
	"github.com/ZheglY/SmartQuarter/services/max-gateway/internal/identity"
)

type CreateUploadRequest struct {
	Filename  string `json:"filename"`
	MIMEType  string `json:"mime_type"`
	SizeBytes int64  `json:"size_bytes"`
}
type CreateIssueRequest struct {
	Category      string   `json:"category"`
	Description   string   `json:"description"`
	LocationText  string   `json:"location_text"`
	AttachmentIDs []string `json:"attachment_ids"`
}

func (a *API) createUpload(w http.ResponseWriter, r *http.Request) {
	var req CreateUploadRequest
	if !a.decode(w, r, &req) {
		return
	}
	v, e := a.Issue.CreateUpload(r.Context(), &pb.CreateUploadRequest{HouseId: current(r).Session.ActiveHouseID, Filename: req.Filename, MimeType: req.MIMEType, SizeBytes: req.SizeBytes})
	if e != nil {
		a.rpcError(w, r, e)
		return
	}
	write(w, 201, map[string]any{"upload_id": v.UploadId, "presigned_url": v.PutUrl, "expires_at": timestamp(v.ExpiresAt), "required_headers": v.RequiredHeaders})
}
func (a *API) completeUpload(w http.ResponseWriter, r *http.Request) {
	if !a.pathID(w, r) || !a.decode(w, r, &struct{}{}) {
		return
	}
	v, e := a.Issue.CompleteUpload(r.Context(), &pb.CompleteUploadRequest{HouseId: current(r).Session.ActiveHouseID, UploadId: r.PathValue("id")})
	if e != nil {
		a.rpcError(w, r, e)
		return
	}
	a.mapped(w, r, 200, func() (any, error) { return attachmentDTO(v) })
}
func (a *API) download(w http.ResponseWriter, r *http.Request) {
	if !a.pathID(w, r) {
		return
	}
	v, e := a.Issue.GetAttachmentDownloadURL(r.Context(), &pb.GetAttachmentDownloadURLRequest{HouseId: current(r).Session.ActiveHouseID, AttachmentId: r.PathValue("id")})
	if e != nil {
		a.rpcError(w, r, e)
		return
	}
	write(w, 200, map[string]any{"url": v.Url, "expires_at": timestamp(v.ExpiresAt)})
}
func (a *API) createIssue(w http.ResponseWriter, r *http.Request) {
	var req CreateIssueRequest
	if !a.decode(w, r, &req) {
		return
	}
	cat, ok := categoriesReverse[req.Category]
	if !ok {
		a.fail(w, r, 400, "INVALID_ARGUMENT", "invalid category")
		return
	}
	for _, id := range req.AttachmentIDs {
		if !identity.ValidID(id) {
			a.fail(w, r, 400, "INVALID_ARGUMENT", "invalid attachment id")
			return
		}
	}
	actor := current(r)
	uc, e := a.Identity.GetUserContext(r.Context(), actor.Session.UserID)
	if e != nil {
		a.rpcError(w, r, e)
		return
	}
	address := ""
	if uc.User.ID == actor.Session.UserID {
		for _, h := range uc.Houses {
			if h.ID == actor.Session.ActiveHouseID {
				address = h.Address
			}
		}
	}
	if strings.TrimSpace(address) == "" {
		a.fail(w, r, 503, "IDENTITY_CONTEXT_UNAVAILABLE", "trusted house address unavailable")
		return
	}
	v, e := a.Issue.CreateIssue(r.Context(), &pb.CreateIssueRequest{HouseId: actor.Session.ActiveHouseID, HouseAddressSnapshot: address, Category: cat, Description: req.Description, LocationText: req.LocationText, AttachmentIds: req.AttachmentIDs})
	if e != nil {
		a.rpcError(w, r, e)
		return
	}
	a.mapped(w, r, 201, func() (any, error) { return issueDTO(v) })
}
func page(r *http.Request) (int32, string, bool) {
	s := r.URL.Query().Get("page_size")
	n := int64(20)
	var e error
	if s != "" {
		n, e = strconv.ParseInt(s, 10, 32)
	}
	token := r.URL.Query().Get("page_token")
	return int32(n), token, e == nil && n > 0 && n <= 100 && len(token) <= 4096
}
func (a *API) listIssues(w http.ResponseWriter, r *http.Request) {
	n, token, ok := page(r)
	if !ok {
		a.fail(w, r, 400, "INVALID_ARGUMENT", "invalid pagination")
		return
	}
	var filters []pb.IssueStatus
	for _, values := range r.URL.Query()["status"] {
		for _, s := range strings.Split(values, ",") {
			v, ok := statusesReverse[s]
			if !ok {
				a.fail(w, r, 400, "INVALID_ARGUMENT", "invalid status")
				return
			}
			filters = append(filters, v)
		}
	}
	v, e := a.Issue.ListIssues(r.Context(), &pb.ListIssuesRequest{HouseId: current(r).Session.ActiveHouseID, Status: filters, PageSize: n, PageToken: token, ChairmanQueue: r.URL.Path == "/api/v1/chairman/issues"})
	if e != nil {
		a.rpcError(w, r, e)
		return
	}
	a.mapped(w, r, 200, func() (any, error) {
		items := make([]IssueDTO, 0, len(v.Items))
		for _, i := range v.Items {
			d, e := issueDTO(i)
			if e != nil {
				return nil, e
			}
			items = append(items, d)
		}
		return map[string]any{"items": items, "next_page_token": v.NextPageToken}, nil
	})
}
func (a *API) getIssue(w http.ResponseWriter, r *http.Request) {
	if !a.pathID(w, r) {
		return
	}
	v, e := a.Issue.GetIssue(r.Context(), &pb.GetIssueRequest{HouseId: current(r).Session.ActiveHouseID, IssueId: r.PathValue("id")})
	if e != nil {
		a.rpcError(w, r, e)
		return
	}
	a.mapped(w, r, 200, func() (any, error) { return detailsDTO(v) })
}
func (a *API) confirm(w http.ResponseWriter, r *http.Request) {
	if !a.pathID(w, r) || !a.decode(w, r, &struct{}{}) {
		return
	}
	v, e := a.Issue.ConfirmIssue(r.Context(), &pb.ConfirmIssueRequest{HouseId: current(r).Session.ActiveHouseID, IssueId: r.PathValue("id")})
	if e != nil {
		a.rpcError(w, r, e)
		return
	}
	write(w, 200, map[string]any{"confirmation_count": v.ConfirmationCount, "confirmed_by_me": v.ConfirmedByMe})
}
func (a *API) changeStatus(w http.ResponseWriter, r *http.Request) {
	var req struct {
		NewStatus string `json:"new_status"`
	}
	if !a.pathID(w, r) || !a.decode(w, r, &req) {
		return
	}
	s, ok := statusesReverse[req.NewStatus]
	if !ok {
		a.fail(w, r, 400, "INVALID_ARGUMENT", "invalid status")
		return
	}
	v, e := a.Issue.UpdateIssueStatus(r.Context(), &pb.UpdateIssueStatusRequest{HouseId: current(r).Session.ActiveHouseID, IssueId: r.PathValue("id"), NewStatus: s})
	if e != nil {
		a.rpcError(w, r, e)
		return
	}
	a.mapped(w, r, 200, func() (any, error) { return issueDTO(v) })
}
func (a *API) generateStatement(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ChairmanNote string `json:"chairman_note"`
	}
	if !a.pathID(w, r) || !a.decode(w, r, &req) {
		return
	}
	v, e := a.Issue.GenerateStatement(r.Context(), &pb.GenerateStatementRequest{HouseId: current(r).Session.ActiveHouseID, IssueId: r.PathValue("id"), ChairmanNote: req.ChairmanNote})
	if e != nil {
		a.rpcError(w, r, e)
		return
	}
	a.mapped(w, r, 201, func() (any, error) { return statementDTO(v) })
}
func (a *API) getStatement(w http.ResponseWriter, r *http.Request) {
	if !a.pathID(w, r) {
		return
	}
	v, e := a.Issue.GetStatement(r.Context(), &pb.GetStatementRequest{HouseId: current(r).Session.ActiveHouseID, IssueId: r.PathValue("id")})
	if e != nil {
		a.rpcError(w, r, e)
		return
	}
	a.mapped(w, r, 200, func() (any, error) { return statementDTO(v) })
}
