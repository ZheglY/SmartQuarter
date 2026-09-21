//go:build integration

package integration

import (
	"context"
	"errors"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/domain"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/repository"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/repository/postgres"
	"github.com/ZheglY/SmartQuarter/services/issue-service/migrations"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func database(t *testing.T) (*pgxpool.Pool, *postgres.Store) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	u, err := url.Parse(dsn)
	if err != nil || u.Path != "/issue_test" {
		t.Fatal("set TEST_DATABASE_URL to the dedicated issue_test database; other databases are refused")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal("cannot open test database")
	}
	t.Cleanup(pool.Close)
	if err = pool.Ping(context.Background()); err != nil {
		t.Fatal("test PostgreSQL unavailable")
	}
	if err = migrations.Run(dsn, "up"); err != nil {
		t.Fatal(err)
	}
	return pool, postgres.New(pool)
}
func newIssue(house, author string) domain.Issue {
	now := time.Now().UTC().Truncate(time.Microsecond)
	return domain.Issue{ID: uuid.NewString(), HouseID: house, CreatedBy: author, HouseAddressSnapshot: "Тестовый дом 1", Category: domain.Safety, Description: "Открытый люк", Status: domain.Detected, CreatedAt: now, UpdatedAt: now}
}
func TestPostgres(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	u, _ := url.Parse(dsn)
	if u == nil || u.Path != "/issue_test" {
		t.Fatal("dedicated TEST_DATABASE_URL required")
	}
	for _, direction := range []string{"up", "down", "up"} {
		if err := migrations.Run(dsn, direction); err != nil {
			t.Fatalf("migration %s: %v", direction, err)
		}
	}
	pool, store := database(t)
	ctx := context.Background()
	house, author := uuid.NewString(), uuid.NewString()
	issue := newIssue(house, author)
	rollback := errors.New("deliberate rollback")
	err := store.WithinTx(ctx, func(q repository.Queries) error {
		if err := q.InsertIssue(ctx, issue); err != nil {
			return err
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatal(err)
	}
	if _, err = store.Read().Issue(ctx, issue.ID, false); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("rolled back issue exists")
	}
	if err = store.WithinTx(ctx, func(q repository.Queries) error { return q.InsertIssue(ctx, issue) }); err != nil {
		t.Fatal(err)
	}
	found, err := store.Read().Issue(ctx, issue.ID, false)
	if err != nil || found.HouseID != house {
		t.Fatal("issue roundtrip failed", err)
	}

	t.Run("concurrent confirmations and duplicate", func(t *testing.T) {
		user := uuid.NewString()
		results := make(chan error, 2)
		var wg sync.WaitGroup
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				results <- store.WithinTx(ctx, func(q repository.Queries) error {
					if _, err := q.Issue(ctx, issue.ID, true); err != nil {
						return err
					}
					_, err := q.AddConfirmation(ctx, domain.Confirmation{IssueID: issue.ID, UserID: user, CreatedAt: time.Now()})
					return err
				})
			}()
		}
		wg.Wait()
		close(results)
		success, duplicate := 0, 0
		for err := range results {
			if err == nil {
				success++
			} else if errors.Is(err, domain.ErrExists) {
				duplicate++
			} else {
				t.Fatal(err)
			}
		}
		if success != 1 || duplicate != 1 {
			t.Fatalf("success=%d duplicate=%d", success, duplicate)
		}
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				e := store.WithinTx(ctx, func(q repository.Queries) error {
					_, e := q.AddConfirmation(ctx, domain.Confirmation{IssueID: issue.ID, UserID: uuid.NewString(), CreatedAt: time.Now()})
					return e
				})
				if e != nil {
					t.Error(e)
				}
			}()
		}
		wg.Wait()
		item, e := store.Read().Issue(ctx, issue.ID, false)
		if e != nil || item.ConfirmationsCount != 3 {
			t.Fatalf("counter=%d, %v", item.ConfirmationsCount, e)
		}
	})
	t.Run("cursor tie break and isolation", func(t *testing.T) {
		tie := newIssue(house, author)
		tie.CreatedAt = issue.CreatedAt
		tie.UpdatedAt = issue.UpdatedAt
		if err := store.Read().InsertIssue(ctx, tie); err != nil {
			t.Fatal(err)
		}
		first, e := store.Read().ListIssues(ctx, domain.ListFilter{HouseID: house, Limit: 1})
		if e != nil || len(first) != 1 {
			t.Fatal(e)
		}
		second, e := store.Read().ListIssues(ctx, domain.ListFilter{HouseID: house, Limit: 1, BeforeTime: &first[0].CreatedAt, BeforeID: first[0].ID})
		if e != nil || len(second) != 1 || second[0].ID == first[0].ID {
			t.Fatal("cursor failure", e)
		}
		none, e := store.Read().ListIssues(ctx, domain.ListFilter{HouseID: uuid.NewString(), Limit: 10})
		if e != nil || len(none) != 0 {
			t.Fatal("house leaked")
		}
	})
	t.Run("outbox skip locked and retry", func(t *testing.T) {
		event := domain.Event{ID: uuid.NewString(), AggregateType: "issue", AggregateID: issue.ID, Type: "issue.created", Version: 1, Payload: []byte("{}"), OccurredAt: time.Now()}
		if err := store.Read().AddOutbox(ctx, event); err != nil {
			t.Fatal(err)
		}
		lock, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer lock.Rollback(ctx)
		if _, err = lock.Exec(ctx, "SELECT event_id FROM outbox_events WHERE event_id=$1 FOR UPDATE", event.ID); err != nil {
			t.Fatal(err)
		}
		n, err := store.PublishBatch(ctx, 100, func(context.Context, domain.Event) error { return nil })
		if err != nil || n != 0 {
			t.Fatal("locked event was consumed", err)
		}
		if err = lock.Rollback(ctx); err != nil {
			t.Fatal(err)
		}
		n, err = store.PublishBatch(ctx, 100, func(context.Context, domain.Event) error { return errors.New("Redis offline") })
		if err != nil || n != 0 {
			t.Fatal(err)
		}
		var attempts int
		var published *time.Time
		if err = pool.QueryRow(ctx, "SELECT attempts,published_at FROM outbox_events WHERE event_id=$1", event.ID).Scan(&attempts, &published); err != nil || attempts != 1 || published != nil {
			t.Fatal("retry state not saved", err)
		}
		if _, err = pool.Exec(ctx, "UPDATE outbox_events SET next_attempt_at=NOW() WHERE event_id=$1", event.ID); err != nil {
			t.Fatal(err)
		}
		n, err = store.PublishBatch(ctx, 100, func(context.Context, domain.Event) error { return nil })
		if err != nil || n != 1 {
			t.Fatal("recovery failed", err)
		}
	})
}
