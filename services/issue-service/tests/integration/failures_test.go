//go:build integration

package integration

import (
	"context"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/domain"
	pb "github.com/ZheglY/SmartQuarter/services/issue-service/internal/gen/smartquarter/issue/v1"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/storage"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/storage/yandexs3"
	transport "github.com/ZheglY/SmartQuarter/services/issue-service/internal/transport/grpc"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/usecase"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDependencyFailures(t *testing.T) {
	pool, store := database(t)
	house, user := uuid.NewString(), uuid.NewString()
	ctx := actorContext(house, user, "RESIDENT")
	outage := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) }))
	defer outage.Close()
	s3, err := yandexs3.New(context.Background(), yandexs3.Options{Endpoint: outage.URL, Region: "ru-central1", Bucket: "issue-test", PathStyle: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	c := client(t, transport.NewServer(usecase.New(store, s3, usecase.Options{})))
	u, err := c.CreateUpload(ctx, &pb.CreateUploadRequest{HouseId: house, Filename: "photo.png", MimeType: "image/png", SizeBytes: 100})
	requireCode(t, err, codes.OK)
	_, err = c.CompleteUpload(ctx, &pb.CompleteUploadRequest{HouseId: house, UploadId: u.UploadId})
	requireCode(t, err, codes.Unavailable)
	a, err := store.Read().Attachment(context.Background(), u.UploadId, false)
	if err != nil || a.Status != domain.Uploading {
		t.Fatal("S3 outage changed attachment state", err)
	}
	pool.Close() // Connection loss must be translated, with no raw pgx details.
	_, err = c.ListIssues(ctx, &pb.ListIssuesRequest{HouseId: house})
	requireCode(t, err, codes.Unavailable)
}
func TestRejectedAndExpiredUploads(t *testing.T) {
	_, store := database(t)
	objects := objects(t)
	background := context.Background()
	house, user := uuid.NewString(), uuid.NewString()
	ctx := actorContext(house, user, "RESIDENT")
	now := time.Now().UTC()
	c := client(t, transport.NewServer(usecase.New(store, objects, usecase.Options{Now: func() time.Time { return now }, UploadTTL: time.Minute})))
	data := []byte("this is not a PNG")
	upload, err := c.CreateUpload(ctx, &pb.CreateUploadRequest{HouseId: house, Filename: "bad.png", MimeType: "image/png", SizeBytes: int64(len(data))})
	requireCode(t, err, codes.OK)
	if code := signedPut(t, storage.SignedURL{URL: upload.PutUrl, Headers: upload.RequiredHeaders}, data); code != 200 {
		t.Fatalf("PUT %d", code)
	}
	_, err = c.CompleteUpload(ctx, &pb.CompleteUploadRequest{HouseId: house, UploadId: upload.UploadId})
	requireCode(t, err, codes.InvalidArgument)
	a, err := store.Read().Attachment(background, upload.UploadId, false)
	if err != nil || a.Status != domain.Rejected {
		t.Fatal("rejection not committed", err)
	}
	expired, err := c.CreateUpload(ctx, &pb.CreateUploadRequest{HouseId: house, Filename: "late.png", MimeType: "image/png", SizeBytes: 100})
	requireCode(t, err, codes.OK)
	// Use a separate service with a later clock rather than racing a mutable test clock.
	late := client(t, transport.NewServer(usecase.New(store, objects, usecase.Options{Now: func() time.Time { return now.Add(2 * time.Minute) }})))
	_, err = late.CompleteUpload(ctx, &pb.CompleteUploadRequest{HouseId: house, UploadId: expired.UploadId})
	requireCode(t, err, codes.FailedPrecondition)
	a, err = store.Read().Attachment(background, expired.UploadId, false)
	if err != nil || a.Status != domain.Expired {
		t.Fatal("expiry not committed", err)
	}
}
