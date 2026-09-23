package domain

import (
	"context"
	"errors"
	"time"
)

var (
	ErrAlreadyVoted = errors.New("already voted")
	ErrPollClosed   = errors.New("poll is closed")
	ErrInvalidDate  = errors.New("ends_at must be strictly after starts_at")
	ErrAccessDenied = errors.New("access denied")
	ErrNotFound     = errors.New("resource not found")
)

type Announcement struct {
	ID           string
	HouseID      string
	AuthorUserID string
	Title        string
	Body         string
	Status       string
	PublishedAt  time.Time
	CreatedAt    time.Time
}

type PollOption struct {
	ID       string
	Text     string
	Position int32
}

type PollOptionResult struct {
	OptionID   string
	VotesCount int32
}

type Poll struct {
	ID           string
	HouseID      string
	AuthorUserID string
	Question     string
	Status       string
	Options      []PollOption
	EndsAt       time.Time
	CreatedAt    time.Time
}

type PollDetails struct {
	Poll       Poll
	Results    []PollOptionResult
	TotalVotes int32
	MyOptionID string
}

type CalendarEvent struct {
	ID          string
	HouseID     string
	CreatedBy   string
	Title       string
	Description string
	StartsAt    time.Time
	EndsAt      time.Time
	CreatedAt   time.Time
}

type Initiative struct {
	ID            string
	HouseID       string
	AuthorUserID  string
	Title         string
	Description   string
	Status        string
	SupportsCount int32
	SupportedByMe bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type OutboxEvent struct {
	OccurredAt time.Time
	EventID    string
	EventType  string
	Payload    []byte
}

type CommunityRepository interface {
	CreateAnnouncement(ctx context.Context, a *Announcement) error
	ListAnnouncements(ctx context.Context, houseID string, limit, offset int) ([]Announcement, error)

	CreatePoll(ctx context.Context, p *Poll) error
	GetPoll(ctx context.Context, houseID, pollID, userID string) (*PollDetails, error)
	ListPolls(ctx context.Context, houseID string, statuses []string, limit, offset int) ([]Poll, error)
	VotePoll(ctx context.Context, houseID, pollID, optionID, userID string) error

	CreateCalendarEvent(ctx context.Context, e *CalendarEvent) error
	ListCalendarEvents(ctx context.Context, houseID string, from, to time.Time) ([]CalendarEvent, error)

	CreateInitiative(ctx context.Context, i *Initiative) error
	ListInitiatives(ctx context.Context, houseID, userID string, limit, offset int) ([]Initiative, error)
	SupportInitiative(ctx context.Context, houseID, initiativeID, userID string) error

	GetUnpublishedOutboxEvents(ctx context.Context, limit int) ([]OutboxEvent, error)
	MarkOutboxEventsPublished(ctx context.Context, eventIDs []string) error
}

type CommunityService interface {
	CreateAnnouncement(ctx context.Context, houseID, authorID, title, body string) (*Announcement, error)
	ListAnnouncements(ctx context.Context, houseID string, limit, offset int) ([]Announcement, error)

	CreatePoll(ctx context.Context, houseID, authorID, question string, options []string, endsAt time.Time) (*Poll, error)
	GetPoll(ctx context.Context, houseID, pollID, userID string) (*PollDetails, error)
	ListPolls(ctx context.Context, houseID string, statuses []string, limit, offset int) ([]Poll, error)
	VotePoll(ctx context.Context, houseID, pollID, optionID, userID string) (int32, string, error)

	CreateCalendarEvent(ctx context.Context, houseID, authorID, title, desc string, startsAt, endsAt time.Time) (*CalendarEvent, error)
	ListCalendarEvents(ctx context.Context, houseID string, from, to time.Time) ([]CalendarEvent, error)

	CreateInitiative(ctx context.Context, houseID, authorID, title, desc string) (*Initiative, error)
	ListInitiatives(ctx context.Context, houseID, userID string, limit, offset int) ([]Initiative, error)
	SupportInitiative(ctx context.Context, houseID, initiativeID, userID string) (int32, error)
}
