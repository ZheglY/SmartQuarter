package grpc

import (
	"context"
	"errors"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/domain"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func mapError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) {
		return status.Error(codes.Canceled, "request canceled")
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return status.Error(codes.DeadlineExceeded, "request deadline exceeded")
	}
	mappings := []struct {
		err     error
		code    codes.Code
		message string
	}{
		{domain.ErrInvalid, codes.InvalidArgument, "invalid request"},
		{domain.ErrUnauthenticated, codes.Unauthenticated, "actor context required"},
		{domain.ErrPermission, codes.PermissionDenied, "access denied"},
		{domain.ErrNotFound, codes.NotFound, "resource not found"},
		{domain.ErrExists, codes.AlreadyExists, "already exists"},
		{domain.ErrPrecondition, codes.FailedPrecondition, "precondition failed"},
		{domain.ErrLimit, codes.ResourceExhausted, "limit exceeded"},
		{domain.ErrUnavailable, codes.Unavailable, "dependency unavailable"},
	}
	for _, m := range mappings {
		if errors.Is(err, m.err) {
			return status.Error(m.code, m.message)
		}
	}
	return status.Error(codes.Internal, "internal error")
}
