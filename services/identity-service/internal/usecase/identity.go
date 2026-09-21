package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/ZheglY/SmartQuarter/services/identity-service/internal/domain"
	"github.com/google/uuid"
)

type IdentityUseCase struct {
	repo domain.Repository
}

func New(repo domain.Repository) *IdentityUseCase {
	return &IdentityUseCase{
		repo: repo,
	}
}

// UpsertMaxUser валидирует входные параметры и обновляет/создает
// пользователя MAX
func (uc *Identity.UseCase) UpsertMaxUser(
	ctx context.Context,
	maxUserID int64,
	displayName, username string,
) (*domain.User, error) {
	if maxUserID <= 0 {
		return nil, fmt.Errorf("%w: max_user_id must be greater than 0", domain.ErrInvalidInput)
	}

	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		displayName = "Житель"
	}
	username = strings.TrimSpace(username)

	return uc.repo.UpsertMaxUser(ctx, maxUserID, displayName, username)
}

// GetUserContext собирает профиль, дома, членства и определяет default_house_id
func (uc *Identity.UseCase) GetUserContext(ctx context.Context, userID string) (*domain.UserContext, error) {
	if _, err := uuid.Parse(userID); err != nil {
		return nil, fmt.Errorf("%w: invalid user_id UUID", domain.ErrInvalidInput)
	}

	// 1. Получаем профиль пользователя
	user, err := uc.repo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// 2. Получаем все членства пользователя в домах
	memberships, err := uc.repo.ListMembershipsByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// 3. Собираем уникальные ID домов, к которым привязан пользователь
	houseIDsMap := make(map[string]struct{})
	var houseIDs []string
	for _, m := range memberships {
		if _, exists := houseIDsMap[m.HouseID]; !exists {
			houseIDsMap[m.HouseID] = struct{}{}
			houseIDs = append(houseIDs, m.HouseID)
		}
	}

	// 4. Загружаем данные по всем домам батчем
	houses, err := uc.repo.ListHousesByIDs(ctx, houseIDs)
	if err != nil {
		return nil, err
	}

	// 5. Определяем default_house_id
	defaultHouseID := ""
	if user.DefaultHouseID != nil && *user.DefaultHouseID != "" {
		defaultHouseID = *user.DefaultHouseID
	} else if len(memberships) > 0 {
		// Если в профиле дефолнтый дом не выставлен, берем первый активный дом
		for _, m := range memberships {
			if m.Status == domain.MembershipStatusActive {
				defaultHouseID = m.HouseID
				break
			}
		}
		// Если активных нет. берем первый из списка
		if defaultHouseID == "" {
			defaultHouseID = memberships[0].HouseID
		}
	}

	return &domain.UserContext{
		User:           *user,
		Houses:         houses,
		Memberships:    memberships,
		DefaultHouseID: defaultHouseID,
	}, nil
}

// GetMembership валидирует входные UUID и возвращает членство
// пользователя в доме
func (uc *IdentityUseCase) GetMembership(ctx context.Context, userID, houseID string) (*domain.Membership, error) {
	if _, err := uuid.Parse(userID); err != nil {
		return nil, fmt.Errorf("%w: invalid user_id UUID", domain.ErrInvalidInput)
	}
	if _, err := uuid.Parse(houseID); err != nil {
		return nil, fmt.Errorf("%w: invalid house_id UUID", domain.ErrInvalidInput)
	}

	return uc.repo.GetMembership(ctx, userID, houseID)
}

// ListMemberships валидирует UUID пользователя и отдает список его членств
func (uc *IdentityUseCase) ListMemberships(ctx context.Context, userID string) ([]domain.Membership, error) {
	if _, err := uuid.Parse(userID); err != nil {
		return nil, fmt.Errorf("%w: invalid user_id UUID", domain.ErrInvalidInput)
	}

	return uc.repo.ListMembershipsByUserID(ctx, userID)
}
