package yandexs3

import (
	"context"
	"errors"
	"fmt"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/domain"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/storage"
	"github.com/aws/smithy-go"
	"testing"
)

func TestTranslateStorageFailures(t *testing.T) {
	for _, tc := range []struct {
		code string
		want error
	}{
		{"NoSuchBucket", domain.ErrUnavailable},
		{"NoSuchKey", storage.ErrObjectNotFound},
		{"NotFound", storage.ErrObjectNotFound},
		{"PreconditionFailed", domain.ErrPrecondition},
		{"AccessDenied", domain.ErrUnavailable},
	} {
		t.Run(tc.code, func(t *testing.T) {
			original := fmt.Errorf("SDK wrapper: %w", &smithy.GenericAPIError{Code: tc.code, Message: "private storage details"})
			got := translate(original)
			if !errors.Is(got, tc.want) {
				t.Fatalf("got %v want %v", got, tc.want)
			}
			if got.Error() == "private storage details" {
				t.Fatal("SDK details exposed")
			}
		})
	}
	for _, err := range []error{context.Canceled, context.DeadlineExceeded} {
		if !errors.Is(translate(fmt.Errorf("SDK: %w", err)), err) {
			t.Fatal("context error lost")
		}
	}
}
