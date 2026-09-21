package storage

import (
	"context"
	"errors"
	"io"
	"time"
)

var ErrObjectNotFound = errors.New("object not found")

type SignedURL struct {
	URL       string
	Headers   map[string]string
	ExpiresAt time.Time
}
type ObjectInfo struct {
	Size              int64
	ContentType, ETag string
}
type ObjectStorage interface {
	PresignUpload(context.Context, string, string, int64, time.Duration) (SignedURL, error)
	PresignDownload(context.Context, string, time.Duration) (SignedURL, error)
	HeadObject(context.Context, string) (ObjectInfo, error)
	GetObject(context.Context, string, string) (io.ReadCloser, error)
	DeleteObject(context.Context, string) error
}
