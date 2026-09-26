package rpc

import (
	"context"
	"errors"
	"net/http"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

// HTTPReady requires both a live RPC connection and healthy service dependencies.
func HTTPReady(conn *grpc.ClientConn, url string) func(context.Context) error {
	client := &http.Client{Timeout: 2 * time.Second}
	return func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		// A quiet ClientConn enters Idle. Merely reading its state never wakes it,
		// leaving an otherwise healthy service unready until a business RPC arrives.
		for {
			state := conn.GetState()
			if state == connectivity.Ready {
				break
			}
			if state == connectivity.Shutdown {
				return errors.New("gRPC connection closed")
			}
			if state == connectivity.Idle {
				conn.Connect()
			}
			if !conn.WaitForStateChange(ctx, state) {
				return ctx.Err()
			}
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
