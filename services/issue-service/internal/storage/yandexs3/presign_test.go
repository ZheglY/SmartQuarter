package yandexs3

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestConditionalSignature(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "example")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "not-a-real-secret")
	c, err := New(context.Background(), Options{Endpoint: "https://storage.yandexcloud.net", Region: "ru-central1", Bucket: "test-bucket"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	p, err := c.PresignUpload(context.Background(), "houses/h/attachments/a/original", "image/png", 100, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(p.URL)
	if err != nil {
		t.Fatal(err)
	}
	signed := u.Query().Get("X-Amz-SignedHeaders")
	if !strings.Contains(signed, "if-none-match") || p.Headers["If-None-Match"] != "*" {
		t.Fatal("overwrite protection not signed")
	}
	if u.Scheme != "https" || p.Headers["Content-Type"] != "image/png" {
		t.Fatal("invalid upload contract")
	}
}
