package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"time"
)

type Metrics struct {
	Registry      *prometheus.Registry
	Requests      *prometheus.CounterVec
	Errors        *prometheus.CounterVec
	Duration      *prometheus.HistogramVec
	OutboxPending prometheus.Gauge
	OutboxErrors  prometheus.Counter
	S3Operations  *prometheus.CounterVec
	S3Errors      *prometheus.CounterVec
}

func NewMetrics() *Metrics {
	m := &Metrics{Registry: prometheus.NewRegistry(),
		Requests:      prometheus.NewCounterVec(prometheus.CounterOpts{Name: "grpc_requests_total", Help: "Completed unary requests."}, []string{"method", "code"}),
		Errors:        prometheus.NewCounterVec(prometheus.CounterOpts{Name: "grpc_errors_total", Help: "Failed unary requests."}, []string{"method", "code"}),
		Duration:      prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "grpc_request_duration_seconds", Help: "Unary request latency.", Buckets: prometheus.DefBuckets}, []string{"method"}),
		OutboxPending: prometheus.NewGauge(prometheus.GaugeOpts{Name: "outbox_pending_count", Help: "Unpublished database events."}),
		OutboxErrors:  prometheus.NewCounter(prometheus.CounterOpts{Name: "outbox_publish_errors_total", Help: "Failed publication attempts."}),
		S3Operations:  prometheus.NewCounterVec(prometheus.CounterOpts{Name: "s3_operations_total", Help: "S3 operations including presigning."}, []string{"operation"}),
		S3Errors:      prometheus.NewCounterVec(prometheus.CounterOpts{Name: "s3_operation_errors_total", Help: "Failed S3 operations."}, []string{"operation"})}
	m.Registry.MustRegister(m.Requests, m.Errors, m.Duration, m.OutboxPending, m.OutboxErrors, m.S3Operations, m.S3Errors, prometheus.NewGoCollector(), prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	return m
}
func (m *Metrics) ObserveS3(operation string, err error) {
	m.S3Operations.WithLabelValues(operation).Inc()
	if err != nil {
		m.S3Errors.WithLabelValues(operation).Inc()
	}
}
func (m *Metrics) ObserveRPC(method, code string, started time.Time) {
	m.Requests.WithLabelValues(method, code).Inc()
	m.Duration.WithLabelValues(method).Observe(time.Since(started).Seconds())
	if code != "OK" {
		m.Errors.WithLabelValues(method, code).Inc()
	}
}
