package model

import "time"

type RunConfigSnapshot struct {
	CollectorURL    string `json:"collector_url"`
	WorkerBackend   string `json:"worker_backend"`
	WorkerWriteMode string `json:"worker_write_mode"`
	RunScenario     string `json:"run_scenario"`
	GeneratorEPS    int    `json:"generator_eps"`
	GeneratorBatch  int    `json:"generator_batch"`
	GeneratorSec    int    `json:"generator_sec"`
}

type RunResult struct {
	RunID          string            `json:"run_id"`
	Backend        string            `json:"backend"`
	SentEvents     int               `json:"sent_events"`
	SentRequests   int               `json:"sent_requests"`
	FailedRequests int               `json:"failed_requests"`
	DBCountBefore  int64             `json:"db_count_before"`
	DBCountAfter   int64             `json:"db_count_after"`
	DBInserted     int64             `json:"db_inserted"`
	GeneratorSentEPS   float64 		 `json:"generator_sent_eps"`
	StorageEffectiveEPS float64  	 `json:"storage_effective_eps"`
	SendElapsedSec     float64 		 `json:"send_elapsed_sec"`
	TotalElapsedSec    float64 		 `json:"total_elapsed_sec"`
	DrainWaitSec       float64 		 `json:"drain_wait_sec"`
	LossPercent        float64 		 `json:"loss_percent"`
	Notes          string            `json:"notes,omitempty"`
	ConfigSnapshot RunConfigSnapshot `json:"config_snapshot"`
	StartedAt      time.Time         `json:"started_at"`
	FinishedAt     time.Time         `json:"finished_at"`

	StreamLenAtSendFinish int64   `json:"stream_len_at_send_finish"`
	PendingAtSendFinish   int64   `json:"pending_at_send_finish"`
	DBCountAtSendFinish   int64   `json:"db_count_at_send_finish"`

	StreamLenAtFinish int64 `json:"stream_len_at_finish"`
	PendingAtFinish   int64 `json:"pending_at_finish"`

	E2ELatencyAvgMs   float64 `json:"e2e_latency_avg_ms"`
	E2ELatencyP95Ms   float64 `json:"e2e_latency_p95_ms"`
	E2ELatencyP99Ms   float64 `json:"e2e_latency_p99_ms"`
	QueueLatencyAvgMs float64 `json:"queue_latency_avg_ms"`
	QueueLatencyP95Ms float64 `json:"queue_latency_p95_ms"`
	QueueLatencyP99Ms float64 `json:"queue_latency_p99_ms"`

	SystemCPUAvgPercent    float64 `json:"system_cpu_avg_percent"`
	SystemCPUMaxPercent    float64 `json:"system_cpu_max_percent"`
	SystemMemoryAvgMB      float64 `json:"system_memory_avg_mb"`
	SystemMemoryMaxMB      float64 `json:"system_memory_max_mb"`
	SystemDiskReadMB       float64 `json:"system_disk_read_mb"`
	SystemDiskWriteMB      float64 `json:"system_disk_write_mb"`
	SystemNetRxMB          float64 `json:"system_net_rx_mb"`
	SystemNetTxMB          float64 `json:"system_net_tx_mb"`
}