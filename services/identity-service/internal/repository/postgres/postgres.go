package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ZheglY/SmartQuarter/services/identity-service/internal/domain"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgres(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{
		pool: pool,
	}
}

// UpsertMaxUser создает пользователя или обновляет disply_name и username при повторном входе
func (r *PostgresRepository) UpsertMaxUser(
	ctx context.Context,
	maxUserID int64,
	displayName, username string,
) (*domain.User, error) {
	query := `
		INSERT INTO users (max_user_id, display_name, username, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW())
		ON CONFLICT (max_user_id) DO UPDATE SET
			display_name = EXCLUDED.display_name,
			username = EXCLUDED.username,
			updated_at = NOW()
		RETURNING id, max_user_id, display_name, username, default_house_id, created_at, updated_at;
	`

	var u domain.User
	err := r.pool.QueryRow(ctx, query, maxUserID, displayName, username).Scan(
		&u.ID,
		&u.MaxUserID,
		&u.DisplayName,
		&u.Username,
		&u.DefaultHouseID,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("repository.UpsertMaxuser: %w", err)
	}

	return &u, nil
}

// GetUserByID возвращает пользователя по его внутреннему UUID
func (r *PostgresRepository) GetUserByID(ctx context.Context, userID string) (*domain.User, error) {
	query := `
		SELECT id, max_user_id, display_name, username, default_house_id, created_at, updated_at
		FROM users
		WHERE id = $1;
	`

	var u domain.User
	err := r.pool.QueryRow(ctx, query, userID).Scan(
		&u.ID,
		&u.MaxUserID,
		&u.DisplayName,
		&u.Username,
		&u.DefaultHouseID,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		return nil, fmt.Errorf("repository.GetUserByID: %w", err)
	}

	return &u, nil
}

// GetHouseByID возвращает информацию о доме
func (r *PostgresRepository) GetHouseByID(ctx context.Context, houseID string) (*domain.House, error) {
	query := `
	 	SELECT id, name, address, city, created_at, updated_at
		FROM houses
		WHERE id = $1
	`

	var h domain.House
	err := r.pool.QueryRow(ctx, query, houseID).Scan(
		&h.ID,
		&h.Name,
		&h.Address,
		&h.City,
		&h.CreatedAt,
		&h.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrHouseNotFound
		}
		return nil, fmt.Errorf("repository.GetHouseByID: %w", err)
	}

	return &h, nil
}

// GetMembership возвращает членство пользователя в конкретном доме с
// проверкой роли и статуса
func (r *PostgresRepository) GetMembership(ctx context.Context, userID, houseID string) (*domain.Membership, error) {
	query := `
	 	SELECT id, user_id, house_id, role, status, created_at, updated_at
		FROM memberships
		WHERE user_id = $1 AND house_id = $2;
	`

	var m domain.Membership
	var roleStr, statusStr string

	err := r.pool.QueryRow(ctx, query, userID, houseID).Scan(
		&m.ID,
		&m.UserID,
		&m.HouseID,
		&roleStr,
		&statusStr,
		&m.CreatedAt,
		&m.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrMembershipNotFound
		}
		return nil, fmt.Errorf("repostiroy.GetMembership: %w", err)
	}

	m.Role = domain.Role(roleStr)
	m.Status = domain.MembershipStatus(statusStr)

	return &m, nil
}

// ListMembershipsByUserID возвращает все записи о домах
// к которым привязан пользователь
func (r *PostgresRepository) ListMembershipsByUserID(ctx context.Context, userID string) ([]domain.Membership, error) {
	query := `
		SELECT id, user_id, house_id, role, status, created_at, updated_at
		FROM memberships
		WHERE user_id = $1
		ORDER BY created_at ASC;
	`

	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("repository.ListMembershipsByUserID: %w", err)
	}
	defer rows.Close()

	var result []domain.Membership
	for rows.Next() {
		var m domain.Membership
		var roleStr, statusStr string

		if err := rows.Scan(
			&m.ID,
			&m.UserID,
			&m.HouseID,
			&roleStr,
			&statusStr,
			&m.CreatedAt,
			&m.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("repository.ListMembershipsByUserID scan: %w", err)
		}

		m.Role = domain.Role(roleStr)
		m.Status = domain.MembershipStatus(statusStr)
		result = append(result, m)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("repository.ListMembershipsByUserID rows: %w", err)
	}

	return result, nil
}

// ListHousesByIDs делает батч-выборку домов по списку UUID (для сборки UserContext)
func (r *PostgresRepository) ListHousesByIDs(ctx context.Context, houseIDs []string) ([]domain.House, error) {
	if len(houseIDs) == 0 {
		return []domain.House{}, nil
	}

	query := `
		SELECT id, name, address, city, created_at, updated_at
		FROM houses
		WHERE id = ANY($1);
	`

	rows, err := r.pool.Query(ctx, query, houseIDs)
	if err != nil {
		return nil, fmt.Errorf("repository.ListHousesByIDs: %w", err)
	}
	defer rows.Close()

	var houses []domain.House
	for rows.Next() {
		var h domain.House
		if err := rows.Scan(
			&h.ID,
			&h.Name,
			&h.Address,
			&h.City,
			&h.CreatedAt,
			&h.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("repository.ListHousesByIDs scan: %w", err)
		}
		houses = append(houses, h)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("repository.ListHouseByIDs rows: %w", err)
	}

	return houses, nil
}

// Ping проверяет доступность соединения с базой данных для /readyz
func (r *PostgresRepository) Ping(ctx context.Context) error {
	return r.pool.Ping(ctx)
}
