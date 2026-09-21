package http

import (
	"context"
	"errors"
	"github.com/prometheus/client_golang/prometheus"
	"net/http/httptest"
	"testing"
)

func TestTechnicalRoutes(t *testing.T) {
	ok := func(context.Context) error { return nil }
	for _, test := range []struct {
		path string
		code int
	}{{"/livez", 200}, {"/readyz", 200}, {"/metrics", 200}, {"/api/v1/issues", 404}} {
		r := httptest.NewRecorder()
		NewHandler(ok, ok, prometheus.NewRegistry()).ServeHTTP(r, httptest.NewRequest("GET", test.path, nil))
		if r.Code != test.code {
			t.Fatalf("%s status %d", test.path, r.Code)
		}
	}
	bad := func(context.Context) error { return errors.New("private database error") }
	r := httptest.NewRecorder()
	NewHandler(bad, ok, prometheus.NewRegistry()).ServeHTTP(r, httptest.NewRequest("GET", "/readyz", nil))
	if r.Code != 503 || r.Body.String() != "{\"status\":\"not_ready\"}\n" {
		t.Fatalf("unsafe readiness: %s", r.Body.String())
	}
}
