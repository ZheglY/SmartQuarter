package house

import (
	"context"
	"encoding/json"
	"github.com/redis/go-redis/v9"
	"log/slog"
	"time"
)

// PublishBatch uses row locks through publication and acknowledgement. A crash
// after XADD may replay an event; consumers deduplicate its stable event UUID.
func (s *Service) PublishBatch(ctx context.Context, r *redis.Client, stream string) error {
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	events, e := rows(ctx, tx, `SELECT to_jsonb(e)-'published_at'-'attempts'-'last_error' FROM identity_outbox e WHERE published_at IS NULL ORDER BY occurred_at LIMIT 100 FOR UPDATE SKIP LOCKED`)
	if e != nil {
		return e
	}
	for _, ev := range events {
		raw, e := json.Marshal(ev)
		if e != nil {
			outboxFailures.Inc()
			return e
		}
		e = r.XAdd(ctx, &redis.XAddArgs{Stream: stream, Values: map[string]any{"event_id": ev.str("event_id"), "event_type": ev.str("event_type"), "data": string(raw)}}).Err()
		if e != nil {
			outboxFailures.Inc()
			if _, e = tx.Exec(ctx, `UPDATE identity_outbox SET attempts=attempts+1,last_error='REDIS_UNAVAILABLE' WHERE event_id=$1`, ev.str("event_id")); e != nil {
				return e
			}
			break
		}
		if _, e = tx.Exec(ctx, `UPDATE identity_outbox SET published_at=now(),attempts=attempts+1,last_error=NULL WHERE event_id=$1`, ev.str("event_id")); e != nil {
			return e
		}
		outboxPublished.WithLabelValues(ev.str("event_type")).Inc()
	}
	return tx.Commit(ctx)
}
func (s *Service) RunOutbox(ctx context.Context, r *redis.Client, stream string) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			batch, cancel := context.WithTimeout(ctx, 10*time.Second)
			e := s.PublishBatch(batch, r, stream)
			cancel()
			if e != nil && ctx.Err() == nil {
				slog.Warn("identity outbox retry", "error_code", "PUBLISH_FAILED")
			}
		}
	}
}
