package model

import "time"

type QueryConfigSnapshot struct {
	Backend       string `json:"backend"`
	DurationSec   int    `json:"duration_sec"`
	IntervalSec   int    `json:"interval_sec"`
	WarmupSec     int    `json:"warmup_sec"`
	Concurrency   int    `json:"concurrency"`
	RunScenario   string `json:"run_scenario"`
	WorkloadName  string `json:"workload_name"`
	WorkloadPath  string `json:"workload_path"`
}

type QueryWorkloadItem struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Value   string `json:"value,omitempty"`
	Limit   int    `json:"limit,omitempty"`
	Enabled bool   `json:"enabled"`
}

type QueryStat struct {
	Name           string  `json:"name"`
	Count          int     `json:"count"`
	Failed         int     `json:"failed"`
	AvgDurationMs  float64 `json:"avg_duration_ms"`
	MaxDurationMs  float64 `json:"max_duration_ms"`
	MinDurationMs  float64 `json:"min_duration_ms"`
	LastDurationMs float64 `json:"last_duration_ms"`
}

type QueryRunResult struct {
	RunID          string              `json:"run_id"`
	Backend        string              `json:"backend"`
	TotalQueries   int                 `json:"total_queries"`
	FailedQueries  int                 `json:"failed_queries"`
	Notes          string              `json:"notes,omitempty"`
	ConfigSnapshot QueryConfigSnapshot `json:"config_snapshot"`
	Workload       []QueryWorkloadItem `json:"workload"`
	StartedAt      time.Time           `json:"started_at"`
	FinishedAt     time.Time           `json:"finished_at"`
	Queries        []QueryStat         `json:"queries"`
}