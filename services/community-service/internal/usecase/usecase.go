package usecase

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/ZheglY/SmartQuarter/services/community-service/internal/domain"
)

type communityUseCase struct {
	repo domain.CommunityRepository
}

func NewCommunityService(repo domain.CommunityRepository) domain.CommunityService {
	return &communityUseCase{repo: repo}
}

func (u *communityUseCase) CreateAnnouncement(ctx context.Context, houseID, authorID, title, body string) (*domain.Announcement, error) {
	ann := &domain.Announcement{
		ID:           uuid.New().String(),
		HouseID:      houseID,
		AuthorUserID: authorID,
		Title:        title,
		Body:         body,
		Status:       "PUBLISHED",
		CreatedAt:    time.Now(),
		PublishedAt:  time.Now(),
	}
	if err := u.repo.CreateAnnouncement(ctx, ann); err != nil {
		return nil, err
	}
	return ann, nil
}

func (u *communityUseCase) ListAnnouncements(ctx context.Context, houseID string, limit, offset int) ([]domain.Announcement, error) {
	if limit <= 0 {
		limit = 20
	}
	return u.repo.ListAnnouncements(ctx, houseID, limit, offset)
}

func (u *communityUseCase) CreatePoll(ctx context.Context, houseID, authorID, question string, options []string, endsAt time.Time) (*domain.Poll, error) {
	if endsAt.Before(time.Now()) {
		return nil, domain.ErrInvalidDate
	}

	p := &domain.Poll{
		ID:           uuid.New().String(),
		HouseID:      houseID,
		AuthorUserID: authorID,
		Question:     question,
		Status:       "OPEN",
		EndsAt:       endsAt,
		CreatedAt:    time.Now(),
	}
	for i, optText := range options {
		p.Options = append(p.Options, domain.PollOption{
			ID:       uuid.New().String(),
			Text:     optText,
			Position: int32(i),
		})
	}
	if err := u.repo.CreatePoll(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (u *communityUseCase) GetPoll(ctx context.Context, houseID, pollID, userID string) (*domain.PollDetails, error) {
	return u.repo.GetPoll(ctx, houseID, pollID, userID)
}

func (u *communityUseCase) ListPolls(ctx context.Context, houseID string, statuses []string, limit, offset int) ([]domain.Poll, error) {
	if limit <= 0 {
		limit = 20
	}
	return u.repo.ListPolls(ctx, houseID, statuses, limit, offset)
}

func (u *communityUseCase) VotePoll(ctx context.Context, houseID, pollID, optionID, userID string) (int32, string, error) {
	details, err := u.repo.GetPoll(ctx, houseID, pollID, userID)
	if err != nil {
		return 0, "", err
	}

	if details.Poll.Status == "CLOSED" || details.Poll.EndsAt.Before(time.Now()) {
		return 0, "", domain.ErrPollClosed
	}

	err = u.repo.VotePoll(ctx, houseID, pollID, optionID, userID)
	if err != nil {
		return 0, "", err
	}

	details, err = u.repo.GetPoll(ctx, houseID, pollID, userID)
	if err != nil {
		return 0, "", err
	}
	return details.TotalVotes, details.MyOptionID, nil
}

func (u *communityUseCase) CreateCalendarEvent(ctx context.Context, houseID, authorID, title, desc string, startsAt, endsAt time.Time) (*domain.CalendarEvent, error) {
	if !endsAt.After(startsAt) {
		return nil, domain.ErrInvalidDate
	}

	e := &domain.CalendarEvent{
		ID:          uuid.New().String(),
		HouseID:     houseID,
		CreatedBy:   authorID,
		Title:       title,
		Description: desc,
		StartsAt:    startsAt,
		EndsAt:      endsAt,
		CreatedAt:   time.Now(),
	}
	if err := u.repo.CreateCalendarEvent(ctx, e); err != nil {
		return nil, err
	}
	return e, nil
}

func (u *communityUseCase) ListCalendarEvents(ctx context.Context, houseID string, from, to time.Time) ([]domain.CalendarEvent, error) {
	return u.repo.ListCalendarEvents(ctx, houseID, from, to)
}

func (u *communityUseCase) CreateInitiative(ctx context.Context, houseID, authorID, title, desc string) (*domain.Initiative, error) {
	now := time.Now()
	i := &domain.Initiative{
		ID:            uuid.New().String(),
		HouseID:       houseID,
		AuthorUserID:  authorID,
		Title:         title,
		Description:   desc,
		Status:        "OPEN",
		SupportsCount: 0,
		SupportedByMe: false,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := u.repo.CreateInitiative(ctx, i); err != nil {
		return nil, err
	}
	return i, nil
}

func (u *communityUseCase) ListInitiatives(ctx context.Context, houseID, userID string, limit, offset int) ([]domain.Initiative, error) {
	if limit <= 0 {
		limit = 20
	}
	return u.repo.ListInitiatives(ctx, houseID, userID, limit, offset)
}

func (u *communityUseCase) SupportInitiative(ctx context.Context, houseID, initiativeID, userID string) (int32, error) {
	err := u.repo.SupportInitiative(ctx, houseID, initiativeID, userID)
	if err != nil {
		return 0, err
	}

	items, err := u.repo.ListInitiatives(ctx, houseID, userID, 1, 0)
	if err != nil || len(items) == 0 {
		return 0, err
	}
	for _, item := range items {
		if item.ID == initiativeID {
			return item.SupportsCount, nil
		}
	}
	return 0, nil
}
