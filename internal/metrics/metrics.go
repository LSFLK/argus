package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	HttpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "argus_http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"path", "method", "status"},
	)

	HttpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "argus_http_request_duration_seconds",
			Help:    "Duration of HTTP requests in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"path", "method"},
	)

	LogsIngestedTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "argus_logs_ingested_total",
			Help: "Total number of audit logs successfully ingested",
		},
	)

	SignatureVerificationErrors = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "argus_signature_verification_errors_total",
			Help: "Total number of signature verification failures",
		},
	)
)
