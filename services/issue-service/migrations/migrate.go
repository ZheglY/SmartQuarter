package migrations

import (
	"embed"
	"errors"
	"fmt"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"net/url"
	"strings"
)

//go:embed *.sql
var files embed.FS

func Run(databaseURL, direction string) error {
	u, err := url.Parse(databaseURL)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.Host == "" || (u.Path != "/issue_db" && !strings.HasPrefix(u.Path, "/issue_test")) {
		return fmt.Errorf("migration requires a PostgreSQL URL for issue_db or issue_test*")
	}
	if direction != "up" && direction != "down" {
		return fmt.Errorf("migration direction must be up or down")
	}
	source, err := iofs.New(files, ".")
	if err != nil {
		return err
	}
	dsn := "pgx5" + databaseURL[strings.Index(databaseURL, ":"):]
	m, err := migrate.NewWithSourceInstance("iofs", source, dsn)
	if err != nil {
		return fmt.Errorf("migration connection failed")
	}
	defer m.Close()
	switch direction {
	case "up":
		err = m.Up()
	case "down":
		err = m.Down()
	default:
		return fmt.Errorf("migration direction must be up or down")
	}
	if errors.Is(err, migrate.ErrNoChange) {
		return nil
	}
	return err
}
