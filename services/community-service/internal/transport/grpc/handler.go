package grpc

import (
	"context"
	"errors"
	"strconv"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/ZheglY/SmartQuarter/services/community-service/internal/domain"
	communityv1 "github.com/ZheglY/SmartQuarter/services/community-service/internal/gen/community/v1"
)

type CommunityHandler struct {
	communityv1.UnimplementedCommunityServiceServer
	useCase domain.CommunityService
	logger  *zap.Logger
}

func NewCommunityHandler(useCase domain.CommunityService, logger *zap.Logger) *CommunityHandler {
	return &CommunityHandler{useCase: useCase, logger: logger}
}

func extractContext(ctx context.Context, requestedHouseID string) (userID, role string, err error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", "", status.Error(codes.Unauthenticated, "metadata is not provided")
	}

	userVals := md.Get("x-actor-user-id")
	if len(userVals) == 0 {
		return "", "", status.Error(codes.Unauthenticated, "x-actor-user-id is missing")
	}

	roleVals := md.Get("x-actor-role")
	roleStr := "RESIDENT"
	if len(roleVals) > 0 {
		roleStr = roleVals[0]
	}

	houseVals := md.Get("x-house-id")
	if len(houseVals) > 0 && houseVals[0] != requestedHouseID {
		return "", "", status.Error(codes.PermissionDenied, "access to this house is denied")
	}

	return userVals[0], roleStr, nil
}

func mapError(err error) error {
	if errors.Is(err, domain.ErrAlreadyVoted) {
		return status.Errorf(codes.AlreadyExists, err.Error())
	}
	if errors.Is(err, domain.ErrPollClosed) || errors.Is(err, domain.ErrInvalidDate) {
		return status.Errorf(codes.FailedPrecondition, err.Error())
	}
	if errors.Is(err, domain.ErrNotFound) {
		return status.Errorf(codes.NotFound, err.Error())
	}
	return status.Errorf(codes.Internal, "internal server error: %v", err)
}

func (h *CommunityHandler) CreateAnnouncement(ctx context.Context, req *communityv1.CreateAnnouncementRequest) (*communityv1.Announcement, error) {
	actorID, role, err := extractContext(ctx, req.GetHouseId())
	if err != nil {
		return nil, err
	}
	if role != "CHAIRMAN" && role != "ADMIN" {
		return nil, status.Error(codes.PermissionDenied, "only chairman or admin can create announcements")
	}

	ann, err := h.useCase.CreateAnnouncement(ctx, req.GetHouseId(), actorID, req.GetTitle(), req.GetBody())
	if err != nil {
		return nil, mapError(err)
	}

	return &communityv1.Announcement{
		Id:           ann.ID,
		HouseId:      ann.HouseID,
		AuthorUserId: ann.AuthorUserID,
		Title:        ann.Title,
		Body:         ann.Body,
		Status:       communityv1.AnnouncementStatus_ANNOUNCEMENT_STATUS_PUBLISHED,
		PublishedAt:  timestamppb.New(ann.PublishedAt),
		CreatedAt:    timestamppb.New(ann.CreatedAt),
	}, nil
}

func (h *CommunityHandler) ListAnnouncements(ctx context.Context, req *communityv1.ListAnnouncementsRequest) (*communityv1.ListAnnouncementsResponse, error) {
	_, _, err := extractContext(ctx, req.GetHouseId())
	if err != nil {
		return nil, err
	}

	limit := int(req.GetPageSize())
	offset := 0
	if req.GetPageToken() != "" {
		offset, _ = strconv.Atoi(req.GetPageToken())
	}

	list, err := h.useCase.ListAnnouncements(ctx, req.GetHouseId(), limit, offset)
	if err != nil {
		return nil, mapError(err)
	}

	var items []*communityv1.Announcement
	for _, a := range list {
		items = append(items, &communityv1.Announcement{
			Id:           a.ID,
			HouseId:      a.HouseID,
			AuthorUserId: a.AuthorUserID,
			Title:        a.Title,
			Body:         a.Body,
			Status:       communityv1.AnnouncementStatus_ANNOUNCEMENT_STATUS_PUBLISHED,
			PublishedAt:  timestamppb.New(a.PublishedAt),
			CreatedAt:    timestamppb.New(a.CreatedAt),
		})
	}
	nextPage := ""
	if len(items) == limit && limit > 0 {
		nextPage = strconv.Itoa(offset + limit)
	}
	return &communityv1.ListAnnouncementsResponse{Items: items, NextPageToken: nextPage}, nil
}

