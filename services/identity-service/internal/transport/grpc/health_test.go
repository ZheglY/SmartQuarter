package grpc

import (
	"context"
	"errors"
	"google.golang.org/grpc/codes"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
	"testing"
)

type ping struct{ err error }

func (p ping) Ping(context.Context) error { return p.err }
func TestHealthTracksDatabase(t *testing.T) {
	for _, tc := range []struct {
		db   Pinger
		want healthpb.HealthCheckResponse_ServingStatus
	}{{nil, healthpb.HealthCheckResponse_NOT_SERVING}, {ping{}, healthpb.HealthCheckResponse_SERVING}, {ping{errors.New("offline")}, healthpb.HealthCheckResponse_NOT_SERVING}} {
		v, err := (&Health{DB: tc.db}).Check(context.Background(), &healthpb.HealthCheckRequest{Service: "smartquarter.identity.v1.IdentityService"})
		if err != nil || v.Status != tc.want {
			t.Fatalf("%v %v", v, err)
		}
	}
}
func TestErrorsDoNotLeakDatabaseDetails(t *testing.T) {
	if got := mapError(errors.New("postgres://secret:password@db")); status.Code(got) != codes.Internal || status.Convert(got).Message() != "internal error" {
		t.Fatal(got)
	}
	if status.Code(mapError(context.DeadlineExceeded)) != codes.DeadlineExceeded {
		t.Fatal("deadline lost")
	}
}
