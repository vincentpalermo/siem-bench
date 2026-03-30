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
}