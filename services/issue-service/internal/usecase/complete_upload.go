package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/domain"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/repository"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/storage"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net"
	"time"
)

func (s *Service) CompleteUpload(ctx context.Context, a domain.Actor, house, id string) (result domain.Attachment, err error) {
	if err = validate(ctx, a, house); err != nil {
		return result, err
	}
	if !domain.ValidID(id) {
		return result, domain.ErrInvalid
	}
	// No connection or row lock is held while S3 is contacted and the file is read.
	snapshot, err := s.store.Read().Attachment(ctx, id, false)
	if err != nil {
		return result, err
	}
	if err = snapshot.OwnedBy(a); err != nil {
		return result, err
	}
	if snapshot.Status == domain.Ready {
		return snapshot, nil
	}
	if snapshot.Status != domain.Uploading {
		return result, domain.ErrPrecondition
	}
	target, cause := domain.Expired, domain.ErrPrecondition
	digest, etag := "", ""
	if s.now().Before(snapshot.UploadExpiresAt) {
		digest, etag, err = s.verifyUpload(ctx, snapshot)
		if err != nil && !errors.Is(err, domain.ErrInvalid) {
			return result, err
		}
		target, cause = domain.Ready, nil
		if err != nil {
			target, cause = domain.Rejected, err
		}
	}
	var businessErr error
	deleteKey := ""
	err = s.store.WithinTx(ctx, func(q repository.Queries) error {
		attachment, e := q.Attachment(ctx, id, true)
		if e != nil {
			return e
		}
		if e = attachment.OwnedBy(a); e != nil {
			return e
		}
		if attachment.Status == domain.Ready {
			result = attachment
			return nil
		}
		if attachment.Status != domain.Uploading {
			return domain.ErrPrecondition
		}
		// Never apply a verification result to a different object or upload contract.
		if attachment.ObjectKey != snapshot.ObjectKey || attachment.MIMEType != snapshot.MIMEType || attachment.SizeBytes != snapshot.SizeBytes || !attachment.UploadExpiresAt.Equal(snapshot.UploadExpiresAt) {
			return domain.ErrPrecondition
		}
		now := s.now()
		if !now.Before(attachment.UploadExpiresAt) {
			target, cause = domain.Expired, domain.ErrPrecondition
		}
		if e = attachment.Transition(target); e != nil {
			return e
		}
		if target == domain.Ready {
			attachment.SHA256, attachment.ETag = digest, etag
		} else {
			businessErr = cause
			deleteKey = attachment.ObjectKey
		}
		attachment.UpdatedAt = now
		if e = q.UpdateAttachment(ctx, attachment); e != nil {
			return e
		}
		result = attachment
		return nil
	})
	if err != nil {
		return result, err
	}
	if deleteKey != "" {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		defer cancel()
		_ = s.objects.DeleteObject(cleanup, deleteKey)
	}
	return result, businessErr
}

func (s *Service) verifyUpload(ctx context.Context, attachment domain.Attachment) (string, string, error) {
	info, err := s.objects.HeadObject(ctx, attachment.ObjectKey)
	if errors.Is(err, storage.ErrObjectNotFound) {
		return "", "", domain.Fail(domain.ErrPrecondition, "upload not found")
	}
	if err != nil {
		return "", "", err
	}
	if info.Size != attachment.SizeBytes || info.Size <= 0 || info.Size > s.options.MaxUploadSize || info.ContentType != attachment.MIMEType {
		return "", "", domain.ErrInvalid
	}
	if info.ETag == "" {
		return "", "", domain.ErrUnavailable
	}
	body, err := s.objects.GetObject(ctx, attachment.ObjectKey, info.ETag)
	if err != nil {
		return "", "", err
	}
	defer body.Close()
	digest, err := verifyImage(ctx, body, attachment.MIMEType, attachment.SizeBytes)
	return digest, info.ETag, err
}

type countingWriter struct{ n int64 }

func (c *countingWriter) Write(p []byte) (int, error) { c.n += int64(len(p)); return len(p), nil }

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}
func verifyImage(ctx context.Context, body io.Reader, mime string, expected int64) (string, error) {
	hash := sha256.New()
	count := &countingWriter{}
	stream := io.TeeReader(io.LimitReader(contextReader{ctx, body}, expected+1), io.MultiWriter(hash, count))
	cfg, format, err := image.DecodeConfig(stream)
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		var networkError net.Error
		if errors.As(err, &networkError) {
			return "", domain.ErrUnavailable
		}
		return "", domain.ErrInvalid
	}
	if "image/"+format != mime || cfg.Width <= 0 || cfg.Height <= 0 || int64(cfg.Width)*int64(cfg.Height) > 100_000_000 {
		return "", domain.ErrInvalid
	}
	if _, err = io.Copy(io.Discard, stream); err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", domain.ErrUnavailable
	}
	if count.n != expected {
		return "", domain.ErrInvalid
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
