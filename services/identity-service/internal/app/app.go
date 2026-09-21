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
	"google.golang.org/grpc"

	"github.com/ZheglY/SmartQuarter/services/identity-service/internal/config"
	identityv1 "github.com/ZheglY/SmartQuarter/services/identity-service/internal/gen/smartquarter/identity/v1"
	"github.com/ZheglY/SmartQuarter/services/identity-service/internal/repository/postgres"
	grpchandler "github.com/ZheglY/SmartQuarter/services/identity-service/internal/transport/grpc"
	httphandler "github.com/ZheglY/SmartQuarter/services/identity-service/internal/transport/http"
	"github.com/ZheglY/SmartQuarter/services/identity-service/internal/usecase"
	"github.com/ZheglY/SmartQuarter/services/identity-service/migrations"
)

func Run() error {
	// 1. Инициализация структурированного логгера
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// 2. Загрузка конфигурации
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("app: load config failed: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 3. Автоматическое применение миграций goose
	logger.Info("applying database migrations...")
	if err := migrations.Up(ctx, cfg.DatabaseURL); err != nil {
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
	grpcServer := grpc.NewServer()
	identityv1.RegisterIdentityServiceServer(grpcServer, grpcService)

	grpcListener, err := net.Listen("tcp", cfg.GRPCPort)
	if err != nil {
		return fmt.Errorf("app: listen grpc port %s failed: %w", cfg.GRPCPort, err)
	}

	go func() {
		logger.Info("gRPC server listening", slog.String("port", cfg.GRPCPort))
		if err := grpcServer.Serve(grpcListener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			logger.Error("gRPC server stopped with error", slog.String("error", err.Error()))
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
			logger.Error("HTTP server stopped with error", slog.String("error", err.Error()))
		}
	}()

	// 8. Ожидание сигналов завершения (Graceful Shutdown)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	sig := <-quit

	logger.Info("shutdown signal received", slog.String("signal", sig.String()))

	// 9. Остановка серверов
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	// Сначала останавливаем прием новых HTTP-запросов
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("HTTP server graceful shutdown failed", slog.String("error", err.Error()))
	}

	// Плавно завершаем все активные gRPC RPC-вызовы
	grpcServer.GracefulStop()
	logger.Info("service stopped gracefully")
	return nil
}
