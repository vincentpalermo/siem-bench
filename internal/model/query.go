package model

import "time"

type EventQueryResult struct {
	ID         string    `json:"id"`
	Timestamp  time.Time `json:"timestamp"`
	SourceType string    `json:"source_type"`
	Host       string    `json:"host"`
	UserName   string    `json:"user_name"`
	SrcIP      string    `json:"src_ip"`
	DstIP      string    `json:"dst_ip"`
	EventCode  string    `json:"event_code"`
	Severity   int       `json:"severity"`
	Message    string    `json:"message"`
}

type SeverityCount struct {
	Severity int   `json:"severity"`
	Count    int64 `json:"count"`
}

type HostCount struct {
	Host  string `json:"host"`
	Count int64  `json:"count"`
}
