package grpc

import (
	"context"
	"google.golang.org/grpc/codes"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
	"time"
)

type Pinger interface{ Ping(context.Context) error }
type Health struct {
	healthpb.UnimplementedHealthServer
	DB Pinger
}

func (h *Health) Check(ctx context.Context, req *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error) {
	if req.GetService() != "" && req.GetService() != "smartquarter.identity.v1.IdentityService" {
		return nil, status.Error(codes.NotFound, "unknown service")
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	state := healthpb.HealthCheckResponse_SERVING
	if h.DB == nil || h.DB.Ping(ctx) != nil {
		state = healthpb.HealthCheckResponse_NOT_SERVING
	}
	return &healthpb.HealthCheckResponse{Status: state}, nil
}
