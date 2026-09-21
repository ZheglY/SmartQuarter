package grpc

import (
	"context"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func LoggingInterceptor(logger *zap.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		start := time.Now()

		md, _ := metadata.FromIncomingContext(ctx)
		reqID := ""
		if vals := md.Get("x-request-id"); len(vals) > 0 {
			reqID = vals[0]
		}
		userID := ""
		if vals := md.Get("x-actor-user-id"); len(vals) > 0 {
			userID = vals[0]
		}
		houseID := ""
		if vals := md.Get("x-house-id"); len(vals) > 0 {
			houseID = vals[0]
		}

		logger.Info("gRPC call started",
			zap.String("method", info.FullMethod),
			zap.String("request_id", reqID),
			zap.String("user_id", userID),
			zap.String("house_id", houseID),
		)

		resp, err = handler(ctx, req)

		duration := time.Since(start)

		if err != nil {
			logger.Error("gRPC call failed",
				zap.String("method", info.FullMethod),
				zap.String("request_id", reqID),
				zap.Duration("duration", duration),
				zap.Error(err),
			)
		} else {
			logger.Info("gRPC call completed",
				zap.String("method", info.FullMethod),
				zap.String("request_id", reqID),
				zap.Duration("duration", duration),
			)
		}

		return resp, err
	}
}
