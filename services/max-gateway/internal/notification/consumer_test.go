//go:build integration

package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/ZheglY/SmartQuarter/services/max-gateway/internal/maxapi"
	"github.com/ZheglY/SmartQuarter/services/max-gateway/internal/observability"
	"github.com/ZheglY/SmartQuarter/services/max-gateway/tests/testdata"
)

func TestPendingRecoveryDedupAndDeadLetter(t *testing.T) {
	if os.Getenv("GATEWAY_E2E") != "1" {
		t.Fatal("dedicated Redis required")
	}
	ctx := context.Background()
	r := redis.NewClient(&redis.Options{Addr: "localhost:16379", MaxRetries: -1})
	defer r.Close()
	stream := "gateway-test:" + uuid.NewString()
	defer r.Del(ctx, stream, stream+":dead")
	if e := r.XGroupCreateMkStream(ctx, stream, "test", "0").Err(); e != nil {
		t.Fatal(e)
	}
	var calls atomic.Int64
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(503)
			return
		}
		fmt.Fprint(w, `{"message":{}}`)
	}))
	defer s.Close()
	c := &Consumer{Redis: r, Stream: stream, Group: "test", Name: "recovered", Identity: testdata.Start(t), Bot: &maxapi.Client{BaseURL: s.URL, Token: "test-token"}, Metrics: observability.New(), Logger: zap.NewNop()}
	event := map[string]any{"event_id": uuid.NewString(), "event_type": "issue.status_changed", "event_version": 1, "occurred_at": time.Now().UTC(), "producer": "issue-service", "payload": map[string]string{"issue_id": uuid.NewString(), "house_id": testdata.House, "created_by": testdata.Users[101]}}
	b, _ := json.Marshal(event)
	values := map[string]any{"event_id": event["event_id"], "event_type": event["event_type"], "data": string(b)}
	id, e := r.XAdd(ctx, &redis.XAddArgs{Stream: stream, Values: values}).Result()
	if e != nil {
		t.Fatal(e)
	}
	entries, e := r.XReadGroup(ctx, &redis.XReadGroupArgs{Group: "test", Consumer: "crashed", Streams: []string{stream, ">"}, Count: 1}).Result()
	if e != nil {
		t.Fatal(e)
	}
	c.process(ctx, entries[0].Messages[0])
	pending, e := r.XPending(ctx, stream, "test").Result()
	if e != nil || pending.Count != 1 {
		t.Fatal("failed delivery lost")
	}
	// Simulate an old owner without sleeping for the production reclaim interval.
	if e = r.Do(ctx, "XCLAIM", stream, "test", "crashed", 0, id, "IDLE", 31000).Err(); e != nil {
		t.Fatal(e)
	}
	workerCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() { defer close(done); c.Run(workerCtx) }()
	defer func() { cancel(); <-done }()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		p, _ := r.XPending(ctx, stream, "test").Result()
		if p.Count == 0 && calls.Load() == 2 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if calls.Load() != 2 {
		t.Fatal("pending not recovered", calls.Load())
	}
	r.XAdd(ctx, &redis.XAddArgs{Stream: stream, Values: values})
	r.XAdd(ctx, &redis.XAddArgs{Stream: stream, Values: map[string]any{"data": "malformed"}})
	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if r.XLen(ctx, stream+":dead").Val() == 1 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if calls.Load() != 2 || r.XLen(ctx, stream+":dead").Val() != 1 {
		t.Fatal("dedup or dead letter failed")
	}
	key := "gateway:notification:" + event["event_id"].(string)
	defer r.Del(ctx, key, key+":done", key+":retry")
}
