//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/domain"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/observability"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/outbox"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"net"
	"testing"
	"time"
)

func TestRedisRecoveryAndDuplicateDelivery(t *testing.T) {
	pool, store := database(t)
	ctx := context.Background()
	// Bind an unused local socket without serving Redis: deterministic unavailable endpoint.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	offline := redis.NewClient(&redis.Options{Addr: listener.Addr().String(), DialTimeout: 100 * time.Millisecond, ReadTimeout: 100 * time.Millisecond, WriteTimeout: 100 * time.Millisecond, MaxRetries: -1, ContextTimeoutEnabled: true})
	defer offline.Close()
	online := redis.NewClient(&redis.Options{Addr: "localhost:16379", MaxRetries: -1})
	defer online.Close()
	name := "issue-test:" + uuid.NewString()
	defer online.Del(ctx, name)
	ev := domain.Event{ID: uuid.NewString(), AggregateType: "issue", AggregateID: uuid.NewString(), Type: "issue.created", Version: 1, OccurredAt: time.Now().UTC(), Payload: json.RawMessage("{}"), Producer: "issue-service"}
	if err = store.Read().AddOutbox(ctx, ev); err != nil {
		t.Fatal(err)
	}
	bad := outbox.New(store, offline, name, 100, time.Second, observability.NewMetrics(), zap.NewNop())
	n, err := store.PublishBatch(ctx, 100, bad.Publish)
	if err != nil || n != 0 {
		t.Fatal("offline batch", n, err)
	}
	var attempts int
	var published *time.Time
	if err = pool.QueryRow(ctx, "SELECT attempts,published_at FROM outbox_events WHERE event_id=$1", ev.ID).Scan(&attempts, &published); err != nil || attempts != 1 || published != nil {
		t.Fatal("missing retry state", err)
	}
	if _, err = pool.Exec(ctx, "UPDATE outbox_events SET next_attempt_at=NOW() WHERE event_id=$1", ev.ID); err != nil {
		t.Fatal(err)
	}
	good := outbox.New(store, online, name, 100, time.Second, observability.NewMetrics(), zap.NewNop())
	// Simulate successful XADD followed by a process crash before the SQL acknowledgement.
	if err = good.Publish(ctx, ev); err != nil {
		t.Fatal(err)
	}
	n, err = store.PublishBatch(ctx, 100, good.Publish)
	if err != nil || n != 1 {
		t.Fatal("recovery", n, err)
	}
	messages, err := online.XRange(ctx, name, "-", "+").Result()
	if err != nil || len(messages) != 2 {
		t.Fatal("redelivery missing", err)
	}
	for _, m := range messages {
		if m.Values["event_id"] != ev.ID {
			t.Fatal("event id changed on redelivery")
		}
	}
}
