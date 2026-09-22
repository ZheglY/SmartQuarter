package usecase

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

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
func (uc *IdentityUseCase) UpsertMaxUser(
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
	if utf8.RuneCountInString(displayName) > 255 || utf8.RuneCountInString(username) > 255 {
		return nil, fmt.Errorf("%w: name too long", domain.ErrInvalidInput)
	}

	return uc.repo.UpsertMaxUser(ctx, maxUserID, displayName, username)
}

// GetUserContext собирает профиль, дома, членства и определяет default_house_id
func (uc *IdentityUseCase) GetUserContext(ctx context.Context, userID string) (*domain.UserContext, error) {
	if !validID(userID) {
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
	for _, m := range memberships {
		if m.Status != domain.MembershipStatusActive {
			continue
		}
		if defaultHouseID == "" {
			defaultHouseID = m.HouseID
		}
		if user.DefaultHouseID != nil && *user.DefaultHouseID == m.HouseID {
			defaultHouseID = m.HouseID
			break
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
	if !validID(userID) {
		return nil, fmt.Errorf("%w: invalid user_id UUID", domain.ErrInvalidInput)
	}
	if !validID(houseID) {
		return nil, fmt.Errorf("%w: invalid house_id UUID", domain.ErrInvalidInput)
	}

	return uc.repo.GetMembership(ctx, userID, houseID)
}

// ListMemberships валидирует UUID пользователя и отдает список его членств
func (uc *IdentityUseCase) ListMemberships(ctx context.Context, userID string) ([]domain.Membership, error) {
	if !validID(userID) {
		return nil, fmt.Errorf("%w: invalid user_id UUID", domain.ErrInvalidInput)
	}

	return uc.repo.ListMembershipsByUserID(ctx, userID)
}

func validID(id string) bool {
	parsed, err := uuid.Parse(id)
	return err == nil && parsed != uuid.Nil && parsed.String() == id
}
