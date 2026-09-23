package postgres

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ZheglY/SmartQuarter/services/community-service/internal/domain"
)

type CommunityRepo struct {
	db *pgxpool.Pool
}

func NewCommunityRepo(db *pgxpool.Pool) *CommunityRepo {
	return &CommunityRepo{db: db}
}

func (r *CommunityRepo) insertOutboxEvent(ctx context.Context, tx pgx.Tx, eventType string, payload map[string]any) error {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	query := `INSERT INTO outbox_events (event_id, event_type, producer, payload) VALUES ($1, $2, $3, $4)`
	_, err = tx.Exec(ctx, query, uuid.New().String(), eventType, "community-service", payloadBytes)
	return err
}

func (r *CommunityRepo) CreateAnnouncement(ctx context.Context, a *domain.Announcement) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	query := `
		INSERT INTO announcements (id, house_id, author_user_id, title, body, status, published_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
	_, err = tx.Exec(ctx, query, a.ID, a.HouseID, a.AuthorUserID, a.Title, a.Body, a.Status, a.PublishedAt, a.CreatedAt)
	if err != nil {
		return err
	}

	err = r.insertOutboxEvent(ctx, tx, "announcement.created", map[string]any{
		"announcement_id": a.ID,
		"house_id":        a.HouseID,
		"author_user_id":  a.AuthorUserID,
	})
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *CommunityRepo) ListAnnouncements(ctx context.Context, houseID string, limit, offset int) ([]domain.Announcement, error) {
	query := `
		SELECT id, house_id, author_user_id, title, body, status, published_at, created_at
		FROM announcements WHERE house_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`
	rows, err := r.db.Query(ctx, query, houseID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []domain.Announcement
	for rows.Next() {
		var a domain.Announcement
		if err := rows.Scan(&a.ID, &a.HouseID, &a.AuthorUserID, &a.Title, &a.Body, &a.Status, &a.PublishedAt, &a.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, a)
	}
	return items, nil
}

func (r *CommunityRepo) CreatePoll(ctx context.Context, p *domain.Poll) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	queryPoll := `INSERT INTO polls (id, house_id, author_user_id, question, status, ends_at, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7)`
	_, err = tx.Exec(ctx, queryPoll, p.ID, p.HouseID, p.AuthorUserID, p.Question, p.Status, p.EndsAt, p.CreatedAt)
	if err != nil {
		return err
	}

	queryOption := `INSERT INTO poll_options (id, poll_id, text, position) VALUES ($1, $2, $3, $4)`
	for _, opt := range p.Options {
		_, err = tx.Exec(ctx, queryOption, opt.ID, p.ID, opt.Text, opt.Position)
		if err != nil {
			return err
		}
	}

	err = r.insertOutboxEvent(ctx, tx, "poll.created", map[string]any{
		"poll_id":        p.ID,
		"house_id":       p.HouseID,
		"author_user_id": p.AuthorUserID,
	})
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *CommunityRepo) GetPoll(ctx context.Context, houseID, pollID, userID string) (*domain.PollDetails, error) {
	queryPoll := `SELECT id, house_id, author_user_id, question, status, ends_at, created_at FROM polls WHERE id = $1 AND house_id = $2`
	var p domain.Poll
	err := r.db.QueryRow(ctx, queryPoll, pollID, houseID).Scan(&p.ID, &p.HouseID, &p.AuthorUserID, &p.Question, &p.Status, &p.EndsAt, &p.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}

	queryOpts := `
		SELECT o.id, o.text, o.position, COUNT(v.user_id) as votes
		FROM poll_options o
		LEFT JOIN poll_votes v ON o.id = v.option_id
		WHERE o.poll_id = $1
		GROUP BY o.id, o.text, o.position
		ORDER BY o.position`

	rows, err := r.db.Query(ctx, queryOpts, pollID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var details domain.PollDetails
	details.Poll = p

	for rows.Next() {
		var opt domain.PollOption
		var res domain.PollOptionResult
		if err := rows.Scan(&opt.ID, &opt.Text, &opt.Position, &res.VotesCount); err != nil {
			return nil, err
		}
		res.OptionID = opt.ID
		details.Poll.Options = append(details.Poll.Options, opt)
		details.Results = append(details.Results, res)
		details.TotalVotes += res.VotesCount
	}

	queryMyVote := `SELECT option_id FROM poll_votes WHERE poll_id = $1 AND user_id = $2`
	var myOpt string
	err = r.db.QueryRow(ctx, queryMyVote, pollID, userID).Scan(&myOpt)
	if err == nil {
		details.MyOptionID = myOpt
	}

	return &details, nil
}

func (r *CommunityRepo) ListPolls(ctx context.Context, houseID string, statuses []string, limit, offset int) ([]domain.Poll, error) {
	query := `SELECT id, house_id, author_user_id, question, status, ends_at, created_at FROM polls WHERE house_id = $1`

	if len(statuses) > 0 {
		query += ` AND status = ANY($4) ORDER BY created_at DESC LIMIT $2 OFFSET $3`
		rows, err := r.db.Query(ctx, query, houseID, limit, offset, statuses)
		return r.scanPolls(rows, err)
	}

	query += ` ORDER BY created_at DESC LIMIT $2 OFFSET $3`
	rows, err := r.db.Query(ctx, query, houseID, limit, offset)
	return r.scanPolls(rows, err)
}

func (r *CommunityRepo) scanPolls(rows pgx.Rows, err error) ([]domain.Poll, error) {
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []domain.Poll
	for rows.Next() {
		var p domain.Poll
		if err := rows.Scan(&p.ID, &p.HouseID, &p.AuthorUserID, &p.Question, &p.Status, &p.EndsAt, &p.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, p)
	}
	return items, nil
}

func (r *CommunityRepo) VotePoll(ctx context.Context, houseID, pollID, optionID, userID string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	query := `INSERT INTO poll_votes (poll_id, option_id, user_id) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`
	res, err := tx.Exec(ctx, query, pollID, optionID, userID)
	if err != nil {
		return err
	}

	if res.RowsAffected() == 0 {
		return domain.ErrAlreadyVoted
	}

	err = r.insertOutboxEvent(ctx, tx, "poll.voted", map[string]any{
		"poll_id":   pollID,
		"house_id":  houseID,
		"option_id": optionID,
	})
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *CommunityRepo) CreateCalendarEvent(ctx context.Context, e *domain.CalendarEvent) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	query := `INSERT INTO calendar_events (id, house_id, created_by, title, description, starts_at, ends_at, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
	_, err = tx.Exec(ctx, query, e.ID, e.HouseID, e.CreatedBy, e.Title, e.Description, e.StartsAt, e.EndsAt, e.CreatedAt)
	if err != nil {
		return err
	}

	err = r.insertOutboxEvent(ctx, tx, "calendar.event_created", map[string]any{
		"event_id":  e.ID,
		"house_id":  e.HouseID,
		"starts_at": e.StartsAt.Format(time.RFC3339),
	})
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *CommunityRepo) ListCalendarEvents(ctx context.Context, houseID string, from, to time.Time) ([]domain.CalendarEvent, error) {
	query := `SELECT id, house_id, created_by, title, description, starts_at, ends_at, created_at FROM calendar_events WHERE house_id = $1 AND starts_at >= $2 AND starts_at < $3 ORDER BY starts_at ASC`
	rows, err := r.db.Query(ctx, query, houseID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []domain.CalendarEvent
	for rows.Next() {
		var e domain.CalendarEvent
		if err := rows.Scan(&e.ID, &e.HouseID, &e.CreatedBy, &e.Title, &e.Description, &e.StartsAt, &e.EndsAt, &e.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, e)
	}
	return items, nil
}

func (r *CommunityRepo) CreateInitiative(ctx context.Context, i *domain.Initiative) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	query := `INSERT INTO initiatives (id, house_id, author_user_id, title, description, status, supports_count, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`
	_, err = tx.Exec(ctx, query, i.ID, i.HouseID, i.AuthorUserID, i.Title, i.Description, i.Status, i.SupportsCount, i.CreatedAt, i.UpdatedAt)
	if err != nil {
		return err
	}

	err = r.insertOutboxEvent(ctx, tx, "initiative.created", map[string]any{
		"initiative_id":  i.ID,
		"house_id":       i.HouseID,
		"author_user_id": i.AuthorUserID,
	})
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *CommunityRepo) ListInitiatives(ctx context.Context, houseID, userID string, limit, offset int) ([]domain.Initiative, error) {
	query := `
		SELECT i.id, i.house_id, i.author_user_id, i.title, i.description, i.status, i.supports_count, i.created_at, i.updated_at,
		       EXISTS(SELECT 1 FROM initiative_supports s WHERE s.initiative_id = i.id AND s.user_id = $4) as supported
		FROM initiatives i
		WHERE i.house_id = $1
		ORDER BY i.created_at DESC LIMIT $2 OFFSET $3`

	rows, err := r.db.Query(ctx, query, houseID, limit, offset, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []domain.Initiative
	for rows.Next() {
		var i domain.Initiative
		if err := rows.Scan(&i.ID, &i.HouseID, &i.AuthorUserID, &i.Title, &i.Description, &i.Status, &i.SupportsCount, &i.CreatedAt, &i.UpdatedAt, &i.SupportedByMe); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, nil
}

func (r *CommunityRepo) SupportInitiative(ctx context.Context, houseID, initiativeID, userID string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	querySupport := `INSERT INTO initiative_supports (initiative_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`
	res, err := tx.Exec(ctx, querySupport, initiativeID, userID)
	if err != nil {
		return err
	}

	if res.RowsAffected() == 0 {
		return domain.ErrAlreadyVoted
	}

	queryUpdate := `UPDATE initiatives SET supports_count = supports_count + 1 WHERE id = $1`
	if _, err := tx.Exec(ctx, queryUpdate, initiativeID); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *CommunityRepo) GetUnpublishedOutboxEvents(ctx context.Context, limit int) ([]domain.OutboxEvent, error) {
	query := `
		SELECT event_id, event_type, payload, occurred_at
		FROM outbox_events
		WHERE published_at IS NULL
		ORDER BY occurred_at ASC
		LIMIT $1
		FOR UPDATE SKIP LOCKED`
	rows, err := r.db.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []domain.OutboxEvent
	for rows.Next() {
		var e domain.OutboxEvent
		if err := rows.Scan(&e.EventID, &e.EventType, &e.Payload, &e.OccurredAt); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, nil
}

func (r *CommunityRepo) MarkOutboxEventsPublished(ctx context.Context, eventIDs []string) error {
	if len(eventIDs) == 0 {
		return nil
	}
	query := `UPDATE outbox_events SET published_at = NOW() WHERE event_id = ANY($1)`
	_, err := r.db.Exec(ctx, query, eventIDs)
	return err
}
