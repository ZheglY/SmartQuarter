package usecase

import (
	"context"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/domain"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/storage"
)

func (s *Service) GetAttachmentDownloadURL(ctx context.Context, a domain.Actor, house, id string) (storage.SignedURL, error) {
	if err := validate(ctx, a, house); err != nil {
		return storage.SignedURL{}, err
	}
	if !domain.ValidID(id) {
		return storage.SignedURL{}, domain.ErrInvalid
	}
	attachment, err := s.store.Read().Attachment(ctx, id, false)
	if err != nil {
		return storage.SignedURL{}, err
	}
	if err = a.InHouse(attachment.HouseID); err != nil {
		return storage.SignedURL{}, err
	}
	switch attachment.Status {
	case domain.Attached:
	case domain.Ready:
		if err = attachment.OwnedBy(a); err != nil {
			return storage.SignedURL{}, err
		}
	default:
		return storage.SignedURL{}, domain.ErrPrecondition
	}
	return s.objects.PresignDownload(ctx, attachment.ObjectKey, s.options.DownloadTTL)
}
