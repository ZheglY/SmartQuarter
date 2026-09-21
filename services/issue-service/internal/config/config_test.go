package config

import "testing"

func TestLoad(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://issue:secret@localhost/issue_db")
	t.Setenv("S3_BUCKET", "test-bucket")
	t.Setenv("APP_ENV", "local")
	if _, err := Load(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ k, v string }{
		{"DATABASE_URL", "postgres://issue:secret@localhost/identity_db"},
		{"S3_ENDPOINT", "http://user:secret@localhost:9000"},
		{"S3_PUBLIC_ENDPOINT", "https://example.com?secret=1"},
		{"MAX_UPLOAD_SIZE", "-1"}, {"MAX_PAGE_SIZE", "1001"}, {"REQUEST_TIMEOUT", "0s"},
		{"S3_UPLOAD_URL_TTL", "100ms"}, {"S3_DOWNLOAD_URL_TTL", "2h"}, {"GRPC_ADDR", "bad"},
	} {
		t.Run(tc.k, func(t *testing.T) {
			t.Setenv(tc.k, tc.v)
			if _, err := Load(); err == nil {
				t.Fatal("accepted invalid configuration")
			}
		})
	}
	t.Run("production transport", func(t *testing.T) {
		t.Setenv("APP_ENV", "production")
		t.Setenv("S3_ENDPOINT", "http://localhost:9000")
		if _, err := Load(); err == nil {
			t.Fatal("accepted plain HTTP")
		}
	})
}
