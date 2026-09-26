package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/ZheglY/SmartQuarter/services/max-gateway/internal/config"
	cpb "github.com/ZheglY/SmartQuarter/services/max-gateway/internal/gen/smartquarter/community/v1"
	ipb "github.com/ZheglY/SmartQuarter/services/max-gateway/internal/gen/smartquarter/identity/v1"
	pb "github.com/ZheglY/SmartQuarter/services/max-gateway/internal/gen/smartquarter/issue/v1"
	"github.com/ZheglY/SmartQuarter/services/max-gateway/internal/identity"
	"github.com/ZheglY/SmartQuarter/services/max-gateway/internal/maxapi"
	"github.com/ZheglY/SmartQuarter/services/max-gateway/internal/observability"
	"github.com/ZheglY/SmartQuarter/services/max-gateway/internal/state"
)

type API struct {
	Config         config.Config
	Store          state.Store
	Identity       identity.Client
	House          ipb.HouseServiceClient
	Issue          pb.IssueServiceClient
	Community      cpb.CommunityServiceClient
	Bot            *maxapi.Client
	Metrics        *observability.Metrics
	Logger         *zap.Logger
	IssueReady     func(context.Context) error
	CommunityReady func(context.Context) error
}

func (a *API) Handler() http.Handler {
	mux := http.NewServeMux()
	a.houseRoutes(mux)
	mux.HandleFunc("GET /livez", func(w http.ResponseWriter, r *http.Request) { write(w, 200, map[string]string{"status": "ok"}) })
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { write(w, 200, map[string]string{"status": "ok"}) })
	mux.HandleFunc("GET /readyz", a.ready)
	mux.HandleFunc("GET /api/v1/health", a.ready)
	mux.Handle("GET /metrics", promhttp.HandlerFor(a.Metrics.Registry, promhttp.HandlerOpts{}))
	mux.HandleFunc("POST /api/v1/session/max", a.origin(a.bootstrap))
	mux.HandleFunc("POST /webhooks/max", a.webhook)
	session := func(pattern string, h http.HandlerFunc) {
		mux.HandleFunc(pattern, a.origin(a.authenticate(false, false, h)))
	}
	business := func(pattern string, manager bool, h http.HandlerFunc) {
		mux.HandleFunc(pattern, a.origin(a.authenticate(true, manager, h)))
	}
	session("GET /api/v1/me", a.me)
	a.communityRoutes(business)
	session("POST /api/v1/session/active-house", a.switchHouse)
	session("POST /api/v1/session/logout", a.logout)
	business("POST /api/v1/uploads", false, a.createUpload)
	business("POST /api/v1/uploads/{id}/complete", false, a.completeUpload)
	business("GET /api/v1/attachments/{id}/download-url", false, a.download)
	business("POST /api/v1/issues", false, a.idempotent(a.createIssue))
	business("GET /api/v1/issues", false, a.listIssues)
	business("GET /api/v1/issues/{id}", false, a.getIssue)
	business("POST /api/v1/issues/{id}/confirm", false, a.confirm)
	business("GET /api/v1/chairman/issues", true, a.listIssues)
	business("PATCH /api/v1/issues/{id}/status", true, a.changeStatus)
	business("POST /api/v1/issues/{id}/statement", true, a.idempotent(a.generateStatement))
	business("GET /api/v1/issues/{id}/statement", true, a.getStatement)
	business("POST /api/v1/announcements", true, a.idempotent(a.createAnnouncement))
	business("GET /api/v1/announcements", false, a.listAnnouncements)
	business("GET /api/v1/house/service-contacts", false, a.listContacts)
	business("POST /api/v1/chairman/service-contacts", true, a.idempotent(a.contactMutation))
	business("PATCH /api/v1/chairman/service-contacts/{id}", true, a.idempotent(a.contactMutation))
	business("DELETE /api/v1/chairman/service-contacts/{id}", true, a.idempotent(a.contactMutation))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { a.fail(w, r, 404, "NOT_FOUND", "route not found") })
	return a.observe(mux)
}
func (a *API) ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if a.Store.R.Ping(ctx).Err() != nil || a.Identity.Ready(ctx) != nil || a.IssueReady == nil || a.IssueReady(ctx) != nil {
		a.fail(w, r, 503, "DEPENDENCY_UNAVAILABLE", "required dependency unavailable")
		return
	}
	if a.Community != nil && (a.CommunityReady == nil || a.CommunityReady(ctx) != nil) {
		a.fail(w, r, 503, "DEPENDENCY_UNAVAILABLE", "community unavailable")
		return
	}
	write(w, 200, map[string]string{"status": "ready"})
}
func write(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}
func (a *API) fail(w http.ResponseWriter, r *http.Request, code int, name, message string) {
	a.Metrics.HTTPError.WithLabelValues(name).Inc()
	if t, _ := r.Context().Value(traceKey{}).(*trace); t != nil {
		t.Error = name
	}
	write(w, code, map[string]any{"error": map[string]string{"code": name, "message": message, "request_id": requestID(r)}})
}
func (a *API) rpcError(w http.ResponseWriter, r *http.Request, err error) {
	code := status.Code(err)
	if errors.Is(err, context.DeadlineExceeded) {
		code = codes.DeadlineExceeded
	}
	if errors.Is(err, context.Canceled) {
		code = codes.Canceled
	}
	c, n, m := 500, "INTERNAL_ERROR", "internal error"
	switch code {
	case codes.InvalidArgument:
		c, n, m = 400, "INVALID_ARGUMENT", "invalid request"
	case codes.Unauthenticated:
		c, n, m = 401, "UNAUTHENTICATED", "authentication required"
	case codes.PermissionDenied:
		c, n, m = 403, "PERMISSION_DENIED", "access denied"
	case codes.NotFound:
		c, n, m = 404, "RESOURCE_NOT_FOUND", "resource not found"
	case codes.AlreadyExists:
		c, n, m = 409, "ALREADY_EXISTS", "resource already exists"
	case codes.FailedPrecondition:
		c, n, m = 409, "FAILED_PRECONDITION", "precondition failed"
	case codes.Aborted:
		c, n, m = 409, "TRANSACTION_RETRY", "transaction conflicted; retry after refreshing"
	case codes.ResourceExhausted:
		c, n, m = 429, "RESOURCE_EXHAUSTED", "limit exceeded"
	case codes.Unavailable, codes.Unimplemented:
		c, n, m = 503, "DEPENDENCY_UNAVAILABLE", "dependency unavailable"
	case codes.DeadlineExceeded:
		c, n, m = 504, "DEADLINE_EXCEEDED", "request timed out"
	case codes.Canceled:
		c, n, m = 408, "REQUEST_CANCELED", "request canceled"
	}
	a.fail(w, r, c, n, m)
}
func (a *API) decode(w http.ResponseWriter, r *http.Request, v any) bool {
	mediaType, _, mediaErr := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if mediaErr != nil || mediaType != "application/json" {
		a.fail(w, r, 415, "UNSUPPORTED_MEDIA_TYPE", "application/json required")
		return false
	}
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	e := d.Decode(v)
	if e == nil {
		var extra any
		if d.Decode(&extra) != io.EOF {
			e = errors.New("trailing JSON")
		}
	}
	if e != nil {
		var tooBig *http.MaxBytesError
		if errors.As(e, &tooBig) {
			a.fail(w, r, 413, "BODY_TOO_LARGE", "request body too large")
		} else {
			a.fail(w, r, 400, "INVALID_ARGUMENT", "invalid JSON request")
		}
		return false
	}
	return true
}
func (a *API) pathID(w http.ResponseWriter, r *http.Request) bool {
	if !identity.ValidID(r.PathValue("id")) {
		a.fail(w, r, 400, "INVALID_ARGUMENT", "invalid resource id")
		return false
	}
	return true
}
