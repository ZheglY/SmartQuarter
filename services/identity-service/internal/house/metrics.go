package house

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var workflowOperations = promauto.NewCounterVec(prometheus.CounterOpts{Name: "identity_house_operations_total", Help: "House workflow operations by RPC and gRPC status."}, []string{"operation", "code"})
var outboxPublished = promauto.NewCounterVec(prometheus.CounterOpts{Name: "identity_outbox_published_total", Help: "Outbox events published to Redis."}, []string{"event_type"})
var outboxFailures = promauto.NewCounter(prometheus.CounterOpts{Name: "identity_outbox_publish_failures_total", Help: "Failed Identity outbox Redis publications."})
