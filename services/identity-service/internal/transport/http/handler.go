package http

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Pinger - проверка доступности зависимостей (PostgreSQL)
type Pinger interface {
	Ping(ctx context.Context) error
}

type statusResponse struct {
	Status string `json:"status"`
}

// NewRouter регистрирует технические эндпоинты сервиса
func NewRouter(ping Pinger) http.Handler {
	mux := http.NewServeMux()

	// 1. Live probe: процесс запущен
	mux.HandleFunc("GET /livez", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(statusResponse{Status: "ok"})
	})

	// 2. Ready probe: проверка соединения с базой данных
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if ping != nil {
			if err := ping.Ping(ctx); err != nil {
				w.WriteHeader(http.StatusServiceUnavailable)
				_ = json.NewEncoder(w).Encode(statusResponse{Status: "not_ready"})
				return
			}
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(statusResponse{Status: "ok"})
	})

	// 3. Prometheus metrics
	mux.Handle("GET /metrics", promhttp.Handler())

	return mux
}
