package rpc

import (
	"context"
	"errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"net/http"
	"time"
)

// HTTPReady requires both a live RPC connection and healthy service dependencies.
func HTTPReady(conn *grpc.ClientConn, url string) func(context.Context) error {
	client := &http.Client{Timeout: 2 * time.Second}
	return func(ctx context.Context) error {
		if conn.GetState() != connectivity.Ready {
			return errors.New("gRPC unavailable")
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		res, err := client.Do(req)
		if err != nil {
			return err
		}
		defer res.Body.Close()
		if res.StatusCode != http.StatusOK {
			return errors.New("service dependencies unavailable")
		}
		return nil
	}
}
