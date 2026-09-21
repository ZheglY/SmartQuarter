//go:build yandex

package integration

import (
	"bytes"
	"context"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/storage"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/storage/yandexs3"
	"github.com/google/uuid"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func TestYandexSmoke(t *testing.T) {
	bucket := os.Getenv("YANDEX_SMOKE_BUCKET")
	if bucket == "" || os.Getenv("YANDEX_SMOKE_CONFIRM") != "yes" {
		t.Skip("set dedicated YANDEX_SMOKE_BUCKET and YANDEX_SMOKE_CONFIRM=yes")
	}
	c, err := yandexs3.New(context.Background(), yandexs3.Options{Endpoint: "https://storage.yandexcloud.net", Region: "ru-central1", Bucket: bucket, PathStyle: true}, nil)
	if err != nil {
		t.Fatal("cannot initialize Yandex S3")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	key := "issue-service-smoke/" + uuid.NewString() + "/original"
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		if e := c.DeleteObject(cleanup, key); e != nil {
			t.Error("smoke object cleanup failed")
		}
	}()
	data := []byte("issue-service conditional upload smoke")
	put, err := c.PresignUpload(ctx, key, "image/png", int64(len(data)), time.Minute)
	if err != nil {
		t.Fatal("presign PUT failed")
	}
	doPut := func(p storage.SignedURL) int {
		req, e := http.NewRequestWithContext(ctx, "PUT", p.URL, bytes.NewReader(data))
		if e != nil {
			t.Fatal("request failed")
		}
		for k, v := range p.Headers {
			req.Header.Set(k, v)
		}
		r, e := (&http.Client{Timeout: 15 * time.Second}).Do(req)
		if e != nil {
			t.Fatal("PUT failed")
		}
		defer r.Body.Close()
		io.Copy(io.Discard, r.Body)
		return r.StatusCode
	}
	if code := doPut(put); code != 200 {
		t.Fatalf("PUT status %d", code)
	}
	if code := doPut(put); code != 412 {
		t.Fatalf("overwrite must be rejected, status %d", code)
	}
	if !strings.Contains(put.URL, "if-none-match") {
		t.Fatal("conditional header not signed")
	}
	info, err := c.HeadObject(ctx, key)
	if err != nil || info.Size != int64(len(data)) {
		t.Fatal("HEAD failed")
	}
	body, err := c.GetObject(ctx, key, info.ETag)
	if err != nil {
		t.Fatal("GET failed")
	}
	got, e := io.ReadAll(body)
	body.Close()
	if e != nil || !bytes.Equal(got, data) {
		t.Fatal("object mismatch")
	}
	signed, err := c.PresignDownload(ctx, key, time.Second)
	if err != nil {
		t.Fatal("presign GET failed")
	}
	time.Sleep(2100 * time.Millisecond)
	req, _ := http.NewRequestWithContext(ctx, "GET", signed.URL, nil)
	r, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		t.Fatal("expired GET failed")
	}
	r.Body.Close()
	if r.StatusCode != 403 {
		t.Fatalf("expired URL status %d", r.StatusCode)
	}
	if err = c.Check(ctx); err != nil {
		t.Fatal("bucket check failed")
	}
}
