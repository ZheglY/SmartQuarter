package notification

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	ipb "github.com/ZheglY/SmartQuarter/services/max-gateway/internal/gen/smartquarter/identity/v1"
	"github.com/ZheglY/SmartQuarter/services/max-gateway/internal/identity"
	"github.com/ZheglY/SmartQuarter/services/max-gateway/internal/maxapi"
	"github.com/ZheglY/SmartQuarter/services/max-gateway/internal/observability"
)

type Consumer struct {
	Redis               *redis.Client
	Stream, Group, Name string
	Identity            identity.Client
	House               ipb.HouseServiceClient
	Bot                 *maxapi.Client
	Metrics             *observability.Metrics
	Logger              *zap.Logger
}
type Event struct {
	ID         string    `json:"event_id"`
	Type       string    `json:"event_type"`
	Version    int       `json:"event_version"`
	OccurredAt time.Time `json:"occurred_at"`
	Producer   string    `json:"producer"`
	Payload    struct {
		IssueID         string `json:"issue_id"`
		PollID          string `json:"poll_id"`
		InitiativeID    string `json:"initiative_id"`
		AnnouncementID  string `json:"announcement_id"`
		CalendarEventID string `json:"event_id"`
		StartsAt        string `json:"starts_at"`
		HouseID         string `json:"house_id"`
		CreatedBy       string `json:"created_by"`
		To              string `json:"to"`
		RecipientUserID string `json:"recipient_user_id"`
	} `json:"payload"`
}

var ack = redis.NewScript(`redis.call('SET',KEYS[2],'1','EX',604800);redis.call('XACK',KEYS[1],ARGV[1],ARGV[2]);redis.call('DEL',KEYS[3],KEYS[4]);return 1`)
var dead = redis.NewScript(`redis.call('XADD',KEYS[2],'*','source_id',ARGV[2],'reason',ARGV[3],'data',ARGV[4]);redis.call('XACK',KEYS[1],ARGV[1],ARGV[2]);return 1`)

