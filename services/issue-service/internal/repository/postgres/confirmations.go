package postgres

import (
	"context"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/domain"
)

func (q *queries) Confirmed(ctx context.Context, issueID, userID string) (bool, error) {
	var yes bool
	err := q.db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM confirmations WHERE issue_id=$1 AND user_id=$2)", issueID, userID).Scan(&yes)
	return yes, translate(err)
}
func (q *queries) AddConfirmation(ctx context.Context, c domain.Confirmation) (int, error) {
	_, err := q.db.Exec(ctx, "INSERT INTO confirmations(issue_id,user_id,created_at) VALUES($1,$2,$3)", c.IssueID, c.UserID, c.CreatedAt)
	if err != nil {
		return 0, translate(err)
	}
	var count int
	err = q.db.QueryRow(ctx, "UPDATE issues SET confirmations_count=confirmations_count+1,updated_at=$2 WHERE id=$1 RETURNING confirmations_count", c.IssueID, c.CreatedAt).Scan(&count)
	return count, translate(err)
}