func (h *CommunityHandler) CreatePoll(ctx context.Context, req *communityv1.CreatePollRequest) (*communityv1.Poll, error) {
	actorID, role, err := extractContext(ctx, req.GetHouseId())
	if err != nil {
		return nil, err
	}
	if role != "CHAIRMAN" && role != "ADMIN" {
		return nil, status.Error(codes.PermissionDenied, "only chairman or admin can create polls")
	}

	p, err := h.useCase.CreatePoll(ctx, req.GetHouseId(), actorID, req.GetQuestion(), req.GetOptions(), req.GetEndsAt().AsTime())
	if err != nil {
		return nil, mapError(err)
	}

	var opts []*communityv1.PollOption
	for _, o := range p.Options {
		opts = append(opts, &communityv1.PollOption{Id: o.ID, Text: o.Text, Position: o.Position})
	}
	return &communityv1.Poll{
		Id:           p.ID,
		HouseId:      p.HouseID,
		AuthorUserId: p.AuthorUserID,
		Question:     p.Question,
		Status:       communityv1.PollStatus_POLL_STATUS_OPEN,
		Options:      opts,
		EndsAt:       timestamppb.New(p.EndsAt),
		CreatedAt:    timestamppb.New(p.CreatedAt),
	}, nil
}

func (h *CommunityHandler) GetPoll(ctx context.Context, req *communityv1.GetPollRequest) (*communityv1.PollDetails, error) {
	actorID, _, err := extractContext(ctx, req.GetHouseId())
	if err != nil {
		return nil, err
	}

	d, err := h.useCase.GetPoll(ctx, req.GetHouseId(), req.GetPollId(), actorID)
	if err != nil {
		return nil, mapError(err)
	}

	var opts []*communityv1.PollOption
	for _, o := range d.Poll.Options {
		opts = append(opts, &communityv1.PollOption{Id: o.ID, Text: o.Text, Position: o.Position})
	}
	var res []*communityv1.PollOptionResult
	for _, r := range d.Results {
		res = append(res, &communityv1.PollOptionResult{OptionId: r.OptionID, VotesCount: r.VotesCount})
	}

	statusEnum := communityv1.PollStatus_POLL_STATUS_OPEN
	if d.Poll.Status == "CLOSED" {
		statusEnum = communityv1.PollStatus_POLL_STATUS_CLOSED
	}

	return &communityv1.PollDetails{
		Poll: &communityv1.Poll{
			Id:           d.Poll.ID,
			HouseId:      d.Poll.HouseID,
			AuthorUserId: d.Poll.AuthorUserID,
			Question:     d.Poll.Question,
			Status:       statusEnum,
			Options:      opts,
			EndsAt:       timestamppb.New(d.Poll.EndsAt),
			CreatedAt:    timestamppb.New(d.Poll.CreatedAt),
		},
		Results:    res,
		TotalVotes: d.TotalVotes,
		MyOptionId: d.MyOptionID,
	}, nil
}

func (h *CommunityHandler) ListPolls(ctx context.Context, req *communityv1.ListPollsRequest) (*communityv1.ListPollsResponse, error) {
	_, _, err := extractContext(ctx, req.GetHouseId())
	if err != nil {
		return nil, err
	}

	limit := int(req.GetPageSize())
	offset := 0
	if req.GetPageToken() != "" {
		offset, _ = strconv.Atoi(req.GetPageToken())
	}

	var statuses []string
	for _, st := range req.GetStatus() {
		statuses = append(statuses, st.String())
	}

	list, err := h.useCase.ListPolls(ctx, req.GetHouseId(), statuses, limit, offset)
	if err != nil {
		return nil, mapError(err)
	}

	var items []*communityv1.Poll
	for _, p := range list {
		statusEnum := communityv1.PollStatus_POLL_STATUS_OPEN
		if p.Status == "CLOSED" {
			statusEnum = communityv1.PollStatus_POLL_STATUS_CLOSED
		}
		items = append(items, &communityv1.Poll{
			Id:           p.ID,
			HouseId:      p.HouseID,
			AuthorUserId: p.AuthorUserID,
			Question:     p.Question,
			Status:       statusEnum,
			EndsAt:       timestamppb.New(p.EndsAt),
			CreatedAt:    timestamppb.New(p.CreatedAt),
		})
	}
	nextPage := ""
	if len(items) == limit && limit > 0 {
		nextPage = strconv.Itoa(offset + limit)
	}
	return &communityv1.ListPollsResponse{Items: items, NextPageToken: nextPage}, nil
}

func (h *CommunityHandler) VotePoll(ctx context.Context, req *communityv1.VotePollRequest) (*communityv1.VotePollResponse, error) {
	actorID, _, err := extractContext(ctx, req.GetHouseId())
	if err != nil {
		return nil, err
	}

	total, myOpt, err := h.useCase.VotePoll(ctx, req.GetHouseId(), req.GetPollId(), req.GetOptionId(), actorID)
	if err != nil {
		return nil, mapError(err)
	}
	return &communityv1.VotePollResponse{PollId: req.GetPollId(), OptionId: req.GetOptionId(), TotalVotes: total, MyOptionId: myOpt}, nil
}

