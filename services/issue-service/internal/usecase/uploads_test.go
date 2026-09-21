package usecase

import (
	"bytes"
	"context"
	"errors"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/domain"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/repository"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/storage"
	"github.com/google/uuid"
	"image"
	"image/png"
	"io"
	"testing"
	"time"
)

type fakeStore struct {
	repository.Queries
	transactionError error
}

func (f *fakeStore) Read() repository.Queries { return f.Queries }
func (f *fakeStore) WithinTx(ctx context.Context, fn func(repository.Queries) error) error {
	if f.transactionError != nil {
		return f.transactionError
	}
	return fn(f.Queries)
}

type uploadQueries struct {
	repository.Queries
	attachment domain.Attachment
	err        error
}

func (q *uploadQueries) InsertAttachment(_ context.Context, a domain.Attachment) error {
	q.attachment = a
	return q.err
}
func (q *uploadQueries) UpdateAttachment(_ context.Context, a domain.Attachment) error {
	q.attachment = a
	return q.err
}
func (q *uploadQueries) Attachment(context.Context, string, bool) (domain.Attachment, error) {
	return q.attachment, q.err
}

type fakeObjects struct {
	storage.ObjectStorage
	data    []byte
	err     error
	deleted bool
	mime    string
	size    int64
}

func (o *fakeObjects) PresignUpload(context.Context, string, string, int64, time.Duration) (storage.SignedURL, error) {
	return storage.SignedURL{URL: "https://example.invalid/put"}, o.err
}
func (o *fakeObjects) PresignDownload(context.Context, string, time.Duration) (storage.SignedURL, error) {
	return storage.SignedURL{URL: "https://example.invalid/get"}, o.err
}
func (o *fakeObjects) HeadObject(context.Context, string) (storage.ObjectInfo, error) {
	n := int64(len(o.data))
	if o.size != 0 {
		n = o.size
	}
	return storage.ObjectInfo{Size: n, ContentType: o.mime, ETag: "test-etag"}, o.err
}
func (o *fakeObjects) GetObject(context.Context, string, string) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(o.data)), o.err
}
func (o *fakeObjects) DeleteObject(context.Context, string) error { o.deleted = true; return nil }
func pngBytes() []byte {
	var b bytes.Buffer
	_ = png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 2, 2)))
	return b.Bytes()
}
func actor() domain.Actor {
	return domain.Actor{UserID: uuid.NewString(), HouseID: uuid.NewString(), Role: domain.Resident}
}
func TestUploads(t *testing.T) {
	a := actor()
	now := time.Now()
	q := &uploadQueries{}
	o := &fakeObjects{data: pngBytes(), mime: "image/png"}
	service := New(&fakeStore{Queries: q}, o, Options{Now: func() time.Time { return now }})
	input := CreateUploadInput{HouseID: a.HouseID, Filename: "photo.png", MIMEType: "image/png", SizeBytes: int64(len(o.data))}
	upload, err := service.CreateUpload(context.Background(), a, input)
	if err != nil || upload.ID == "" {
		t.Fatal(err)
	}
	ready, err := service.CompleteUpload(context.Background(), a, a.HouseID, upload.ID)
	if err != nil || ready.Status != domain.Ready || len(ready.SHA256) != 64 {
		t.Fatal("complete failed", err)
	}
	o.err = domain.ErrUnavailable
	replay, err := service.CompleteUpload(context.Background(), a, a.HouseID, upload.ID)
	if err != nil || replay.SHA256 != ready.SHA256 {
		t.Fatal("READY replay must not call S3", err)
	}
	for _, test := range []struct {
		name   string
		change func()
		want   error
	}{
		{"different user", func() { q.attachment.UploadedBy = uuid.NewString() }, domain.ErrPermission},
		{"different house", func() { q.attachment.HouseID = uuid.NewString() }, domain.ErrPermission},
		{"attached", func() { q.attachment.Status = domain.Attached }, domain.ErrPrecondition},
		{"expired", func() { q.attachment.UploadExpiresAt = now.Add(-time.Second) }, domain.ErrPrecondition},
		{"missing object", func() { o.err = storage.ErrObjectNotFound }, domain.ErrPrecondition},
		{"S3 unavailable", func() { o.err = domain.ErrUnavailable }, domain.ErrUnavailable},
		{"size mismatch", func() { o.size = 123 }, domain.ErrInvalid},
		{"invalid signature", func() { o.data = bytes.Repeat([]byte("x"), len(pngBytes())) }, domain.ErrInvalid},
	} {
		t.Run(test.name, func(t *testing.T) {
			q.attachment = domain.Attachment{ID: upload.ID, HouseID: a.HouseID, UploadedBy: a.UserID, Status: domain.Uploading, UploadExpiresAt: now.Add(time.Minute), MIMEType: "image/png", SizeBytes: int64(len(pngBytes()))}
			o.err = nil
			o.size = 0
			o.data = pngBytes()
			test.change()
			_, e := service.CompleteUpload(context.Background(), a, a.HouseID, upload.ID)
			if !errors.Is(e, test.want) {
				t.Fatalf("want %v got %v", test.want, e)
			}
			if test.want == domain.ErrInvalid && q.attachment.Status != domain.Rejected {
				t.Error("invalid upload not rejected")
			}
		})
	}
	input.Filename = "../bad.png"
	if _, err = service.CreateUpload(context.Background(), a, input); !errors.Is(err, domain.ErrInvalid) {
		t.Fatal("bad filename accepted")
	}
	input.Filename = "ok.png"
	o.err = domain.ErrUnavailable
	if _, err = service.CreateUpload(context.Background(), a, input); !errors.Is(err, domain.ErrUnavailable) {
		t.Fatal("S3 error missing")
	}
	q.err = domain.ErrUnavailable
	if _, err = service.CreateUpload(context.Background(), a, input); !errors.Is(err, domain.ErrUnavailable) {
		t.Fatal("repository error missing")
	}
	if _, err = service.CreateUpload(context.Background(), domain.Actor{}, input); !errors.Is(err, domain.ErrUnauthenticated) {
		t.Fatal("empty actor accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = service.CreateUpload(ctx, a, input); !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation ignored")
	}
}
