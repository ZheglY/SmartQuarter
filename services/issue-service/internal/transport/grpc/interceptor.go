package grpc

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/domain"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/observability"
	"github.com/google/uuid"
	"go.uber.org/zap"
	grpcgo "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type actorKey struct{}

func Actor(ctx context.Context) domain.Actor { a, _ := ctx.Value(actorKey{}).(domain.Actor); return a }
func Unary(logger *zap.Logger, metrics *observability.Metrics, timeout time.Duration) grpcgo.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpcgo.UnaryServerInfo, handler grpcgo.UnaryHandler) (response interface{}, err error) {
		started := time.Now()
		md, _ := metadata.FromIncomingContext(ctx)
		id := single(md, "x-request-id")
		if !domain.ValidID(id) {
			id = uuid.NewString()
		}
		_ = grpcgo.SetHeader(ctx, metadata.Pairs("x-request-id", id))
		a := domain.Actor{UserID: single(md, "x-actor-user-id"), HouseID: single(md, "x-house-id"), Role: domain.Role(single(md, "x-actor-role"))}
		method := info.FullMethod
		// Bound metric labels even for custom/test unknown methods.
		if !strings.HasPrefix(method, "/smartquarter.issue.v1.IssueService/") {
			method = "unknown"
		}
		defer func() {
			if recovered := recover(); recovered != nil {
				// Panic values may contain credentials or request data; log only their type.
				logger.Error("grpc panic recovered", zap.String("request_id", id), zap.String("grpc_method", method), zap.String("panic_type", fmt.Sprintf("%T", recovered)), zap.Stack("stacktrace"))
				response = nil
				err = status.Error(codes.Internal, "internal error")
			}
			code := status.Code(err).String()
			metrics.ObserveRPC(method, code, started)
			fields := []zap.Field{zap.String("request_id", id), zap.String("grpc_method", method), zap.String("grpc_code", code), zap.String("error_code", code), zap.Float64("duration_ms", float64(time.Since(started).Microseconds())/1000)}
			if domain.ValidID(a.UserID) {
				fields = append(fields, zap.String("actor_user_id", a.UserID))
			}
			if domain.ValidID(a.HouseID) {
				fields = append(fields, zap.String("house_id", a.HouseID))
			}
			if r, ok := req.(interface{ GetIssueId() string }); ok && domain.ValidID(r.GetIssueId()) {
				fields = append(fields, zap.String("issue_id", r.GetIssueId()))
			}
			logger.Info("grpc request", fields...)
		}()
		if e := a.Validate(); e != nil {
			return nil, mapError(e)
		}
		ctx = context.WithValue(ctx, actorKey{}, a)
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		if e := ctx.Err(); e != nil {
			return nil, mapError(e)
		}
		return handler(ctx, req)
	}
}
func single(md metadata.MD, key string) string {
	v := md.Get(key)
	if len(v) != 1 {
		return ""
	}
	return v[0]
}
