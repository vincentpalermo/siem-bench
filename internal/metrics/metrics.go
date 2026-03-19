package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	CollectorRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "siem_collector_requests_total",
			Help: "Total number of collector HTTP requests",
		},
		[]string{"endpoint", "method", "status"},
	)

	CollectorEventsAcceptedTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "siem_collector_events_accepted_total",
			Help: "Total number of events accepted by collector",
		},
	)

	CollectorPublishErrorsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "siem_collector_publish_errors_total",
			Help: "Total number of Redis publish errors in collector",
		},
	)

	CollectorRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "siem_collector_request_duration_seconds",
			Help:    "Collector request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"endpoint"},
	)

	WorkerMessagesReadTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "siem_worker_messages_read_total",
			Help: "Total number of messages read by worker from Redis",
		},
	)

	WorkerEventsStoredTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "siem_worker_events_stored_total",
			Help: "Total number of events stored by worker into PostgreSQL",
		},
	)

	WorkerInsertErrorsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "siem_worker_insert_errors_total",
			Help: "Total number of PostgreSQL insert errors in worker",
		},
	)

	WorkerReadErrorsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "siem_worker_read_errors_total",
			Help: "Total number of Redis read errors in worker",
		},
	)

	WorkerAckErrorsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "siem_worker_ack_errors_total",
			Help: "Total number of Redis ack errors in worker",
		},
	)

	WorkerBatchSize = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "siem_worker_batch_size",
			Help:    "Batch size processed by worker",
			Buckets: []float64{1, 5, 10, 20, 50, 100, 200, 500},
		},
	)

	WorkerInsertDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "siem_worker_insert_duration_seconds",
			Help:    "Time spent inserting events into PostgreSQL",
			Buckets: prometheus.DefBuckets,
		},
	)
)

func MustRegister() {
	prometheus.MustRegister(
		CollectorRequestsTotal,
		CollectorEventsAcceptedTotal,
		CollectorPublishErrorsTotal,
		CollectorRequestDuration,
		WorkerMessagesReadTotal,
		WorkerEventsStoredTotal,
		WorkerInsertErrorsTotal,
		WorkerReadErrorsTotal,
		WorkerAckErrorsTotal,
		WorkerBatchSize,
		WorkerInsertDuration,
	)
}
