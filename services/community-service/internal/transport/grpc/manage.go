package grpc

import (
	"context"
	pb "github.com/ZheglY/SmartQuarter/services/community-service/internal/gen/community/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func managerContext(ctx context.Context, house, id string) (string, error) {
	user, role, e := extractContext(ctx, house)
	if e != nil {
		return "", e
	}
	if role != "CHAIRMAN" && role != "ADMIN" {
		return "", status.Error(codes.PermissionDenied, "chairman required")
	}
	if !validID(id) {
		return "", status.Error(codes.InvalidArgument, "invalid resource id")
	}
	return user, nil
}
func (h *CommunityHandler) ClosePoll(ctx context.Context, r *pb.GetPollRequest) (*pb.PollDetails, error) {
	user, e := managerContext(ctx, r.GetHouseId(), r.GetPollId())
	if e != nil {
		return nil, e
	}
	if _, e = h.useCase.ClosePoll(ctx, r.HouseId, r.PollId, user); e != nil {
		return nil, mapError(e)
	}
	return h.GetPoll(ctx, r)
}
func (h *CommunityHandler) UpdateCalendarEvent(ctx context.Context, r *pb.UpdateCalendarEventRequest) (*pb.CalendarEvent, error) {
	if _, e := managerContext(ctx, r.GetHouseId(), r.GetId()); e != nil {
		return nil, e
	}
	if r.GetStartsAt().CheckValid() != nil || r.GetEndsAt().CheckValid() != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid dates")
	}
	v, e := h.useCase.UpdateCalendarEvent(ctx, r.HouseId, r.Id, r.Title, r.Description, r.StartsAt.AsTime(), r.EndsAt.AsTime())
	if e != nil {
		return nil, mapError(e)
	}
	return &pb.CalendarEvent{Id: v.ID, HouseId: v.HouseID, CreatedBy: v.CreatedBy, Title: v.Title, Description: v.Description, StartsAt: timestamppb.New(v.StartsAt), EndsAt: timestamppb.New(v.EndsAt), CreatedAt: timestamppb.New(v.CreatedAt)}, nil
}
func (h *CommunityHandler) DeleteCalendarEvent(ctx context.Context, r *pb.DeleteCalendarEventRequest) (*pb.DeleteCalendarEventResponse, error) {
	if _, e := managerContext(ctx, r.GetHouseId(), r.GetId()); e != nil {
		return nil, e
	}
	if e := h.useCase.DeleteCalendarEvent(ctx, r.HouseId, r.Id); e != nil {
		return nil, mapError(e)
	}
	return &pb.DeleteCalendarEventResponse{Id: r.Id}, nil
}
func (h *CommunityHandler) CloseInitiative(ctx context.Context, r *pb.SupportInitiativeRequest) (*pb.Initiative, error) {
	user, e := managerContext(ctx, r.GetHouseId(), r.GetInitiativeId())
	if e != nil {
		return nil, e
	}
	v, e := h.useCase.CloseInitiative(ctx, r.HouseId, r.InitiativeId, user)
	if e != nil {
		return nil, mapError(e)
	}
	return &pb.Initiative{Id: v.ID, HouseId: v.HouseID, AuthorUserId: v.AuthorUserID, Title: v.Title, Description: v.Description, Status: pb.InitiativeStatus_INITIATIVE_STATUS_CLOSED, SupportsCount: v.SupportsCount, SupportedByMe: v.SupportedByMe, CreatedAt: timestamppb.New(v.CreatedAt), UpdatedAt: timestamppb.New(v.UpdatedAt)}, nil
}
