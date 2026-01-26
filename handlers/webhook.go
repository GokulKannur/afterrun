package handlers

import (
	"cronmonitor/db"
	"cronmonitor/models"
	"cronmonitor/services"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

type PingRequest struct {
	Status     string                 `json:"status"`
	DurationMs int                    `json:"duration_ms"`
	Metrics    map[string]interface{} `json:"metrics"`
	Stderr     string                 `json:"stderr"`
}

func PingHandler(c *gin.Context) {
	fmt.Println("HANDLER v2: Received ping")
	pingKey := c.Param("ping_key")

	var job models.Job
	err := db.GetDB().QueryRow("SELECT id, name, ping_key FROM jobs WHERE ping_key = $1", pingKey).Scan(&job.ID, &job.Name, &job.PingKey)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	// Phase 1.4: Idempotency Check
	// Prevent duplicate pings within 10 seconds of the last run
	var dummy int
	err = db.GetDB().QueryRow(`
		SELECT 1 FROM job_runs 
		WHERE job_id = $1 AND created_at > NOW() - INTERVAL '10 seconds' 
		LIMIT 1
	`, job.ID).Scan(&dummy)

	if err == nil {
		fmt.Println("Duplicate ping suppressed")
		c.Status(http.StatusOK)
		return
	} else if err != sql.ErrNoRows {
		// Log error but continue (fail-open)
		fmt.Printf("Error checking for duplicates: %v\n", err)
	}

	var req PingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	metricsJSON, _ := json.Marshal(req.Metrics)

	// Insert Job Run
	var runID string
	err = db.GetDB().QueryRow(`
		INSERT INTO job_runs (job_id, status, duration_ms, metrics, stderr)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`, job.ID, req.Status, req.DurationMs, metricsJSON, req.Stderr).Scan(&runID)

	if err != nil {
		fmt.Printf("Error saving run: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save run"})
		return
	}

	run := models.JobRun{
		ID:         runID,
		JobID:      job.ID,
		Status:     req.Status,
		DurationMs: req.DurationMs,
		Metrics:    req.Metrics,
		Stderr:     req.Stderr,
	}

	// Verify Rules
	go func() {
		rows, err := db.GetDB().Query("SELECT id, metric_name, operator, threshold_value FROM rules WHERE job_id = $1", job.ID)
		if err != nil {
			fmt.Printf("Error fetching rules: %v\n", err)
			return
		}
		defer rows.Close()

		for rows.Next() {
			var rule models.Rule
			if err := rows.Scan(&rule.ID, &rule.MetricName, &rule.Operator, &rule.ThresholdValue); err != nil {
				continue
			}
			rule.JobID = job.ID

			violated, val := services.EvaluateRule(req.Metrics, rule)
			if violated {
				services.SendAlert(job, run, rule, val, req.Stderr)
			}
		}
	}()

	c.Status(http.StatusOK)
}
