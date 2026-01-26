package models

import (
	"time"
)

type Job struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	PingKey   string    `json:"ping_key"`
	CreatedAt time.Time `json:"created_at"`
}

type JobRun struct {
	ID         string                 `json:"id"`
	JobID      string                 `json:"job_id"`
	Status     string                 `json:"status"`
	DurationMs int                    `json:"duration_ms"`
	Metrics    map[string]interface{} `json:"metrics"`
	Stderr     string                 `json:"stderr"`
	CreatedAt  time.Time              `json:"created_at"`
}

type Rule struct {
	ID             string  `json:"id"`
	JobID          string  `json:"job_id"`
	MetricName     string  `json:"metric_name"`
	Operator       string  `json:"operator"`
	ThresholdValue float64 `json:"threshold_value"`
	Severity       string  `json:"severity"`
}

type Alert struct {
	ID      string    `json:"id"`
	JobID   string    `json:"job_id"`
	RunID   string    `json:"run_id"`
	Message string    `json:"message"`
	SentAt  time.Time `json:"sent_at"`
}
