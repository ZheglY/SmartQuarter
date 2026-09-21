package postgres

import (
	"context"
	"errors"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/domain"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/repository"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

type database interface {
	Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error)
	Query(context.Context, string, ...interface{}) (pgx.Rows, error)
	QueryRow(context.Context, string, ...interface{}) pgx.Row
}
type queries struct{ db database }
type Store struct{ Pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Store       { return &Store{Pool: pool} }
func (s *Store) Read() repository.Queries { return &queries{db: s.Pool} }
func (s *Store) WithinTx(ctx context.Context, fn func(repository.Queries) error) error {
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return translate(err)
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = tx.Rollback(cleanup)
	}()
	if err = fn(&queries{db: tx}); err != nil {
		return err
	}
	return translate(tx.Commit(ctx))
}
func translate(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return domain.ErrExists
		case "23503":
			return domain.ErrPrecondition
		case "23514", "22001", "22P02":
			return domain.ErrInvalid
		}
	}
	return domain.ErrUnavailable
}

type scanner interface{ Scan(...interface{}) error }