func (c *Consumer) Run(ctx context.Context) {
	if c.Name == "" {
		c.Name = uuid.NewString()
	}
	for ctx.Err() == nil {
		e := c.Redis.XGroupCreateMkStream(ctx, c.Stream, c.Group, "0").Err()
		if e != nil && !strings.Contains(e.Error(), "BUSYGROUP") {
			if !pause(ctx, time.Second) {
				return
			}
			continue
		}
		break
	}
	cursor := "0-0"
	for ctx.Err() == nil {
		readyCtx, readyCancel := context.WithTimeout(ctx, 2*time.Second)
		readyErr := c.Identity.Ready(readyCtx)
		readyCancel()
		if readyErr != nil {
			if !pause(ctx, time.Second) {
				return
			}
			continue
		}

		pending, next, e := c.Redis.XAutoClaim(ctx, &redis.XAutoClaimArgs{Stream: c.Stream, Group: c.Group, Consumer: c.Name, MinIdle: 30 * time.Second, Start: cursor, Count: 10}).Result()
		if e == nil {
			cursor = next
			for _, m := range pending {
				c.process(ctx, m)
			}
		}
		streams, e := c.Redis.XReadGroup(ctx, &redis.XReadGroupArgs{Group: c.Group, Consumer: c.Name, Streams: []string{c.Stream, ">"}, Count: 10, Block: time.Second}).Result()
		if e != nil && e != redis.Nil {
			if !pause(ctx, time.Second) {
				return
			}
			continue
		}
		for _, s := range streams {
			for _, m := range s.Messages {
				c.process(ctx, m)
			}
		}
	}
}
func (c *Consumer) process(ctx context.Context, m redis.XMessage) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	raw, _ := m.Values["data"].(string)
	var e Event
	if json.Unmarshal([]byte(raw), &e) != nil || !identity.ValidID(e.ID) || e.Version != 1 || e.OccurredAt.IsZero() || !validEvent(e) || m.Values["event_id"] != e.ID || m.Values["event_type"] != e.Type {
		c.deadLetter(ctx, m, "invalid_envelope", raw)
		return
	}
	if quietEvent(e.Type) {
		// Routine events remain available to other consumers, but are not chat notifications.
		// A failed ACK stays pending and is reclaimed normally, without contacting MAX.
		c.Redis.XAck(ctx, c.Stream, c.Group, m.ID)
		return
	}
	text := ""
	switch e.Type {
	case "issue.created":
		text = "📍 Спасибо за сигнал! Ваша заявка принята."
	case "issue.confirmed":
		text = "🤝 Сосед подтвердил вашу заявку. Вместе проще добиться решения!"
	case "issue.status_changed":
		text = "🔄 По вашей заявке есть новости. Проверьте новый статус."
	case "statement.generated":
		text = "📝 По вашей заявке подготовлено обращение. Подробности — в карточке."
	default:
		text = eventText(e.Type)
		if text == "" {
			c.deadLetter(ctx, m, "unsupported_event", raw)
			return
		}
	}
	key := "gateway:notification:" + e.ID
	done := key + ":done"
	retry := key + ":retry"
	n, err := c.Redis.Exists(ctx, done).Result()
	if err != nil {
		return
	}
	if n > 0 {
		c.Redis.XAck(ctx, c.Stream, c.Group, m.ID)
		return
	}
	lock, err := c.Redis.SetNX(ctx, key, c.Name, time.Minute).Result()
	if err != nil || !lock {
		return
	}
	defer c.Redis.Del(ctx, key)
	count, err := c.Redis.Incr(ctx, retry).Result()
	if err != nil {
		return
	}
	c.Redis.Expire(ctx, retry, 7*24*time.Hour)
	c.Metrics.NotificationEvents.Inc()
	err = c.deliver(ctx, e, text)
	if err == nil {
		err = ack.Run(ctx, c.Redis, []string{c.Stream, done, key, retry}, c.Group, m.ID).Err()
	}
	result := "sent"
	if err != nil {
		result = "retry"
		c.Metrics.NotificationFailures.Inc()
		if count >= 10 {
			c.deadLetter(ctx, m, "delivery_exhausted", raw)
			result = "dead_letter"
		}
	}
	c.Logger.Info("notification", zap.String("event_id", e.ID), zap.String("event_type", e.Type), zap.String("delivery_result", result), zap.Int64("retry_count", count))
}
func (c *Consumer) deliver(ctx context.Context, e Event, text string) error {
	if c.House != nil {
		return c.deliverRecipients(ctx, e, text)
	}
	if e.Producer != "issue-service" {
		return errors.New("house recipient service unavailable")
	}
	uc, err := c.Identity.GetUserContext(ctx, e.Payload.CreatedBy)
	if err != nil {
		return err
	}
	if uc.User.ID != e.Payload.CreatedBy {
		return errors.New("recipient mismatch")
	}
	m, err := c.Identity.GetMembership(ctx, uc.User.ID, e.Payload.HouseID)
	if err != nil || !m.Authorizes(uc.User.ID, e.Payload.HouseID) {
		return errors.New("recipient membership unavailable")
	}
	maxID, err := strconv.ParseInt(uc.User.MaxUserID, 10, 64)
	if err != nil || maxID <= 0 {
		return errors.New("recipient mapping unavailable")
	}
	label, payload := eventAction(e)
	return c.Bot.SendAction(ctx, maxID, text, label, payload)
}
func (c *Consumer) deadLetter(ctx context.Context, m redis.XMessage, reason, raw string) {
	c.Metrics.NotificationFailures.Inc()
	if err := dead.Run(ctx, c.Redis, []string{c.Stream, c.Stream + ":dead"}, c.Group, m.ID, reason, raw).Err(); err != nil {
		c.Logger.Warn("dead letter unavailable", zap.String("error_code", "REDIS_UNAVAILABLE"))
	}
}
func pause(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