func (h *CommunityHandler) CreateCalendarEvent(ctx context.Context, req *communityv1.CreateCalendarEventRequest) (*communityv1.CalendarEvent, error) {
	actorID, role, err := extractContext(ctx, req.GetHouseId())
	if err != nil {
		return nil, err
	}
	if role != "CHAIRMAN" && role != "ADMIN" {
		return nil, status.Error(codes.PermissionDenied, "only chairman or admin can create calendar events")
	}

	e, err := h.useCase.CreateCalendarEvent(ctx, req.GetHouseId(), actorID, req.GetTitle(), req.GetDescription(), req.GetStartsAt().AsTime(), req.GetEndsAt().AsTime())
	if err != nil {
		return nil, mapError(err)
	}
	return &communityv1.CalendarEvent{
		Id:          e.ID,
		HouseId:     e.HouseID,
		CreatedBy:   e.CreatedBy,
		Title:       e.Title,
		Description: e.Description,
		StartsAt:    timestamppb.New(e.StartsAt),
		EndsAt:      timestamppb.New(e.EndsAt),
		CreatedAt:   timestamppb.New(e.CreatedAt),
	}, nil
}

func (h *CommunityHandler) ListCalendarEvents(ctx context.Context, req *communityv1.ListCalendarEventsRequest) (*communityv1.ListCalendarEventsResponse, error) {
	_, _, err := extractContext(ctx, req.GetHouseId())
	if err != nil {
		return nil, err
	}

	list, err := h.useCase.ListCalendarEvents(ctx, req.GetHouseId(), req.GetFrom().AsTime(), req.GetTo().AsTime())
	if err != nil {
		return nil, mapError(err)
	}
	var items []*communityv1.CalendarEvent
	for _, e := range list {
		items = append(items, &communityv1.CalendarEvent{
			Id:          e.ID,
			HouseId:     e.HouseID,
			CreatedBy:   e.CreatedBy,
			Title:       e.Title,
			Description: e.Description,
			StartsAt:    timestamppb.New(e.StartsAt),
			EndsAt:      timestamppb.New(e.EndsAt),
			CreatedAt:   timestamppb.New(e.CreatedAt),
		})
	}
	return &communityv1.ListCalendarEventsResponse{Items: items}, nil
}

func (h *CommunityHandler) CreateInitiative(ctx context.Context, req *communityv1.CreateInitiativeRequest) (*communityv1.Initiative, error) {
	actorID, _, err := extractContext(ctx, req.GetHouseId())
	if err != nil {
		return nil, err
	}

	i, err := h.useCase.CreateInitiative(ctx, req.GetHouseId(), actorID, req.GetTitle(), req.GetDescription())
	if err != nil {
		return nil, mapError(err)
	}
	return &communityv1.Initiative{
		Id:            i.ID,
		HouseId:       i.HouseID,
		AuthorUserId:  i.AuthorUserID,
		Title:         i.Title,
		Description:   i.Description,
		Status:        communityv1.InitiativeStatus_INITIATIVE_STATUS_OPEN,
		SupportsCount: i.SupportsCount,
		SupportedByMe: i.SupportedByMe,
		CreatedAt:     timestamppb.New(i.CreatedAt),
		UpdatedAt:     timestamppb.New(i.UpdatedAt),
	}, nil
}

func (h *CommunityHandler) ListInitiatives(ctx context.Context, req *communityv1.ListInitiativesRequest) (*communityv1.ListInitiativesResponse, error) {
	actorID, _, err := extractContext(ctx, req.GetHouseId())
	if err != nil {
		return nil, err
	}

	limit := int(req.GetPageSize())
	offset := 0
	if req.GetPageToken() != "" {
		offset, _ = strconv.Atoi(req.GetPageToken())
	}

	list, err := h.useCase.ListInitiatives(ctx, req.GetHouseId(), actorID, limit, offset)
	if err != nil {
		return nil, mapError(err)
	}

	var items []*communityv1.Initiative
	for _, i := range list {
		statusEnum := communityv1.InitiativeStatus_INITIATIVE_STATUS_OPEN
		if i.Status == "CLOSED" {
			statusEnum = communityv1.InitiativeStatus_INITIATIVE_STATUS_CLOSED
		}
		items = append(items, &communityv1.Initiative{
			Id:            i.ID,
			HouseId:       i.HouseID,
			AuthorUserId:  i.AuthorUserID,
			Title:         i.Title,
			Description:   i.Description,
			Status:        statusEnum,
			SupportsCount: i.SupportsCount,
			SupportedByMe: i.SupportedByMe,
			CreatedAt:     timestamppb.New(i.CreatedAt),
			UpdatedAt:     timestamppb.New(i.UpdatedAt),
		})
	}
	nextPage := ""
	if len(items) == limit && limit > 0 {
		nextPage = strconv.Itoa(offset + limit)
	}
	return &communityv1.ListInitiativesResponse{Items: items, NextPageToken: nextPage}, nil
}

func (h *CommunityHandler) SupportInitiative(ctx context.Context, req *communityv1.SupportInitiativeRequest) (*communityv1.SupportInitiativeResponse, error) {
	actorID, _, err := extractContext(ctx, req.GetHouseId())
	if err != nil {
		return nil, err
	}

	count, err := h.useCase.SupportInitiative(ctx, req.GetHouseId(), req.GetInitiativeId(), actorID)
	if err != nil {
		return nil, mapError(err)
	}
	return &communityv1.SupportInitiativeResponse{
		InitiativeId:  req.GetInitiativeId(),
		SupportsCount: count,
		SupportedByMe: true,
	}, nil
}
