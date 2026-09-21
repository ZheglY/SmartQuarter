package app

import (
	"context"
	"errors"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/config"
	pb "github.com/ZheglY/SmartQuarter/services/issue-service/internal/gen/smartquarter/issue/v1"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/observability"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/outbox"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/repository/postgres"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/storage/yandexs3"
	transport "github.com/ZheglY/SmartQuarter/services/issue-service/internal/transport/grpc"
	httptransport "github.com/ZheglY/SmartQuarter/services/issue-service/internal/transport/http"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/usecase"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"time"
)

func Run(ctx context.Context, cfg config.Config, logger *zap.Logger) error {
	startup, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	pc, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return errors.New("invalid PostgreSQL configuration")
	}
	pc.ConnConfig.ConnectTimeout = 5 * time.Second
	pc.MaxConns = 20
	pool, err := pgxpool.NewWithConfig(startup, pc)
	if err != nil {
		return errors.New("cannot initialize PostgreSQL")
	}
	defer pool.Close()
	if err = pool.Ping(startup); err != nil {
		return errors.New("PostgreSQL unavailable")
	}
	metrics := observability.NewMetrics()
	metrics.Registry.MustRegister(
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{Name: "postgres_pool_connections", Help: "Total pool connections."}, func() float64 { return float64(pool.Stat().TotalConns()) }),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{Name: "postgres_pool_acquired_connections", Help: "Acquired pool connections."}, func() float64 { return float64(pool.Stat().AcquiredConns()) }),
		prometheus.NewCounterFunc(prometheus.CounterOpts{Name: "postgres_pool_acquire_total", Help: "Successful pool acquisitions."}, func() float64 { return float64(pool.Stat().AcquireCount()) }),
	)
	objects, err := yandexs3.New(startup, yandexs3.Options{Endpoint: cfg.S3Endpoint, PublicEndpoint: cfg.S3PublicEndpoint, Region: cfg.S3Region, Bucket: cfg.S3Bucket, PathStyle: cfg.S3PathStyle}, metrics.ObserveS3)
	if err != nil {
		return errors.New("cannot initialize S3")
	}
	stream := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr, Password: cfg.RedisPassword, DB: cfg.RedisDB, DialTimeout: time.Second, ReadTimeout: 2 * time.Second, WriteTimeout: 2 * time.Second, MaxRetries: -1, ContextTimeoutEnabled: true})
	defer stream.Close()
	store := postgres.New(pool)
	service := usecase.New(store, objects, usecase.Options{MaxUploadSize: cfg.MaxUploadSize, MaxPageSize: cfg.MaxPageSize, UploadTTL: cfg.UploadTTL, DownloadTTL: cfg.DownloadTTL})
	server := grpc.NewServer(grpc.UnaryInterceptor(transport.Unary(logger, metrics, cfg.RequestTimeout)), grpc.MaxRecvMsgSize(256<<10), grpc.MaxSendMsgSize(4<<20))
	pb.RegisterIssueServiceServer(server, transport.NewServer(service))
	// Drain RPCs and HTTP before stopping the worker, Redis and PostgreSQL.
	workerCtx, stopWorker := context.WithCancel(context.WithoutCancel(ctx))
	workerDone := make(chan struct{})
	publisher := outbox.New(store, stream, cfg.NotificationStream, cfg.OutboxBatchSize, cfg.OutboxPollInterval, metrics, logger)
	go func() { defer close(workerDone); publisher.Run(workerCtx) }()
	defer func() { stopWorker(); <-workerDone }()
	logger.Info("issue-service starting", zap.String("grpc_addr", cfg.GRPCAddr), zap.String("http_addr", cfg.HTTPAddr))
	return Serve(ctx, cfg, server, httptransport.NewHandler(pool.Ping, objects.Check, metrics.Registry))
}
