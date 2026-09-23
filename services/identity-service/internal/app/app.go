package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/ZheglY/SmartQuarter/services/identity-service/internal/config"
	identityv1 "github.com/ZheglY/SmartQuarter/services/identity-service/internal/gen/smartquarter/identity/v1"
	"github.com/ZheglY/SmartQuarter/services/identity-service/internal/house"
	"github.com/ZheglY/SmartQuarter/services/identity-service/internal/repository/postgres"
	grpchandler "github.com/ZheglY/SmartQuarter/services/identity-service/internal/transport/grpc"
	httphandler "github.com/ZheglY/SmartQuarter/services/identity-service/internal/transport/http"
	"github.com/ZheglY/SmartQuarter/services/identity-service/internal/usecase"
	"github.com/ZheglY/SmartQuarter/services/identity-service/migrations"
)

func Run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	var level slog.Level
	_ = level.UnmarshalText([]byte(cfg.LogLevel))
	// 1. Инициализация структурированного логгера
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	}))
	slog.SetDefault(logger)

	// 2. Загрузка конфигурации

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// 3. Автоматическое применение миграций goose
	logger.Info("applying database migrations...")
	migrationCtx, migrationCancel := context.WithTimeout(ctx, time.Minute)
	defer migrationCancel()
	if err := migrations.Up(migrationCtx, cfg.DatabaseURL); err != nil {
		return fmt.Errorf("app: migrations failed: %w", err)
	}
	logger.Info("migrations applied successfully")

	// 4. Подключение пула соединений PostgreSQL (pgxpool)
	poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("app: parse db config failed: %w", err)
	}
	poolConfig.MaxConns = 25
	poolConfig.MinConns = 5
	poolConfig.MaxConnLifetime = 1 * time.Hour

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return fmt.Errorf("app: connect to db pool failed: %w", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("app: db ping failed: %w", err)
	}
	logger.Info("connected to postgres pool")

	// 5. Инициализация слоев архитектуры
	repo := postgres.NewPostgres(pool)
	uc := usecase.New(repo)
	grpcService := grpchandler.NewHandler(uc)

	// 6. Запуск gRPC сервера
	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		return handler(ctx, req)
	}))
	identityv1.RegisterIdentityServiceServer(grpcServer, grpcService)
	workflow := &house.Service{DB: pool, Admins: cfg.AdminUserIDs}
	identityv1.RegisterHouseServiceServer(grpcServer, &grpchandler.HouseHandler{Workflow: workflow})
	redisClient := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr, Password: cfg.RedisPassword, MaxRetries: -1, DialTimeout: 2 * time.Second, ReadTimeout: 2 * time.Second, WriteTimeout: 2 * time.Second, ContextTimeoutEnabled: true})
	defer redisClient.Close()
	workerCtx, stopWorker := context.WithCancel(ctx)
	workerDone := make(chan struct{})
	go func() { defer close(workerDone); workflow.RunOutbox(workerCtx, redisClient, cfg.NotificationStream) }()
	defer func() { stopWorker(); <-workerDone }()
	healthpb.RegisterHealthServer(grpcServer, &grpchandler.Health{DB: repo})

	grpcListener, err := net.Listen("tcp", cfg.GRPCPort)
	if err != nil {
		return fmt.Errorf("app: listen grpc port %s failed: %w", cfg.GRPCPort, err)
	}

	errs := make(chan error, 2)
	go func() {
		logger.Info("gRPC server listening", slog.String("port", cfg.GRPCPort))
		if err := grpcServer.Serve(grpcListener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			errs <- err
		}
	}()

	// 7.
	httpRouter := httphandler.NewRouter(repo)
	httpServer := &http.Server{
		Addr:         cfg.HTTPPort,
		Handler:      httpRouter,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("HTTP technical server listening", slog.String("port", cfg.HTTPPort))
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errs <- err
		}
	}()

	// 8. Ожидание сигналов завершения (Graceful Shutdown)
	var serveErr error
	select {
	case <-ctx.Done():
	case serveErr = <-errs:
	}

	// 9. Остановка серверов
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	// Сначала останавливаем прием новых HTTP-запросов
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("HTTP server graceful shutdown failed", slog.String("error", err.Error()))
	}

	// Плавно завершаем все активные gRPC RPC-вызовы
	done := make(chan struct{})
	go func() { grpcServer.GracefulStop(); close(done) }()
	select {
	case <-done:
	case <-shutdownCtx.Done():
		grpcServer.Stop()
	}
	logger.Info("service stopped gracefully")
	return serveErr
}
