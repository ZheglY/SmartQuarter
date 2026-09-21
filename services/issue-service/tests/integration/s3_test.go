//go:build integration

package integration

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/storage"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/storage/yandexs3"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	"github.com/google/uuid"
)

func objects(t *testing.T) *yandexs3.Client {
	t.Helper()
	endpoint := os.Getenv("TEST_S3_ENDPOINT")
	if endpoint != "http://localhost:19000" {
		t.Fatal("TEST_S3_ENDPOINT must point to the dedicated local MinIO")
	}
	cfg, err := awsconfig.LoadDefaultConfig(context.Background(), awsconfig.WithRegion("ru-central1"))
	if err != nil {
		t.Fatal(err)
	}
	api := s3.NewFromConfig(cfg, func(o *s3.Options) { o.BaseEndpoint = aws.String(endpoint); o.UsePathStyle = true })
	_, err = api.CreateBucket(context.Background(), &s3.CreateBucketInput{Bucket: aws.String("issue-test")})
	if err != nil {
		var apiError smithy.APIError
		if !errors.As(err, &apiError) || (apiError.ErrorCode() != "BucketAlreadyOwnedByYou" && apiError.ErrorCode() != "BucketAlreadyExists") {
			t.Fatal("cannot create test bucket")
		}
	}
	client, err := yandexs3.New(context.Background(), yandexs3.Options{Endpoint: endpoint, Region: "ru-central1", Bucket: "issue-test", PathStyle: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return client
}
func signedPut(t *testing.T, p storage.SignedURL, data []byte) int {
	t.Helper()
	req, err := http.NewRequest("PUT", p.URL, bytes.NewReader(data))
	if err != nil {
		t.Fatal("invalid signed request")
	}
	for k, v := range p.Headers {
		req.Header.Set(k, v)
	}
	response, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		t.Fatal("signed PUT failed")
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, response.Body)
	return response.StatusCode
}
func TestS3(t *testing.T) {
	c := objects(t)
	ctx := context.Background()
	key := "houses/" + uuid.NewString() + "/attachments/" + uuid.NewString() + "/original"
	t.Cleanup(func() { _ = c.DeleteObject(ctx, key) })
	data := []byte("test payload")
	put, err := c.PresignUpload(ctx, key, "image/png", int64(len(data)), time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if code := signedPut(t, put, data); code != 200 {
		t.Fatalf("PUT status=%d", code)
	}
	if code := signedPut(t, put, data); code != 412 {
		t.Fatalf("overwrite status=%d, expected 412", code)
	}
	info, err := c.HeadObject(ctx, key)
	if err != nil || info.Size != int64(len(data)) {
		t.Fatal("HEAD failed")
	}
	body, err := c.GetObject(ctx, key, info.ETag)
	if err != nil {
		t.Fatal(err)
	}
	actual, _ := io.ReadAll(body)
	body.Close()
	if !bytes.Equal(actual, data) {
		t.Fatal("GET data mismatch")
	}
	download, err := c.PresignDownload(ctx, key, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.Get(download.URL)
	if err != nil {
		t.Fatal("signed GET failed")
	}
	response.Body.Close()
	if response.StatusCode != 200 {
		t.Fatal("signed GET denied")
	}
	time.Sleep(2100 * time.Millisecond)
	response, err = http.Get(download.URL)
	if err != nil {
		t.Fatal("expired GET request failed")
	}
	response.Body.Close()
	if response.StatusCode != 403 {
		t.Fatalf("expired GET status=%d", response.StatusCode)
	}
	missingHeader := put
	missingHeader.Headers = map[string]string{"Content-Type": "image/png"}
	if code := signedPut(t, missingHeader, data); code != 400 && code != 403 {
		t.Fatalf("unsigned overwrite protection was accepted: %d", code)
	}
	if err = c.Check(ctx); err != nil {
		t.Fatal("bucket readiness failed")
	}
	expiredKey := key + "-expired"
	defer c.DeleteObject(ctx, expiredKey)
	expiredPut, err := c.PresignUpload(ctx, expiredKey, "image/png", int64(len(data)), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(2100 * time.Millisecond)
	if code := signedPut(t, expiredPut, data); code != 403 {
		t.Fatalf("expired PUT status=%d", code)
	}
	if strings.Contains(put.URL, "not-a-real-secret") {
		t.Fatal("secret leaked in URL")
	}
}
