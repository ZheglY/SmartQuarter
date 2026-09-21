package postgres

import (
	"context"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/domain"
)

const attachmentColumns = "id::text,house_id::text,COALESCE(issue_id::text,''),uploaded_by::text,object_key,original_filename,mime_type,size_bytes,COALESCE(sha256,''),COALESCE(etag,''),status,upload_expires_at,created_at,updated_at"

func scanAttachment(row scanner) (domain.Attachment, error) {
	var a domain.Attachment
	err := row.Scan(&a.ID, &a.HouseID, &a.IssueID, &a.UploadedBy, &a.ObjectKey, &a.OriginalFilename, &a.MIMEType, &a.SizeBytes, &a.SHA256, &a.ETag, &a.Status, &a.UploadExpiresAt, &a.CreatedAt, &a.UpdatedAt)
	return a, translate(err)
}
func (q *queries) Attachment(ctx context.Context, id string, lock bool) (domain.Attachment, error) {
	sql := "SELECT " + attachmentColumns + " FROM attachments WHERE id=$1"
	if lock {
		sql += " FOR UPDATE"
	}
	return scanAttachment(q.db.QueryRow(ctx, sql, id))
}
func (q *queries) InsertAttachment(ctx context.Context, a domain.Attachment) error {
	_, err := q.db.Exec(ctx, "INSERT INTO attachments(id,house_id,uploaded_by,object_key,original_filename,mime_type,size_bytes,status,upload_expires_at,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)", a.ID, a.HouseID, a.UploadedBy, a.ObjectKey, a.OriginalFilename, a.MIMEType, a.SizeBytes, a.Status, a.UploadExpiresAt, a.CreatedAt, a.UpdatedAt)
	return translate(err)
}
func (q *queries) UpdateAttachment(ctx context.Context, a domain.Attachment) error {
	tag, err := q.db.Exec(ctx, "UPDATE attachments SET issue_id=NULLIF($2,'')::uuid,sha256=NULLIF($3,''),etag=NULLIF($4,''),status=$5,updated_at=$6 WHERE id=$1", a.ID, a.IssueID, a.SHA256, a.ETag, a.Status, a.UpdatedAt)
	if err != nil {
		return translate(err)
	}
	if tag.RowsAffected() != 1 {
		return domain.ErrNotFound
	}
	return nil
}
func (q *queries) Attachments(ctx context.Context, issueID string) ([]domain.Attachment, error) {
	rows, err := q.db.Query(ctx, "SELECT "+attachmentColumns+" FROM attachments WHERE issue_id=$1 ORDER BY created_at,id", issueID)
	if err != nil {
		return nil, translate(err)
	}
	defer rows.Close()
	items := make([]domain.Attachment, 0)
	for rows.Next() {
		a, e := scanAttachment(rows)
		if e != nil {
			return nil, e
		}
		items = append(items, a)
	}
	return items, translate(rows.Err())
}
