package outbox

import (
	"context"
	"encoding/json"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/domain"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/observability"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"time"
)

type Store interface {
	PublishBatch(context.Context, int, func(context.Context, domain.Event) error) (int, error)
	PendingCount(context.Context) (int64, error)
}
type Stream interface {
	XAdd(context.Context, *redis.XAddArgs) *redis.StringCmd
}
type Publisher struct {
	store   Store
	stream  Stream
	name    string
	batch   int
	poll    time.Duration
	metrics *observability.Metrics
	logger  *zap.Logger
}

func New(store Store, stream Stream, name string, batch int, poll time.Duration, metrics *observability.Metrics, logger *zap.Logger) *Publisher {
	return &Publisher{store: store, stream: stream, name: name, batch: batch, poll: poll, metrics: metrics, logger: logger}
}
func (p *Publisher) Publish(ctx context.Context, e domain.Event) error {
	data, err := json.Marshal(e)
	if err != nil {
		return err
	}
	attempt, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	err = p.stream.XAdd(attempt, &redis.XAddArgs{Stream: p.name, Values: map[string]interface{}{"event_id": e.ID, "event_type": e.Type, "data": string(data)}}).Err()
	if err != nil {
		p.metrics.OutboxErrors.Inc()
		p.logger.Warn("outbox publication failed", zap.String("event_id", e.ID), zap.String("error_code", "redis_unavailable"))
	}
	return err
}
func (p *Publisher) Run(ctx context.Context) {
	ticker := time.NewTicker(p.poll)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			return
		}
		batchCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		_, err := p.store.PublishBatch(batchCtx, p.batch, p.Publish)
		if err != nil && ctx.Err() == nil {
			p.metrics.OutboxErrors.Inc()
			p.logger.Warn("outbox batch failed", zap.String("error_code", "outbox_unavailable"))
		}
		if count, e := p.store.PendingCount(batchCtx); e == nil {
			p.metrics.OutboxPending.Set(float64(count))
		}
		cancel()
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
