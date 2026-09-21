package postgres

import (
	"context"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/domain"
)

func (q *queries) LatestStatement(ctx context.Context, issueID string) (domain.StatementDraft, error) {
	var s domain.StatementDraft
	err := q.db.QueryRow(ctx, "SELECT id::text,issue_id::text,version,status,body,chairman_note,source_snapshot,created_by::text,created_at,updated_at FROM statement_drafts WHERE issue_id=$1 ORDER BY version DESC LIMIT 1", issueID).Scan(&s.ID, &s.IssueID, &s.Version, &s.Status, &s.Body, &s.ChairmanNote, &s.SourceSnapshot, &s.CreatedBy, &s.CreatedAt, &s.UpdatedAt)
	return s, translate(err)
}
func (q *queries) InsertStatement(ctx context.Context, s domain.StatementDraft) error {
	_, err := q.db.Exec(ctx, "INSERT INTO statement_drafts(id,issue_id,version,status,body,chairman_note,source_snapshot,created_by,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)", s.ID, s.IssueID, s.Version, s.Status, s.Body, s.ChairmanNote, s.SourceSnapshot, s.CreatedBy, s.CreatedAt, s.UpdatedAt)
	return translate(err)
}
