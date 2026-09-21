package migrations

import "testing"

func TestInvalidDatabase(t *testing.T) {
	for _, s := range []string{"", "no-colon", "postgres://localhost/identity_db", "https://localhost/issue_db"} {
		if err := Run(s, "up"); err == nil {
			t.Fatal("accepted unsafe DSN")
		}
	}
}
