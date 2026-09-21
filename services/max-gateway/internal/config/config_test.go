package config

import (
	"testing"
)

func TestConfigValidation(t *testing.T) {
	for k, v := range map[string]string{"APP_ENV": "local", "ISSUE_GRPC_ADDR": "localhost:18082", "ISSUE_READY_URL": "http://localhost:18083/readyz", "MAX_BOT_TOKEN": "test-only", "MAX_WEBHOOK_SECRET": "test-only", "MAX_BOT_USERNAME": "test_bot", "MAX_MINIAPP_URL": "https://app.example", "TRUSTED_ORIGINS": "https://app.example"} {
		t.Setenv(k, v)
	}
	if _, e := Load(); e != nil {
		t.Fatal(e)
	}
	t.Setenv("SESSION_TTL", "0s")
	if _, e := Load(); e == nil {
		t.Fatal("zero TTL accepted")
	}
	t.Setenv("SESSION_TTL", "12h")
	t.Setenv("TRUSTED_ORIGINS", "*")
	if _, e := Load(); e == nil {
		t.Fatal("wildcard origin accepted")
	}
	t.Setenv("TRUSTED_ORIGINS", "http://app.example")
	t.Setenv("APP_ENV", "production")
	if _, e := Load(); e == nil {
		t.Fatal("insecure production origin accepted")
	}
}
