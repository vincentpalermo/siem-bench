package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	CollectorRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "siem_collector_requests_total",
			Help: "Total number of HTTP requests handled by collector.",
		},
		[]string{"path", "method", "status"},
	)

	CollectorRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "siem_collector_request_duration_seconds",
			Help:    "HTTP request duration in collector.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"path", "method", "status"},
	)

	CollectorPublishErrorsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "siem_collector_publish_errors_total",
			Help: "Total number of Redis publish errors in collector.",
		},
	)

	CollectorEventsAcceptedTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "siem_collector_events_accepted_total",
			Help: "Total number of events accepted by collector.",
		},
	)

	WorkerMessagesReadTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "siem_worker_messages_read_total",
			Help: "Total number of messages read by worker from Redis.",
		},
		[]string{"backend"},
	)

	WorkerReadErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "siem_worker_read_errors_total",
			Help: "Total number of Redis read errors in worker.",
		},
		[]string{"backend"},
	)

	WorkerEventsStoredTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "siem_worker_events_stored_total",
			Help: "Total number of events successfully stored by worker.",
		},
		[]string{"backend"},
	)

	WorkerInsertErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "siem_worker_insert_errors_total",
			Help: "Total number of backend insert errors in worker.",
		},
		[]string{"backend"},
	)

	WorkerAckErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "siem_worker_ack_errors_total",
			Help: "Total number of Redis ACK errors in worker.",
		},
		[]string{"backend"},
	)

	WorkerInsertDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "siem_worker_insert_duration_seconds",
			Help:    "Duration of successful worker insert operations.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"backend"},
	)

	WorkerBatchSize = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "siem_worker_batch_size",
			Help:    "Batch size processed by worker.",
			Buckets: []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000},
		},
		[]string{"backend"},
	)

	WorkerE2ELatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "siem_worker_e2e_latency_seconds",
			Help:    "End-to-end latency from event generation to successful worker processing.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"backend"},
	)

	WorkerQueueLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "siem_worker_queue_latency_seconds",
			Help:    "Latency from collector ingest to successful worker processing.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"backend"},
	)

	WorkerStreamLen = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "siem_worker_stream_len",
			Help: "Current Redis stream length observed by worker.",
		},
		[]string{"backend"},
	)

	WorkerPendingMessages = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "siem_worker_pending_messages",
			Help: "Current number of pending Redis messages in worker consumer group.",
		},
		[]string{"backend"},
	)

	RunInfo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "siem_run_info",
			Help: "Static information about current run configuration.",
		},
		[]string{"backend", "scenario", "write_mode", "stream", "group"},
	)

	QueryRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "siem_query_requests_total",
			Help: "Total number of benchmark query executions.",
		},
		[]string{"backend", "query", "status"},
	)

	QueryDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "siem_query_duration_seconds",
			Help:    "Duration of benchmark queries.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"backend", "query"},
	)

	registered = false
)

func MustRegister() {
	if registered {
		return
	}
	registered = true

	prometheus.MustRegister(
		CollectorRequestsTotal,
		CollectorRequestDuration,
		CollectorPublishErrorsTotal,
		CollectorEventsAcceptedTotal,
		WorkerMessagesReadTotal,
		WorkerReadErrorsTotal,
		WorkerEventsStoredTotal,
		WorkerInsertErrorsTotal,
		WorkerAckErrorsTotal,
		WorkerInsertDuration,
		WorkerBatchSize,
		WorkerE2ELatency,
		WorkerQueueLatency,
		WorkerStreamLen,
		WorkerPendingMessages,
		RunInfo,
		QueryRequestsTotal,
		QueryDuration,
	)
}