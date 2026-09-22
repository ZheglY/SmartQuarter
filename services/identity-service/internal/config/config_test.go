package config

import "testing"

func TestConfigFailsClosed(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	if _, err := Load(); err == nil {
		t.Fatal("missing DB accepted")
	}
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost/issue_db")
	if _, err := Load(); err == nil {
		t.Fatal("foreign DB accepted")
	}
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost/identity_test")
	if _, err := Load(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("IDENTITY_HTTP_PORT", "50051")
	if _, err := Load(); err == nil {
		t.Fatal("port collision accepted")
	}
}
