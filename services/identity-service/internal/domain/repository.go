package domain

import "context"

type Repository interface {
	// UpsertMaxuser вставляет или обновляет пользователя платформы MAX
	UpsertMaxUser(ctx context.Context, maxUserID int64, displayName, username string) (*User, error)

	// GetUserByID возвращает пользователя по его внутреннему UUID
	GetUserByID(ctx context.Context, userID string) (*User, error)

	// GetHouseByID возвращает данные дома по UUID
	GetHouseByID(ctx context.Context, houseID string) (*House, error)

	// GetMembership возвращает отношение пользователя к конкретному дому
	GetMembership(ctx, context.Context, userID, houseID string) (*Membership, error)

	// ListMembershipByUserID возвращает все членства пользователя
	ListMembershipByUserID(ctx context.Context, userID string) ([]Membership, error)

	// ListHouseByIDs возвращает список домов по их UUID
	ListHouseByIDs(ctx context.Context, houseIDs []string) ([]House, error)

	// Ping проверяет готовновть сервиса
	Ping(ctx context.Context) error
}