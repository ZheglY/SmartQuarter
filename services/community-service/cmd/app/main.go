package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/ZheglY/SmartQuarter/services/community-service/internal/config"
	"github.com/ZheglY/SmartQuarter/services/community-service/internal/contacts"
	"github.com/ZheglY/SmartQuarter/services/community-service/internal/events"
	communityv1 "github.com/ZheglY/SmartQuarter/services/community-service/internal/gen/community/v1"
	"github.com/ZheglY/SmartQuarter/services/community-service/internal/observability/logs"
	"github.com/ZheglY/SmartQuarter/services/community-service/internal/repository/postgres"
	grpc_transport "github.com/ZheglY/SmartQuarter/services/community-service/internal/transport/grpc"
	http_transport "github.com/ZheglY/SmartQuarter/services/community-service/internal/transport/http"
	"github.com/ZheglY/SmartQuarter/services/community-service/internal/usecase"
)

func main() {
	logger := logs.NewLogger()
	defer logger.Sync()

	if err := run(logger); err != nil {
		logger.Fatal("application stopped with error", zap.Error(err))
	}
}

func run(logger *zap.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	pool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := pool.Ping(context.Background()); err != nil {
		logger.Warn("Failed to ping DB on startup", zap.Error(err))
	} else {
		logger.Info("Connected to PostgreSQL")
	}

	opt, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return err
	}
	redisClient := redis.NewClient(opt)
	defer redisClient.Close()

	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		logger.Warn("Failed to ping Redis on startup", zap.Error(err))
	} else {
		logger.Info("Connected to Redis")
	}

	repo := postgres.NewCommunityRepo(pool)
	uc := usecase.NewCommunityService(repo)

	outboxWorker := events.NewOutboxWorker(repo, redisClient, logger)
	go outboxWorker.Start(ctx)

	grpcHandler := grpc_transport.NewCommunityHandler(uc, logger)
	grpcHandler.Contacts = &contacts.Service{DB: pool}
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(grpc_transport.LoggingInterceptor(logger)),
	)

	communityv1.RegisterCommunityServiceServer(grpcServer, grpcHandler)
	reflection.Register(grpcServer)

	grpcListener, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		return err
	}
	go func() {
		logger.Info("gRPC server started", zap.String("address", cfg.GRPCAddr))
		_ = grpcServer.Serve(grpcListener)
	}()

	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           http_transport.NewHandler(pool),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errs := make(chan error, 1)
	go func() {
		logger.Info("HTTP technical server started", zap.String("address", cfg.HTTPAddr))
		errs <- httpServer.ListenAndServe()
	}()

	select {
	case err := <-errs:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		logger.Info("shutting down gracefully...")
		grpcServer.GracefulStop()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return httpServer.Shutdown(shutdownCtx)
	}
}
