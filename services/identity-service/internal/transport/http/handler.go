package http

import "net/http"

// NewHandler exposes only process liveness; dependency readiness is added with adapters.
func NewHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{\"status\":\"ok\",\"usecase\":\"identity-usecase\"}\n"))
	})
	return mux
}
