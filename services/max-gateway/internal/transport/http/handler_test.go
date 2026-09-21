package http

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/ZheglY/SmartQuarter/services/max-gateway/internal/config"
	pb "github.com/ZheglY/SmartQuarter/services/max-gateway/internal/gen/smartquarter/issue/v1"
	"github.com/ZheglY/SmartQuarter/services/max-gateway/internal/observability"
)

func testAPI() *API {
	return &API{Config: config.Config{RequestTimeout: time.Second, Origins: []string{"https://app.example"}, CookieName: "sq_session"}, Metrics: observability.New(), Logger: zap.NewNop()}
}
func TestHealth(t *testing.T) {
	a := testAPI()
	w := httptest.NewRecorder()
	a.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/livez", nil))
	if w.Code != 200 || w.Header().Get("Content-Type") != "application/json" {
		t.Fatal(w.Code)
	}
	var body map[string]string
	if json.Unmarshal(w.Body.Bytes(), &body) != nil || body["status"] != "ok" {
		t.Fatal(w.Body)
	}
}
func TestMiddlewareBoundaries(t *testing.T) {
	for _, tc := range []struct {
		method, path, origin string
		want                 int
	}{{"GET", "/missing", "", 404}, {"GET", "/api/v1/issues", "", 401}, {"POST", "/api/v1/issues", "", 403}, {"GET", "/api/v1/me", "https://evil.example", 403}, {"OPTIONS", "/api/v1/issues", "https://app.example", 204}, {"POST", "/webhooks/max", "", 401}} {
		t.Run(tc.method+tc.path+tc.origin, func(t *testing.T) {
			a := testAPI()
			w := httptest.NewRecorder()
			r := httptest.NewRequest(tc.method, tc.path, nil)
			r.Header.Set("Origin", tc.origin)
			r.Header.Set("X-Actor-Role", "ADMIN")
			r.Header.Set("X-Request-Id", "11111111-1111-4111-8111-111111111111")
			a.Handler().ServeHTTP(w, r)
			if w.Code != tc.want {
				t.Fatalf("%d want %d %s", w.Code, tc.want, w.Body)
			}
			if w.Header().Get("X-Request-Id") != "11111111-1111-4111-8111-111111111111" {
				t.Fatal("request id lost")
			}
		})
	}
}
func TestErrorMapping(t *testing.T) {
	for code, want := range map[codes.Code]int{codes.InvalidArgument: 400, codes.Unauthenticated: 401, codes.PermissionDenied: 403, codes.NotFound: 404, codes.AlreadyExists: 409, codes.FailedPrecondition: 409, codes.ResourceExhausted: 429, codes.Unavailable: 503, codes.Unimplemented: 503, codes.Internal: 500, codes.Canceled: 408, codes.DeadlineExceeded: 504} {
		w := httptest.NewRecorder()
		testAPI().rpcError(w, httptest.NewRequest("GET", "/", nil), status.Error(code, "password=secret SQL"))
		if w.Code != want || bytes.Contains(w.Body.Bytes(), []byte("secret")) {
			t.Fatalf("%s: %d %s", code, w.Code, w.Body)
		}
	}
}
func TestMappers(t *testing.T) {
	if _, e := issueDTO(&pb.Issue{Category: 999, Status: 1}); e == nil {
		t.Fatal("unknown enum accepted")
	}
	v, e := detailsDTO(&pb.IssueDetails{Issue: &pb.Issue{Category: 4, Status: 1}, Timeline: []*pb.TimelineEvent{{PayloadJson: `{"from":"DETECTED"}`}}})
	if e != nil {
		t.Fatal(e)
	}
	b, _ := json.Marshal(v)
	for _, s := range []string{`"resolved_at":null`, `"latest_statement":null`, `"category":"INFRASTRUCTURE"`, `"attachments":[]`, `"payload":{"from":"DETECTED"}`} {
		if !bytes.Contains(b, []byte(s)) {
			t.Fatal(string(b), s)
		}
	}
	if _, e := statementDTO(&pb.StatementDraft{SourceSnapshotJson: `[]`}); e == nil {
		t.Fatal("array accepted as object")
	}
}
func TestStrictJSON(t *testing.T) {
	for _, body := range []string{`{"role":"ADMIN"}`, `{} {}`, `{broken`} {
		r := httptest.NewRequest("POST", "/", bytes.NewBufferString(body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		if testAPI().decode(w, r, &struct{}{}) {
			t.Fatal("invalid body accepted")
		}
	}
}
