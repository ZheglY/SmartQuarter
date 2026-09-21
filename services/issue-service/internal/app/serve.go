package app

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/config"
	"google.golang.org/grpc"
)

func Serve(ctx context.Context, cfg config.Config, server *grpc.Server, handler http.Handler) error {
	listener, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		return errors.New("cannot listen on GRPC_ADDR")
	}
	defer listener.Close()
	httpListener, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		return errors.New("cannot listen on HTTP_ADDR")
	}
	defer httpListener.Close()
	health := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: time.Minute}
	errorsCh := make(chan error, 2)
	go func() { errorsCh <- server.Serve(listener) }()
	go func() { errorsCh <- health.Serve(httpListener) }()
	select {
	case <-ctx.Done():
	case err = <-errorsCh:
	}
	stopped := make(chan struct{})
	go func() { server.GracefulStop(); close(stopped) }()
	select {
	case <-stopped:
	case <-time.After(cfg.ShutdownTimeout):
		server.Stop()
		<-stopped
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if shutdownErr := health.Shutdown(shutdownCtx); shutdownErr != nil {
		_ = health.Close()
	}
	if errors.Is(err, grpc.ErrServerStopped) || errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
