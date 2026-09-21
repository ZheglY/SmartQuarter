package grpc

import (
	"context"
	"errors"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/domain"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"testing"
)

func TestErrorMapping(t *testing.T) {
	for _, tc := range []struct {
		err  error
		code codes.Code
	}{
		{domain.ErrInvalid, codes.InvalidArgument}, {domain.ErrUnauthenticated, codes.Unauthenticated}, {domain.ErrPermission, codes.PermissionDenied}, {domain.ErrNotFound, codes.NotFound}, {domain.ErrExists, codes.AlreadyExists}, {domain.ErrPrecondition, codes.FailedPrecondition}, {domain.ErrLimit, codes.ResourceExhausted}, {domain.ErrUnavailable, codes.Unavailable}, {context.Canceled, codes.Canceled}, {context.DeadlineExceeded, codes.DeadlineExceeded}, {errors.New("SQL secret"), codes.Internal},
	} {
		if got := status.Code(mapError(tc.err)); got != tc.code {
			t.Errorf("got %v want %v", got, tc.code)
		}
	}
}
