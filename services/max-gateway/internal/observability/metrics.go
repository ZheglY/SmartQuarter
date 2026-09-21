package observability

import "github.com/prometheus/client_golang/prometheus"

type Metrics struct {
	Registry                                                                                           *prometheus.Registry
	HTTP, HTTPError, GRPC                                                                              *prometheus.CounterVec
	HTTPDuration, GRPCDuration                                                                         *prometheus.HistogramVec
	SessionErrors, RateRejected, Webhooks, WebhookDuplicates, NotificationEvents, NotificationFailures prometheus.Counter
}

func New() *Metrics {
	m := &Metrics{Registry: prometheus.NewRegistry()}
	m.HTTP = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "http_requests_total", Help: "HTTP requests."}, []string{"method", "route", "status"})
	m.HTTPError = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "http_errors_total", Help: "HTTP errors."}, []string{"code"})
	m.GRPC = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "grpc_client_requests_total", Help: "RPC requests."}, []string{"method", "code"})
	m.HTTPDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "http_request_duration_seconds", Help: "HTTP duration."}, []string{"route"})
	m.GRPCDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "grpc_client_request_duration_seconds", Help: "RPC duration."}, []string{"method"})
	m.Registry.MustRegister(m.HTTP, m.HTTPError, m.GRPC, m.HTTPDuration, m.GRPCDuration)
	for _, v := range []struct {
		name string
		p    *prometheus.Counter
	}{{"session_errors_total", &m.SessionErrors}, {"rate_limit_rejections_total", &m.RateRejected}, {"webhook_requests_total", &m.Webhooks}, {"webhook_duplicates_total", &m.WebhookDuplicates}, {"notification_events_total", &m.NotificationEvents}, {"notification_failures_total", &m.NotificationFailures}} {
		*v.p = prometheus.NewCounter(prometheus.CounterOpts{Name: v.name, Help: v.name})
		m.Registry.MustRegister(*v.p)
	}
	return m
}
