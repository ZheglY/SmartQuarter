package postgres

import (
	"context"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/domain"
	"github.com/jackc/pgx/v5"
	"time"
)

func (q *queries) AddOutbox(ctx context.Context, e domain.Event) error {
	_, err := q.db.Exec(ctx, "INSERT INTO outbox_events(event_id,aggregate_type,aggregate_id,event_type,event_version,payload,created_at) VALUES($1,$2,$3,$4,$5,$6,$7)", e.ID, e.AggregateType, e.AggregateID, e.Type, e.Version, e.Payload, e.OccurredAt)
	return translate(err)
}
func (s *Store) PendingCount(ctx context.Context) (int64, error) {
	var n int64
	err := s.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM outbox_events WHERE published_at IS NULL").Scan(&n)
	return n, translate(err)
}

// PublishBatch owns a separate transaction; no business transaction waits for Redis.
// A crash after XADD and before COMMIT can redeliver the same event_id.
func (s *Store) PublishBatch(ctx context.Context, limit int, publish func(context.Context, domain.Event) error) (int, error) {
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, translate(err)
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = tx.Rollback(cleanup)
	}()
	rows, err := tx.Query(ctx, "SELECT event_id::text,aggregate_type,aggregate_id::text,event_type,event_version,payload,created_at FROM outbox_events WHERE published_at IS NULL AND next_attempt_at<=NOW() ORDER BY created_at,event_id LIMIT $1 FOR UPDATE SKIP LOCKED", limit)
	if err != nil {
		return 0, translate(err)
	}
	var events []domain.Event
	for rows.Next() {
		var e domain.Event
		err = rows.Scan(&e.ID, &e.AggregateType, &e.AggregateID, &e.Type, &e.Version, &e.Payload, &e.OccurredAt)
		if err != nil {
			rows.Close()
			return 0, translate(err)
		}
		e.Producer = "issue-service"
		events = append(events, e)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, translate(err)
	}
	sent := 0
	for _, e := range events {
		if err = publish(ctx, e); err != nil {
			if ctx.Err() != nil {
				return sent, ctx.Err()
			}
			_, err = tx.Exec(ctx, "UPDATE outbox_events SET attempts=attempts+1,last_error='notification stream unavailable',next_attempt_at=NOW()+LEAST(300,POWER(2,LEAST(attempts,8))) * INTERVAL '1 second' WHERE event_id=$1", e.ID)
			if err != nil {
				return sent, translate(err)
			}
			break
		}
		_, err = tx.Exec(ctx, "UPDATE outbox_events SET published_at=NOW(),last_error=NULL WHERE event_id=$1", e.ID)
		if err != nil {
			return sent, translate(err)
		}
		sent++
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, translate(err)
	}
	return sent, nil
}
