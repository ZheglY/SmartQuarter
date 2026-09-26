package postgres

import (
	"context"
	"github.com/ZheglY/SmartQuarter/services/community-service/internal/domain"
	"github.com/jackc/pgx/v5"
)

func (r *CommunityRepo) ClosePoll(ctx context.Context, house, id string) error {
	result, e := r.db.Exec(ctx, `UPDATE polls SET status='CLOSED' WHERE id=$1 AND house_id=$2`, id, house)
	if e == nil && result.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return e
}
func (r *CommunityRepo) UpdateCalendarEvent(ctx context.Context, v *domain.CalendarEvent) error {
	e := r.db.QueryRow(ctx, `UPDATE calendar_events SET title=$3,description=$4,starts_at=$5,ends_at=$6 WHERE id=$1 AND house_id=$2 RETURNING created_by,created_at`, v.ID, v.HouseID, v.Title, v.Description, v.StartsAt, v.EndsAt).Scan(&v.CreatedBy, &v.CreatedAt)
	if e == pgx.ErrNoRows {
		return domain.ErrNotFound
	}
	return e
}
func (r *CommunityRepo) DeleteCalendarEvent(ctx context.Context, house, id string) error {
	result, e := r.db.Exec(ctx, `DELETE FROM calendar_events WHERE id=$1 AND house_id=$2`, id, house)
	if e == nil && result.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return e
}
func (r *CommunityRepo) GetInitiative(ctx context.Context, house, id, user string) (*domain.Initiative, error) {
	v := new(domain.Initiative)
	e := r.db.QueryRow(ctx, `SELECT i.id,i.house_id,i.author_user_id,i.title,i.description,i.status,i.supports_count,i.created_at,i.updated_at,EXISTS(SELECT 1 FROM initiative_supports s WHERE s.initiative_id=i.id AND s.user_id=$3) FROM initiatives i WHERE i.id=$1 AND i.house_id=$2`, id, house, user).Scan(&v.ID, &v.HouseID, &v.AuthorUserID, &v.Title, &v.Description, &v.Status, &v.SupportsCount, &v.CreatedAt, &v.UpdatedAt, &v.SupportedByMe)
	if e == pgx.ErrNoRows {
		return nil, domain.ErrNotFound
	}
	return v, e
}
func (r *CommunityRepo) CloseInitiative(ctx context.Context, house, id string) error {
	result, e := r.db.Exec(ctx, `UPDATE initiatives SET status='CLOSED',updated_at=now() WHERE id=$1 AND house_id=$2`, id, house)
	if e == nil && result.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return e
}
