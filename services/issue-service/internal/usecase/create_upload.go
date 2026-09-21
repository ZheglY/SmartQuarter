package usecase

import (
	"context"
	"fmt"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/domain"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/repository"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/storage"
	"github.com/google/uuid"
)

type CreateUploadInput struct {
	HouseID, Filename, MIMEType string
	SizeBytes                   int64
}
type Upload struct {
	ID     string
	Signed storage.SignedURL
}

func (s *Service) CreateUpload(ctx context.Context, a domain.Actor, in CreateUploadInput) (result Upload, err error) {
	if err = validate(ctx, a, in.HouseID); err != nil {
		return result, err
	}
	if !validFilename(in.Filename) || (in.MIMEType != "image/png" && in.MIMEType != "image/jpeg") || in.SizeBytes <= 0 || in.SizeBytes > s.options.MaxUploadSize {
		return result, domain.ErrInvalid
	}
	now := s.now()
	id := uuid.NewString()
	attachment := domain.Attachment{ID: id, HouseID: a.HouseID, UploadedBy: a.UserID, ObjectKey: fmt.Sprintf("houses/%s/attachments/%s/original", a.HouseID, id), OriginalFilename: in.Filename, MIMEType: in.MIMEType, SizeBytes: in.SizeBytes, Status: domain.Uploading, UploadExpiresAt: now.Add(s.options.UploadTTL), CreatedAt: now, UpdatedAt: now}
	err = s.store.WithinTx(ctx, func(q repository.Queries) error {
		if err := q.InsertAttachment(ctx, attachment); err != nil {
			return err
		}
		signed, err := s.objects.PresignUpload(ctx, attachment.ObjectKey, attachment.MIMEType, attachment.SizeBytes, s.options.UploadTTL)
		if err != nil {
			return err
		}
		signed.ExpiresAt = attachment.UploadExpiresAt
		result = Upload{ID: id, Signed: signed}
		return nil
	})
	return result, err
}
