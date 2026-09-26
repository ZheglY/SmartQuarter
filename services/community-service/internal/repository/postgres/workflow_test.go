package postgres_test

import (
	"context"
	"errors"
	"github.com/ZheglY/SmartQuarter/services/community-service/internal/domain"
	"github.com/ZheglY/SmartQuarter/services/community-service/internal/repository/postgres"
	"github.com/ZheglY/SmartQuarter/services/community-service/internal/usecase"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestPostgresCommunityWorkflow(t *testing.T) {
	dsn := os.Getenv("COMMUNITY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("isolated community_test PostgreSQL required")
	}
	ctx := context.Background()
	db, e := pgxpool.New(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(db.Close)
	if db.Config().ConnConfig.Database != "community_test" {
		t.Fatal("refusing non-test database")
	}
	if os.Getenv("COMMUNITY_TEST_MIGRATE") == "1" {
		for _, name := range []string{"000001_init.up.sql", "000002_service_contacts.up.sql", "000003_community_integrity.up.sql"} {
			raw, e := os.ReadFile(filepath.Join("../../../migrations", name))
			if e != nil {
				t.Fatal(e)
			}
			if _, e = db.Exec(ctx, string(raw)); e != nil {
				t.Fatal(e)
			}
		}
	}
	repo := postgres.NewCommunityRepo(db)
	svc := usecase.NewCommunityService(repo)
	house, other, user := uuid.NewString(), uuid.NewString(), uuid.NewString()
	t.Cleanup(func() {
		for _, table := range []string{"polls", "calendar_events", "initiatives"} {
			_, err := db.Exec(ctx, "DELETE FROM "+table+" WHERE house_id=$1", house)
			if err != nil {
				t.Error(err)
			}
		}
	})
	for _, options := range [][]string{{"one"}, {"yes", " YES "}, {"yes", ""}} {
		if _, e := svc.CreatePoll(ctx, house, user, "Question", options, time.Now().Add(time.Hour)); !errors.Is(e, domain.ErrInvalidArgument) {
			t.Fatal("invalid options accepted", e)
		}
	}
	poll, e := svc.CreatePoll(ctx, house, user, "Question", []string{"Yes", "No"}, time.Now().Add(time.Hour))
	if e != nil {
		t.Fatal(e)
	}
	foreign, e := svc.CreatePoll(ctx, house, user, "Second", []string{"Yes", "No"}, time.Now().Add(time.Hour))
	if e != nil {
		t.Fatal(e)
	}
	if e = repo.VotePoll(ctx, house, poll.ID, foreign.Options[0].ID, user); !errors.Is(e, domain.ErrInvalidArgument) {
		t.Fatal("foreign option accepted", e)
	}
	if e = repo.VotePoll(ctx, other, poll.ID, poll.Options[0].ID, user); !errors.Is(e, domain.ErrNotFound) {
		t.Fatal("cross-house vote", e)
	}
	if _, e = db.Exec(ctx, `INSERT INTO poll_votes(poll_id,option_id,user_id) VALUES($1,$2,$3)`, poll.ID, foreign.Options[0].ID, user); e == nil {
		t.Fatal("composite FK absent")
	}
	var wg sync.WaitGroup
	results := make(chan error, 8)
	for range 8 {
		wg.Add(1)
		go func() { defer wg.Done(); results <- repo.VotePoll(ctx, house, poll.ID, poll.Options[0].ID, user) }()
	}
	wg.Wait()
	close(results)
	successful := 0
	for e := range results {
		if e == nil {
			successful++
		} else if !errors.Is(e, domain.ErrAlreadyVoted) {
			t.Fatal(e)
		}
	}
	if successful != 1 {
		t.Fatal("duplicate race", successful)
	}
	if _, e = db.Exec(ctx, `UPDATE polls SET ends_at=now()-interval '1 second' WHERE id=$1`, poll.ID); e != nil {
		t.Fatal(e)
	}
	if e = repo.VotePoll(ctx, house, poll.ID, poll.Options[0].ID, uuid.NewString()); !errors.Is(e, domain.ErrPollClosed) {
		t.Fatal("expired vote", e)
	}
	detail, e := svc.GetPoll(ctx, house, poll.ID, user)
	if e != nil || detail.Poll.Status != "CLOSED" || detail.TotalVotes != 1 {
		t.Fatal("expired details", detail, e)
	}
	closed, e := svc.ListPolls(ctx, house, []string{"CLOSED"}, 20, 0)
	if e != nil || len(closed) != 1 || len(closed[0].Options) != 2 {
		t.Fatal("expired filter", closed, e)
	}
	if _, e = svc.ClosePoll(ctx, house, foreign.ID, user); e != nil {
		t.Fatal(e)
	}
	if e = repo.VotePoll(ctx, house, foreign.ID, foreign.Options[0].ID, user); !errors.Is(e, domain.ErrPollClosed) {
		t.Fatal("closed vote", e)
	}
	start := time.Now().UTC()
	ev, e := svc.CreateCalendarEvent(ctx, house, user, "Work", "Details", start, start.Add(48*time.Hour))
	if e != nil {
		t.Fatal(e)
	}
	events, e := svc.ListCalendarEvents(ctx, house, start.Add(24*time.Hour), start.Add(72*time.Hour))
	if e != nil || len(events) != 1 {
		t.Fatal("overlap", events, e)
	}
	if _, e = svc.UpdateCalendarEvent(ctx, other, ev.ID, "Changed", "", start, start.Add(time.Hour)); !errors.Is(e, domain.ErrNotFound) {
		t.Fatal("cross-house edit", e)
	}
	if _, e = svc.UpdateCalendarEvent(ctx, house, ev.ID, "Changed", "", start, start); !errors.Is(e, domain.ErrInvalidDate) {
		t.Fatal("bad calendar date", e)
	}
	if e = svc.DeleteCalendarEvent(ctx, other, ev.ID); !errors.Is(e, domain.ErrNotFound) {
		t.Fatal("cross-house delete", e)
	}
	first, e := svc.CreateInitiative(ctx, house, user, "First", "Details")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = svc.CreateInitiative(ctx, house, user, "Second", "Details"); e != nil {
		t.Fatal(e)
	}
	if _, e = svc.SupportInitiative(ctx, other, first.ID, user); !errors.Is(e, domain.ErrNotFound) {
		t.Fatal("cross-house support", e)
	}
	if count, e := svc.SupportInitiative(ctx, house, first.ID, user); e != nil || count != 1 {
		t.Fatal("older initiative count", count, e)
	}
	if _, e = svc.SupportInitiative(ctx, house, first.ID, user); !errors.Is(e, domain.ErrAlreadyVoted) {
		t.Fatal("duplicate support", e)
	}
	if _, e = svc.CloseInitiative(ctx, house, first.ID, user); e != nil {
		t.Fatal(e)
	}
	if _, e = svc.SupportInitiative(ctx, house, first.ID, uuid.NewString()); !errors.Is(e, domain.ErrInitiativeClosed) {
		t.Fatal("closed support", e)
	}
}
