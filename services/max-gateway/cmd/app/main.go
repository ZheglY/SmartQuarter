package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/ZheglY/SmartQuarter/services/max-gateway/internal/config"
	cpb "github.com/ZheglY/SmartQuarter/services/max-gateway/internal/gen/smartquarter/community/v1"
	ipb "github.com/ZheglY/SmartQuarter/services/max-gateway/internal/gen/smartquarter/identity/v1"
	pb "github.com/ZheglY/SmartQuarter/services/max-gateway/internal/gen/smartquarter/issue/v1"
	"github.com/ZheglY/SmartQuarter/services/max-gateway/internal/identity"
	"github.com/ZheglY/SmartQuarter/services/max-gateway/internal/maxapi"
	"github.com/ZheglY/SmartQuarter/services/max-gateway/internal/notification"
	"github.com/ZheglY/SmartQuarter/services/max-gateway/internal/observability"
	"github.com/ZheglY/SmartQuarter/services/max-gateway/internal/rpc"
	"github.com/ZheglY/SmartQuarter/services/max-gateway/internal/state"
	transport "github.com/ZheglY/SmartQuarter/services/max-gateway/internal/transport/http"
)

func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
func run() error {
	if len(os.Args) == 2 && os.Args[1] == "healthcheck" {
		addr := os.Getenv("HTTP_ADDR")
		if addr == "" {
			addr = ":8080"
		}
		_, port, e := net.SplitHostPort(addr)
		if e != nil {
			return errors.New("invalid HTTP_ADDR")
		}
		res, e := (&http.Client{Timeout: 3 * time.Second}).Get("http://127.0.0.1:" + port + "/readyz")
		if e != nil {
			return errors.New("gateway unavailable")
		}
		res.Body.Close()
		if res.StatusCode != 200 {
			return errors.New("gateway not ready")
		}
		return nil
	}
	cfg, e := config.Load()
	if e != nil {
		return e
	}
	lc := zap.NewProductionConfig()
	if e = lc.Level.UnmarshalText([]byte(cfg.LogLevel)); e != nil {
		return errors.New("invalid LOG_LEVEL")
	}
	logger, e := lc.Build(zap.Fields(zap.String("service", "max-gateway"), zap.String("environment", cfg.Environment)))
	if e != nil {
		return e
	}
	defer func() { _ = logger.Sync() }()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	metrics := observability.New()
	r := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr, Password: cfg.RedisPassword, DB: cfg.RedisDB, MaxRetries: -1, DialTimeout: cfg.DialTimeout, ReadTimeout: 2 * time.Second, WriteTimeout: 2 * time.Second, ContextTimeoutEnabled: true})
	defer r.Close()
	conn, e := rpc.Dial(cfg.IssueAddr, cfg.RequestTimeout, logger, metrics, cfg.DialTimeout)
	if e != nil {
		return errors.New("invalid Issue address")
	}
	defer conn.Close()
	conn.Connect()
	bot := &maxapi.Client{BaseURL: cfg.BotBaseURL, Token: cfg.BotToken, MiniAppURL: cfg.MiniAppURL, BotUsername: cfg.BotUsername, Store: state.Store{R: r}, Rate: cfg.NotificationRate}
	identityConn, e := rpc.Dial(cfg.IdentityAddr, cfg.RequestTimeout, logger, metrics, cfg.DialTimeout)
	if e != nil {
		return fmt.Errorf("Identity connection: %w", e)
	}
	defer identityConn.Close()
	identityConn.Connect()
	api := &transport.API{Config: cfg, Store: state.Store{R: r}, Identity: identity.NewGRPC(identityConn, cfg.RequestTimeout), Issue: pb.NewIssueServiceClient(conn), Bot: bot, Metrics: metrics, Logger: logger}
	api.House = ipb.NewHouseServiceClient(identityConn)
	if cfg.CommunityAddr != "" {
		community, e := rpc.Dial(cfg.CommunityAddr, cfg.RequestTimeout, logger, metrics, cfg.DialTimeout)
		if e != nil {
			return errors.New("invalid Community address")
		}
		defer community.Close()
		community.Connect()
		api.Community = cpb.NewCommunityServiceClient(community)
		api.CommunityReady = rpc.HTTPReady(community, cfg.CommunityReadyURL)
	}
	api.IssueReady = rpc.HTTPReady(conn, cfg.IssueReadyURL)
	workerCtx, cancelWorker := context.WithCancel(context.Background())
	workerDone := make(chan struct{})
	worker := &notification.Consumer{Redis: r, Stream: cfg.Stream, Group: cfg.Group, Identity: api.Identity, Bot: bot, Metrics: metrics, Logger: logger}
	worker.House = api.House
	go func() { defer close(workerDone); worker.Run(workerCtx) }()
	defer func() { cancelWorker(); <-workerDone }()
	server := &http.Server{Addr: cfg.HTTPAddr, Handler: api.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: cfg.ReadTimeout, WriteTimeout: cfg.WriteTimeout, IdleTimeout: cfg.IdleTimeout}
	errs := make(chan error, 1)
	go func() { errs <- server.ListenAndServe() }()
	logger.Info("gateway listening", zap.String("http_addr", cfg.HTTPAddr))
	select {
	case e := <-errs:
		if !errors.Is(e, http.ErrServerClosed) {
			return e
		}
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		if e = server.Shutdown(shutdown); e != nil {
			_ = server.Close()
			return e
		}
	}
	return nil
}
