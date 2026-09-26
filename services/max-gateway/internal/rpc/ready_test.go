package rpc

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
)

func TestHTTPReadyReconnectsIdleAndChecksDependencies(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := grpc.NewServer()
	go server.Serve(lis)
	t.Cleanup(server.Stop)
	var code atomic.Int32
	code.Store(http.StatusOK)
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(int(code.Load())) }))
	t.Cleanup(httpServer.Close)
	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithIdleTimeout(100*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	if conn.GetState() != connectivity.Idle {
		t.Fatal("expected initial idle connection")
	}
	check := HTTPReady(conn, httpServer.URL)
	if err := check(context.Background()); err != nil {
		t.Fatalf("initial idle recovery: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	for conn.GetState() != connectivity.Idle {
		state := conn.GetState()
		if state == connectivity.Idle {
			break
		}
		if !conn.WaitForStateChange(ctx, state) {
			t.Fatal("connection did not become idle")
		}
	}
	if err := check(context.Background()); err != nil {
		t.Fatalf("idle timeout recovery: %v", err)
	}
	code.Store(http.StatusServiceUnavailable)
	if err := check(context.Background()); err == nil {
		t.Fatal("unhealthy HTTP dependency reported ready")
	}
	code.Store(http.StatusOK)
	conn.Close()
	if err := check(context.Background()); err == nil {
		t.Fatal("closed RPC connection reported ready")
	}
}

func TestHTTPReadyRespectsCanceledContext(t *testing.T) {
	conn, err := grpc.NewClient("127.0.0.1:1", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := HTTPReady(conn, "http://127.0.0.1:1")(ctx); err == nil {
		t.Fatal("unavailable service reported ready")
	}
}
