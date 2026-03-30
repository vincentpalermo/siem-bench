package model

import "time"

type Event struct {
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
	Raw        string    `json:"raw"`

	GeneratedAt time.Time `json:"generated_at,omitempty"`
	IngestedAt  time.Time `json:"ingested_at,omitempty"`
}