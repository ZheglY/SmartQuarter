package config

import "testing"

func TestPlatformAdminAllowlist(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost/identity_test")
	t.Setenv("ADMIN_USER_IDS", "")
	c, err := Load()
	if err != nil || len(c.AdminUserIDs) != 0 {
		t.Fatalf("unexpected default administrator: %v", err)
	}
	for _, bad := range []string{"123456789", "first-user", "00000000-0000-0000-0000-000000000000"} {
		t.Setenv("ADMIN_USER_IDS", bad)
		if _, err = Load(); err == nil {
			t.Fatalf("accepted invalid admin %q", bad)
		}
	}
	id := "11111111-1111-4111-8111-111111111111"
	t.Setenv("ADMIN_USER_IDS", id+","+id)
	c, err = Load()
	if err != nil || len(c.AdminUserIDs) != 1 || !c.AdminUserIDs[id] {
		t.Fatal("explicit allowlist failed", err)
	}
}

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
