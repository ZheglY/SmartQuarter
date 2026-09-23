//go:build integration

package testdata

import (
	"context"
	"github.com/ZheglY/SmartQuarter/services/max-gateway/internal/identity"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"os"
	"testing"
	"time"
)

// RealIdentity seeds only an explicitly named identity_test database. Production
// onboarding uses the operator provision command, never this fixture.
func RealIdentity(t *testing.T) identity.Client {
	t.Helper()
	ctx := context.Background()
	db, err := pgxpool.New(ctx, os.Getenv("IDENTITY_TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if db.Config().ConnConfig.Database != "identity_test" {
		t.Fatal("refusing to seed non-test database")
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	for _, h := range []string{House, ForeignHouse} {
		address := "Test address 1"
		if h == ForeignHouse {
			address = "Test address 2"
		}
		_, err = tx.Exec(ctx, `INSERT INTO houses(id,name,address,city) VALUES($1,'Test house',$2,'Test') ON CONFLICT(id) DO UPDATE SET address=EXCLUDED.address`, h, address)
		if err != nil {
			t.Fatal(err)
		}
	}
	for maxID, id := range Users {
		house, role := House, "RESIDENT"
		if maxID == 103 {
			role = "CHAIRMAN"
		}
		if maxID == 104 {
			house = ForeignHouse
		}
		_, err = tx.Exec(ctx, `INSERT INTO users(id,max_user_id,display_name,default_house_id) VALUES($1,$2,'Test',$3) ON CONFLICT(id) DO UPDATE SET default_house_id=EXCLUDED.default_house_id`, id, maxID, house)
		if err != nil {
			t.Fatal(err)
		}
		_, err = tx.Exec(ctx, `INSERT INTO memberships(user_id,house_id,role,status) VALUES($1,$2,$3,'ACTIVE') ON CONFLICT(user_id,house_id) DO UPDATE SET role=EXCLUDED.role,status='ACTIVE'`, id, house, role)
		if err != nil {
			t.Fatal(err)
		}
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	conn, err := grpc.NewClient(os.Getenv("IDENTITY_TEST_ADDR"), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	client := identity.NewGRPC(conn, 5*time.Second)
	if err = client.Ready(ctx); err != nil {
		t.Fatal(err)
	}
	return client
}

func SetMembership(t *testing.T, maxID int64, role, state string) {
	t.Helper()
	ctx := context.Background()
	db, err := pgxpool.New(ctx, os.Getenv("IDENTITY_TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if db.Config().ConnConfig.Database != "identity_test" {
		t.Fatal("not a test database")
	}
	tag, err := db.Exec(ctx, `UPDATE memberships SET role=$2,status=$3 WHERE user_id=(SELECT id FROM users WHERE max_user_id=$1)`, maxID, role, state)
	if err != nil || tag.RowsAffected() != 1 {
		t.Fatalf("membership update: %v %v", tag, err)
	}
}
