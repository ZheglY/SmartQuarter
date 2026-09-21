package domain

import (
	"encoding/json"
	"time"
)

type TimelineEvent struct {
	ID, IssueID, Type, ActorUserID string
	Payload                        json.RawMessage
	CreatedAt                      time.Time
}
type Event struct {
	ID            string          `json:"event_id"`
	Type          string          `json:"event_type"`
	Version       int             `json:"event_version"`
	OccurredAt    time.Time       `json:"occurred_at"`
	Producer      string          `json:"producer"`
	Payload       json.RawMessage `json:"payload"`
	AggregateType string          `json:"-"`
	AggregateID   string          `json:"-"`
}
