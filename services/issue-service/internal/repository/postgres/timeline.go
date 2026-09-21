package postgres

import (
	"context"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/domain"
)

func (q *queries) AddTimeline(ctx context.Context, e domain.TimelineEvent) error {
	_, err := q.db.Exec(ctx, "INSERT INTO timeline_events(id,issue_id,type,actor_user_id,payload,created_at) VALUES($1,$2,$3,NULLIF($4,'')::uuid,$5,$6)", e.ID, e.IssueID, e.Type, e.ActorUserID, e.Payload, e.CreatedAt)
	return translate(err)
}
func (q *queries) Timeline(ctx context.Context, issueID string) ([]domain.TimelineEvent, error) {
	rows, err := q.db.Query(ctx, "SELECT id::text,issue_id::text,type,COALESCE(actor_user_id::text,''),payload,created_at FROM timeline_events WHERE issue_id=$1 ORDER BY created_at,id", issueID)
	if err != nil {
		return nil, translate(err)
	}
	defer rows.Close()
	items := make([]domain.TimelineEvent, 0)
	for rows.Next() {
		var e domain.TimelineEvent
		if err = rows.Scan(&e.ID, &e.IssueID, &e.Type, &e.ActorUserID, &e.Payload, &e.CreatedAt); err != nil {
			return nil, translate(err)
		}
		items = append(items, e)
	}
	return items, translate(rows.Err())
}
