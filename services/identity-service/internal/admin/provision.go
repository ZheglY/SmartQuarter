// Package admin contains operator-only commands; it is not exposed by HTTP/gRPC.
package admin

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"io"
	"strings"
	"time"
)

func Provision(args []string, dsn string, out io.Writer) error {
	fs := flag.NewFlagSet("provision", flag.ContinueOnError)
	fs.SetOutput(out)
	maxID := fs.Int64("max-user-id", 0, "verified MAX user ID")
	houseID := fs.String("house-id", "", "existing or new house UUID (required)")
	name := fs.String("house-name", "", "name when creating a new house")
	address := fs.String("address", "", "address when creating a new house")
	city := fs.String("city", "", "city when creating a new house")
	role := fs.String("role", "RESIDENT", "RESIDENT, CHAIRMAN or ADMIN")
	state := fs.String("status", "ACTIVE", "ACTIVE or INACTIVE")
	if err := fs.Parse(args); err != nil {
		return err
	}
	id, err := uuid.Parse(*houseID)
	if err != nil || id == uuid.Nil || id.String() != *houseID || *maxID <= 0 || fs.NArg() != 0 {
		return errors.New("positive --max-user-id and canonical --house-id required")
	}
	if *role != "RESIDENT" && *role != "CHAIRMAN" && *role != "ADMIN" {
		return errors.New("invalid --role")
	}
	if *state != "ACTIVE" && *state != "INACTIVE" {
		return errors.New("invalid --status")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return errors.New("cannot connect to Identity database")
	}
	defer conn.Close(ctx)
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var exists bool
	if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM houses WHERE id=$1)", *houseID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		if strings.TrimSpace(*name) == "" || strings.TrimSpace(*address) == "" || strings.TrimSpace(*city) == "" {
			return errors.New("new house requires --house-name, --address and --city")
		}
		if _, err = tx.Exec(ctx, "INSERT INTO houses(id,name,address,city) VALUES($1,$2,$3,$4)", *houseID, *name, *address, *city); err != nil {
			return err
		}
	}
	var userID string
	if err = tx.QueryRow(ctx, `INSERT INTO users(max_user_id,display_name) VALUES($1,'Житель')
 ON CONFLICT(max_user_id) DO UPDATE SET max_user_id=EXCLUDED.max_user_id RETURNING id`, *maxID).Scan(&userID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO memberships(user_id,house_id,role,status) VALUES($1,$2,$3,$4)
 ON CONFLICT(user_id,house_id) DO UPDATE SET role=EXCLUDED.role,status=EXCLUDED.status,updated_at=now()`, userID, *houseID, *role, *state); err != nil {
		return err
	}
	if *state == "ACTIVE" {
		if _, err = tx.Exec(ctx, "UPDATE users SET default_house_id=COALESCE(default_house_id,$2),updated_at=now() WHERE id=$1", userID, *houseID); err != nil {
			return err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	_, err = fmt.Fprintf(out, "user_id=%s house_id=%s role=%s status=%s\n", userID, *houseID, *role, *state)
	return err
}
