package app

import (
	"context"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/config"
	"google.golang.org/grpc"
	"net"
	"net/http"
	"testing"
	"time"
)

func freeAddress(t *testing.T) string {
	t.Helper()
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	a := l.Addr().String()
	l.Close()
	return a
}
func TestServeShutdown(t *testing.T) {
	cfg := config.Config{GRPCAddr: freeAddress(t), HTTPAddr: freeAddress(t), ShutdownTimeout: time.Second}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- Serve(ctx, cfg, grpc.NewServer(), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	}()
	deadline := time.Now().Add(3 * time.Second)
	for {
		response, e := (&http.Client{Timeout: 200 * time.Millisecond}).Get("http://" + cfg.HTTPAddr + "/livez")
		if e == nil {
			response.Body.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("HTTP listener never started")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case e := <-done:
		if e != nil {
			t.Fatal(e)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("shutdown timed out")
	}
	for _, addr := range []string{cfg.GRPCAddr, cfg.HTTPAddr} {
		c, e := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if e == nil {
			c.Close()
			t.Fatal("listener still accepting connections")
		}
	}
}
