package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/app"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/config"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/observability"
	"github.com/ZheglY/SmartQuarter/services/issue-service/migrations"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	// Проверяет в каком режиме запущено приложение [migrate up|down|healthcheck]
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "healthcheck":
			return healthcheck()
		case "migrate":
			if len(os.Args) != 3 {
				return errors.New("usage: app migrate up|down")
			}
			if err := migrations.Run(os.Getenv("DATABASE_URL"), os.Args[2]); err != nil {
				return errors.New("migration failed; check database, schema version and migration direction")
			}
			return nil
		default:
			return errors.New("usage: app [migrate up|down|healthcheck]")
		}
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	logger, err := observability.NewLogger(cfg.Environment, cfg.LogLevel)
	if err != nil {
		return err
	}
	defer func() { _ = logger.Sync() }()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return app.Run(ctx, cfg, logger)
}

func healthcheck() error {
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8083"
	}
	// разделение на host и port :8083 -> :, 8083
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return errors.New("invalid HTTP_ADDR")
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	/*
		отправляется запрос на получение информации о готовности
		c таймаутом 4 сек
	*/
	response, err := (&http.Client{Timeout: 4 * time.Second}).Get("http://" + net.JoinHostPort(host, port) + "/readyz")
	if err != nil {
		return errors.New("readiness request failed")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return errors.New("service not ready")
	}
	return nil
}

/*
healthcheck()
    ↓
узнать HTTP_ADDR
    ↓
определить host и port
    ↓
GET http://127.0.0.1:8083/readyz
    ↓
ответ 200?
   /    \
 да      нет
 ↓        ↓
nil      error
*/
