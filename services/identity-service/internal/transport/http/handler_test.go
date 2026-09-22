package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type testPinger struct{ err error }

func (p testPinger) Ping(context.Context) error { return p.err }

func TestLivenessDoesNotRequireDatabase(t *testing.T) {
	response := httptest.NewRecorder()
	NewRouter(testPinger{err: errors.New("database offline")}).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/livez", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	if response.Header().Get("Content-Type") != "application/json" {
		t.Fatal("liveness response must be JSON")
	}
	var body statusResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Status != "ok" {
		t.Fatalf("unexpected response: %+v", body)
	}
}

func TestReadinessReflectsDatabase(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		code   int
		status string
	}{
		{"available", nil, http.StatusOK, "ok"},
		{"unavailable", errors.New("database offline"), http.StatusServiceUnavailable, "not_ready"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			NewRouter(testPinger{err: tc.err}).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/readyz", nil))
			if response.Code != tc.code {
				t.Fatalf("status = %d, want %d", response.Code, tc.code)
			}
			var body statusResponse
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body.Status != tc.status {
				t.Fatalf("status = %q, want %q", body.Status, tc.status)
			}
		})
	}
}

func TestUnknownRoute(t *testing.T) {
	response := httptest.NewRecorder()
	NewRouter(testPinger{}).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/missing", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", response.Code)
	}
}
