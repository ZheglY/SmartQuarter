package usecase

import (
	"bytes"
	"context"
	"errors"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/domain"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/storage"
	"github.com/google/uuid"
	"testing"
	"time"
)

type changedDuringVerification struct {
	*fakeObjects
	beforeHead func()
}

func (o *changedDuringVerification) HeadObject(ctx context.Context, key string) (storage.ObjectInfo, error) {
	o.beforeHead()
	return o.fakeObjects.HeadObject(ctx, key)
}
func TestCompleteUploadRechecksAfterVerification(t *testing.T) {
	for _, tc := range []struct {
		name        string
		change      func(*domain.Attachment, *time.Time)
		invalidFile bool
		want        error
		status      domain.AttachmentStatus
		deleted     bool
	}{
		{"concurrent ready wins over stale rejection", func(a *domain.Attachment, _ *time.Time) {
			a.Status = domain.Ready
			a.SHA256 = "winner"
			a.ETag = "winner"
		}, true, nil, domain.Ready, false},
		{"concurrent ready is idempotent", func(a *domain.Attachment, _ *time.Time) {
			a.Status = domain.Ready
			a.SHA256 = "winner"
			a.ETag = "winner"
		}, false, nil, domain.Ready, false},
		{"attached never deleted", func(a *domain.Attachment, _ *time.Time) { a.Status = domain.Attached; a.IssueID = uuid.NewString() }, true, domain.ErrPrecondition, domain.Attached, false},
		{"owner rechecked", func(a *domain.Attachment, _ *time.Time) { a.UploadedBy = uuid.NewString() }, false, domain.ErrPermission, domain.Uploading, false},
		{"house rechecked", func(a *domain.Attachment, _ *time.Time) { a.HouseID = uuid.NewString() }, false, domain.ErrPermission, domain.Uploading, false},
		{"key rechecked", func(a *domain.Attachment, _ *time.Time) { a.ObjectKey = "different" }, false, domain.ErrPrecondition, domain.Uploading, false},
		{"size rechecked", func(a *domain.Attachment, _ *time.Time) { a.SizeBytes++ }, false, domain.ErrPrecondition, domain.Uploading, false},
		{"mime rechecked", func(a *domain.Attachment, _ *time.Time) { a.MIMEType = "image/jpeg" }, false, domain.ErrPrecondition, domain.Uploading, false},
		{"expiry contract rechecked", func(a *domain.Attachment, _ *time.Time) { a.UploadExpiresAt = a.UploadExpiresAt.Add(time.Hour) }, false, domain.ErrPrecondition, domain.Uploading, false},
		{"expires while reading", func(_ *domain.Attachment, now *time.Time) { *now = now.Add(2 * time.Minute) }, false, domain.ErrPrecondition, domain.Expired, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			actor := actor()
			now := time.Now().UTC()
			data := pngBytes()
			q := &uploadQueries{attachment: domain.Attachment{ID: uuid.NewString(), HouseID: actor.HouseID, UploadedBy: actor.UserID, ObjectKey: "object", MIMEType: "image/png", SizeBytes: int64(len(data)), Status: domain.Uploading, UploadExpiresAt: now.Add(time.Minute)}}
			raw := &fakeObjects{data: data, mime: "image/png"}
			if tc.invalidFile {
				raw.data = bytes.Repeat([]byte("x"), len(data))
			}
			objects := &changedDuringVerification{raw, func() { tc.change(&q.attachment, &now) }}
			service := New(&fakeStore{Queries: q}, objects, Options{Now: func() time.Time { return now }})
			result, err := service.CompleteUpload(context.Background(), actor, actor.HouseID, q.attachment.ID)
			if !errors.Is(err, tc.want) || q.attachment.Status != tc.status || raw.deleted != tc.deleted {
				t.Fatalf("err=%v status=%s deleted=%v", err, q.attachment.Status, raw.deleted)
			}
			if tc.status == domain.Ready && result.SHA256 != "winner" {
				t.Fatal("overwrote the concurrent successful verification")
			}
		})
	}
}
func TestCompleteUploadRollbackDoesNotDelete(t *testing.T) {
	actor := actor()
	data := bytes.Repeat([]byte("x"), 64)
	q := &uploadQueries{attachment: domain.Attachment{ID: uuid.NewString(), HouseID: actor.HouseID, UploadedBy: actor.UserID, ObjectKey: "object", MIMEType: "image/png", SizeBytes: int64(len(data)), Status: domain.Uploading, UploadExpiresAt: time.Now().Add(time.Minute)}}
	objects := &fakeObjects{data: data, mime: "image/png"}
	service := New(&fakeStore{Queries: q, transactionError: domain.ErrUnavailable}, objects, Options{})
	_, err := service.CompleteUpload(context.Background(), actor, actor.HouseID, q.attachment.ID)
	if !errors.Is(err, domain.ErrUnavailable) || objects.deleted || q.attachment.Status != domain.Uploading {
		t.Fatal("cleanup ran without committed rejection", err)
	}
}
