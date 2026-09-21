package events

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/ZheglY/SmartQuarter/services/community-service/internal/domain"
)

type OutboxWorker struct {
	repo   domain.CommunityRepository
	redis  *redis.Client
	logger *zap.Logger
}

func NewOutboxWorker(repo domain.CommunityRepository, redisClient *redis.Client, logger *zap.Logger) *OutboxWorker {
	return &OutboxWorker{
		repo:   repo,
		redis:  redisClient,
		logger: logger,
	}
}

func (w *OutboxWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	w.logger.Info("Outbox worker started")

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("Outbox worker stopped")
			return
		case <-ticker.C:
			w.processBatch(ctx)
		}
	}
}

func (w *OutboxWorker) processBatch(ctx context.Context) {
	events, err := w.repo.GetUnpublishedOutboxEvents(ctx, 100)
	if err != nil {
		w.logger.Error("failed to get outbox events", zap.Error(err))
		return
	}

	if len(events) == 0 {
		return
	}

	var publishedIDs []string

	for _, e := range events {
		err := w.redis.XAdd(ctx, &redis.XAddArgs{
			Stream: "max_notifications_stream",
			Values: map[string]interface{}{
				"event_id":   e.EventID,
				"event_type": e.EventType,
				"payload":    string(e.Payload),
			},
		}).Err()

		if err != nil {
			w.logger.Error("failed to publish event to redis", zap.String("event_id", e.EventID), zap.Error(err))
			continue
		}

		publishedIDs = append(publishedIDs, e.EventID)
	}

	if len(publishedIDs) > 0 {
		if err := w.repo.MarkOutboxEventsPublished(ctx, publishedIDs); err != nil {
			w.logger.Error("failed to mark events as published", zap.Error(err))
		} else {
			w.logger.Info("Published outbox events batch", zap.Int("count", len(publishedIDs)))
		}
	}
}
