package grpc

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/observability"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	grpcgo "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestActorAndRecovery(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)
	interceptor := Unary(zap.New(core), observability.NewMetrics(), time.Second)
	info := &grpcgo.UnaryServerInfo{FullMethod: "/smartquarter.issue.v1.IssueService/GetIssue"}
	called := false
	_, err := interceptor(context.Background(), nil, info, func(context.Context, interface{}) (interface{}, error) { called = true; return nil, nil })
	if called || status.Code(err) != codes.Unauthenticated {
		t.Fatalf("missing metadata accepted: %v", err)
	}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-actor-user-id", uuid.NewString(), "x-house-id", uuid.NewString(), "x-actor-role", "RESIDENT"))
	_, err = interceptor(ctx, nil, info, func(ctx context.Context, _ interface{}) (interface{}, error) {
		if Actor(ctx).UserID == "" {
			t.Fatal("actor missing")
		}
		panic("secret")
	})
	if status.Code(err) != codes.Internal || status.Convert(err).Message() != "internal error" {
		t.Fatalf("panic leaked: %v", err)
	}
	entries := logs.FilterMessage("grpc panic recovered").All()
	if len(entries) != 1 || entries[0].Level != zap.ErrorLevel {
		t.Fatal("panic must have a dedicated error log")
	}
	fields := entries[0].ContextMap()
	stack, _ := fields["stacktrace"].(string)
	if !strings.Contains(stack, "TestActorAndRecovery") || fields["panic_type"] != "string" || fields["request_id"] == "" || fields["grpc_method"] != info.FullMethod {
		t.Fatal("panic diagnostic context missing")
	}
	for _, entry := range logs.All() {
		if strings.Contains(fmt.Sprint(entry.ContextMap()), "secret") {
			t.Fatal("raw panic value leaked into logs")
		}
	}
}
