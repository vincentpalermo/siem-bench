package model

import "time"

type QueryStat struct {
	Name          string  `json:"name"`
	Count         int     `json:"count"`
	Failed        int     `json:"failed"`
	AvgDurationMs float64 `json:"avg_duration_ms"`
	MaxDurationMs float64 `json:"max_duration_ms"`
}

type QueryRunResult struct {
	RunID         string      `json:"run_id"`
	Backend       string      `json:"backend"`
	DurationSec   int         `json:"duration_sec"`
	TotalQueries  int         `json:"total_queries"`
	FailedQueries int         `json:"failed_queries"`
	StartedAt     time.Time   `json:"started_at"`
	FinishedAt    time.Time   `json:"finished_at"`
	Queries       []QueryStat `json:"queries"`
}
