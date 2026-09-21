package postgres

import (
	"context"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/domain"
	"github.com/google/uuid"
	"time"
)

const issueColumns = "id::text,house_id::text,created_by::text,house_address_snapshot,category,description,location_text,status,confirmations_count,created_at,updated_at,resolved_at"

func scanIssue(row scanner) (domain.Issue, error) {
	var i domain.Issue
	err := row.Scan(&i.ID, &i.HouseID, &i.CreatedBy, &i.HouseAddressSnapshot, &i.Category, &i.Description, &i.LocationText, &i.Status, &i.ConfirmationsCount, &i.CreatedAt, &i.UpdatedAt, &i.ResolvedAt)
	return i, translate(err)
}
func (q *queries) Issue(ctx context.Context, id string, lock bool) (domain.Issue, error) {
	sql := "SELECT " + issueColumns + " FROM issues WHERE id=$1"
	if lock {
		sql += " FOR UPDATE"
	}
	return scanIssue(q.db.QueryRow(ctx, sql, id))
}
func (q *queries) InsertIssue(ctx context.Context, i domain.Issue) error {
	_, err := q.db.Exec(ctx, "INSERT INTO issues(id,house_id,created_by,house_address_snapshot,category,description,location_text,status,confirmations_count,created_at,updated_at,resolved_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)", i.ID, i.HouseID, i.CreatedBy, i.HouseAddressSnapshot, i.Category, i.Description, i.LocationText, i.Status, i.ConfirmationsCount, i.CreatedAt, i.UpdatedAt, i.ResolvedAt)
	return translate(err)
}
func (q *queries) ChangeStatus(ctx context.Context, i domain.Issue) error {
	tag, err := q.db.Exec(ctx, "UPDATE issues SET status=$2,updated_at=$3,resolved_at=$4 WHERE id=$1", i.ID, i.Status, i.UpdatedAt, i.ResolvedAt)
	if err != nil {
		return translate(err)
	}
	if tag.RowsAffected() != 1 {
		return domain.ErrNotFound
	}
	return nil
}
func (q *queries) ListIssues(ctx context.Context, f domain.ListFilter) ([]domain.Issue, error) {
	var statuses []string
	for _, s := range f.Statuses {
		statuses = append(statuses, string(s))
	}
	beforeID := f.BeforeID
	if beforeID == "" {
		beforeID = uuid.Nil.String()
	}
	var before *time.Time = f.BeforeTime
	rows, err := q.db.Query(ctx, "SELECT "+issueColumns+" FROM issues WHERE house_id=$1 AND ($2::text[] IS NULL OR status=ANY($2)) AND ($3::timestamptz IS NULL OR (created_at,id)<($3,$4::uuid)) AND (NOT $5::boolean OR status<>'RESOLVED') ORDER BY created_at DESC,id DESC LIMIT $6", f.HouseID, statuses, before, beforeID, f.ChairmanQueue, f.Limit)
	if err != nil {
		return nil, translate(err)
	}
	defer rows.Close()
	items := make([]domain.Issue, 0)
	for rows.Next() {
		i, e := scanIssue(rows)
		if e != nil {
			return nil, e
		}
		items = append(items, i)
	}
	return items, translate(rows.Err())
}
