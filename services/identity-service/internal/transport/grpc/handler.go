package grpc

import (
	"context"
	"github.com/ZheglY/SmartQuarter/services/identity-service/internal/domain"
	identityv1 "github.com/ZheglY/SmartQuarter/services/identity-service/internal/gen/smartquarter/identity/v1"
)

// IdentityUseCase описывает методы бизнес-логики, необходимые транспорту
type IdentityUseCase interface {
	UpsertMaxUser(ctx context.Context, maxUserID int64, displayName, username string) (*domain.User, error)
	GetUserContext(ctx context.Context, userID string) (*domain.UserContext, error)
	GetMembership(ctx context.Context, userID, houseID string) (*domain.Membership, error)
	ListMemberships(ctx context.Context, userID string) ([]domain.Membership, error)
}

type Handler struct {
	identityv1.UnimplementedIdentityServiceServer
	useCase IdentityUseCase
}

func NewHandler(useCase IdentityUsecase) *Handler {
	return &Handler {
		useCase: useCase,
	}
}

// UpsertMaxUser создает или обновляет пользователя из данных MAX initData
func (h *Handler) UpsertMaxUser(
	ctx context.Context,
	req *identityv1.UpsertMaxUserRequest,
) (*identityv1.User, error) {
	user, err := h.useCase.UpsertMaxUser(ctx, req.GetMaxUserId(), req.GetDisplayName(), req.GetUsername())
	if err != nil {
		return nil, mapError(err)
	}

	return toProtoUser(*user), nil
}

// GetUserContext собирает агрегированный контекст пользователя для 
// сессии Gateway
func (h *Handler) GetUserContext(
	ctx context.Context,
	req *identityv1.GetUserContextRequest,
) (*identityv1.UserContext, error) {
	uCtx, err := h.useCase.GetUserContext(ctx, req.GetUserId())
	if err != nil {
		return nil, mapError(err)
	}

	protoHouses := make([]*identityv1.House, 0, len(uCtx.Houses))
	for _, house := range u.Ctx.Houses {
		protoHouses = append(protoHouses, toProtoHouse(house))
	}

	protoMemberships := make([]*identityv1.Membership, 0, len(uCtx.Memberships))
	for _, m := range uCtx.Memberships {
		protoMemberships = append(protoMemberships, toProtoMembership(m))
	}

	return &identityv1.UserContext{
		User: toProtouser(uCtx.User),
		Houses: protoHouses,
		Memberships: protoMemberships, 
		DefaultHouseId: uCtx.DefaultHouseID,
	}, nil
}

// Getmembership проверяет и возвращает роль и статус участника
// в конкретном доме
func (h *Handler) GetMembership(
	ctx context.Context,
	req *identityv1.GetMebershipRequest,
) (*identityv1.Membership, error) {
	m, err := h.useCase.GetMembership(ctx, req.GetUserId(), req.GetHouseId())
	if err != nil {
		return nil, mapError(err)
	}

	return toProtomembership(*m), nil
}

// ListMemberships возвращает все членства пользователя в домах
func (h *Handler) ListMemberships(
	ctx context.Context,
	req *identityv1.ListMembershipsRequest,
) (*identityv1.ListmembershipsResponse, error) {
	membershipsm err := h.useCase.ListMemberships(ctx, req.GetuserId())
	if err != nil {
		return nil, mapError(err)
	}

	item := make([]*identityv1.Membership, 0, len(memberships))
	for _, m := range memberships {
		items = append(items, toProtomemberships(m))
	}

	return &identityv1.ListmembershipsResponse{
		Items: item,
	}, nil
}