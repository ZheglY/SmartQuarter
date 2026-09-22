package main

import (
	"fmt"
	"github.com/ZheglY/SmartQuarter/services/identity-service/internal/admin"
	"github.com/ZheglY/SmartQuarter/services/identity-service/internal/config"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/ZheglY/SmartQuarter/services/identity-service/internal/app"
)

func main() {
	if err := run(); err != nil {
		slog.Error("application startup error", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) == 1 {
		return app.Run()
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	switch os.Args[1] {
	case "provision":
		return admin.Provision(os.Args[2:], cfg.DatabaseURL, os.Stdout)
	case "healthcheck":
		res, err := (&http.Client{Timeout: 3 * time.Second}).Get("http://127.0.0.1" + cfg.HTTPPort + "/readyz")
		if err != nil {
			return fmt.Errorf("identity not ready")
		}
		defer res.Body.Close()
		if res.StatusCode != 200 {
			return fmt.Errorf("identity not ready")
		}
		return nil
	default:
		return fmt.Errorf("unknown command %q", os.Args[1])
	}
}
