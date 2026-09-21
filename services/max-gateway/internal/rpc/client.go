package rpc

import (
	"context"
	"net"
	"strings"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/ZheglY/SmartQuarter/services/max-gateway/internal/observability"
)

func Actor(ctx context.Context, request, user, house, role string) context.Context {
	return metadata.NewOutgoingContext(ctx, metadata.Pairs("x-request-id", request, "x-actor-user-id", user, "x-house-id", house, "x-actor-role", role))
}
func Dial(addr string, timeout time.Duration, logger *zap.Logger, m *observability.Metrics, dialTimeout ...time.Duration) (*grpc.ClientConn, error) {
	dt := 5 * time.Second
	if len(dialTimeout) > 0 {
		dt = dialTimeout[0]
	}
	return grpc.NewClient(addr, grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
		return (&net.Dialer{Timeout: dt}).DialContext(ctx, "tcp", addr)
	}), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithDisableRetry(), grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(4<<20)), grpc.WithUnaryInterceptor(func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, call grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		start := time.Now()
		err := call(ctx, method, req, reply, cc, opts...)
		code := status.Code(err).String()
		m.GRPC.WithLabelValues(method, code).Inc()
		m.GRPCDuration.WithLabelValues(method).Observe(time.Since(start).Seconds())
		logger.Info("grpc request", zap.String("grpc_service", strings.Split(strings.TrimPrefix(method, "/"), "/")[0]), zap.String("grpc_method", method), zap.String("grpc_code", code), zap.Float64("duration_ms", float64(time.Since(start).Microseconds())/1000))
		return err
	}))
}
