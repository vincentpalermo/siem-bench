package model

import "time"

type RunResult struct {
	RunID          string    `json:"run_id"`
	Backend        string    `json:"backend"`
	EPS            int       `json:"eps"`
	BatchSize      int       `json:"batch_size"`
	DurationSec    int       `json:"duration_sec"`
	SentEvents     int       `json:"sent_events"`
	SentRequests   int       `json:"sent_requests"`
	FailedRequests int       `json:"failed_requests"`
	DBCountBefore  int64     `json:"db_count_before"`
	DBCountAfter   int64     `json:"db_count_after"`
	DBInserted     int64     `json:"db_inserted"`
	StartedAt      time.Time `json:"started_at"`
	FinishedAt     time.Time `json:"finished_at"`
}
