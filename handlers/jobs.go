package handlers

import (
	"cronmonitor/db"
	"cronmonitor/models"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Helper to generate secure random string
func generatePingKey() (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	length := 16
	b := make([]byte, length)
	for i := range b {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		b[i] = charset[num.Int64()]
	}
	return string(b), nil
}

func CreateJob(c *gin.Context) {
	var req struct {
		Name         string `json:"name"`
		Schedule     string `json:"schedule"`
		Timezone     string `json:"timezone"`
		GraceMinutes int    `json:"grace_minutes"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	if req.Name == "" || len(req.Name) > 255 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Name is required and must be under 255 chars"})
		return
	}
	if req.GraceMinutes < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Grace minutes must be non-negative"})
		return
	}

	// Generate Key with retry on collision
	var pingKey string
	for {
		key, err := generatePingKey()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate key"})
			return
		}
		// Check uniqueness
		var exists int
		err = db.GetDB().QueryRow("SELECT 1 FROM jobs WHERE ping_key = $1", key).Scan(&exists)
		if err == sql.ErrNoRows {
			pingKey = key
			break
		}
	}

	// Insert
	var jobID string
	var createdAt string
	err := db.GetDB().QueryRow(`
		INSERT INTO jobs (name, ping_key, schedule, timezone, grace_minutes)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at
	`, req.Name, pingKey, req.Schedule, req.Timezone, req.GraceMinutes).Scan(&jobID, &createdAt)

	if err != nil {
		fmt.Printf("Error creating job: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create job"})
		return
	}

	pingURL := fmt.Sprintf("http://%s/ping/%s", c.Request.Host, pingKey)

	c.JSON(http.StatusCreated, gin.H{
		"id":            jobID,
		"name":          req.Name,
		"ping_key":      pingKey,
		"ping_url":      pingURL,
		"schedule":      req.Schedule,
		"timezone":      req.Timezone,
		"grace_minutes": req.GraceMinutes,
		"created_at":    createdAt,
	})
}

func ListJobs(c *gin.Context) {
	rows, err := db.GetDB().Query(`
		SELECT id, name, ping_key, schedule, timezone, grace_minutes, created_at 
		FROM jobs 
		ORDER BY created_at DESC
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	defer rows.Close()

	var jobs []models.Job
	for rows.Next() {
		var j models.Job
		// Handle simple fields
		if err := rows.Scan(&j.ID, &j.Name, &j.PingKey, &j.Schedule, &j.Timezone, &j.GraceMinutes, &j.CreatedAt); err != nil {
			continue
		}

		// Fill last run
		// Requirement: "If no runs exist, last_run = null"
		var lastRun models.JobRun
		err := db.GetDB().QueryRow(`
			SELECT status, created_at 
			FROM job_runs 
			WHERE job_id = $1 
			ORDER BY created_at DESC 
			LIMIT 1
		`, j.ID).Scan(&lastRun.Status, &lastRun.CreatedAt)

		if err == nil {
			j.LastRun = &models.JobRun{
				Status:    lastRun.Status,
				CreatedAt: lastRun.CreatedAt,
			}
		}

		j.PingURL = fmt.Sprintf("http://%s/ping/%s", c.Request.Host, j.PingKey)

		jobs = append(jobs, j)
	}

	if jobs == nil {
		jobs = []models.Job{}
	}

	c.JSON(http.StatusOK, gin.H{"jobs": jobs})
}

func GetJob(c *gin.Context) {
	id := c.Param("id")
	var j models.Job

	// Use COALESCE to handle potential nulls if DB has them, though our insert sets values.
	err := db.GetDB().QueryRow(`
		SELECT id, name, ping_key, COALESCE(schedule, ''), COALESCE(timezone, 'UTC'), COALESCE(grace_minutes, 30), created_at 
		FROM jobs WHERE id = $1
	`, id).Scan(&j.ID, &j.Name, &j.PingKey, &j.Schedule, &j.Timezone, &j.GraceMinutes, &j.CreatedAt)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
		return
	} else if err != nil {
		fmt.Printf("GetJob DB error: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	j.PingURL = fmt.Sprintf("http://%s/ping/%s", c.Request.Host, j.PingKey)

	c.JSON(http.StatusOK, j)
}

func GetJobRuns(c *gin.Context) {
	id := c.Param("id")

	rows, err := db.GetDB().Query(`
		SELECT id, status, duration_ms, metrics, stderr, created_at 
		FROM job_runs 
		WHERE job_id = $1 
		ORDER BY created_at DESC 
		LIMIT 100
	`, id)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	defer rows.Close()

	var runs []models.JobRun
	for rows.Next() {
		var r models.JobRun
		var metricsRaw []byte // metrics is JSONB
		if err := rows.Scan(&r.ID, &r.Status, &r.DurationMs, &metricsRaw, &r.Stderr, &r.CreatedAt); err != nil {
			continue
		}
		// Unmarshal metrics bytes to map
		if len(metricsRaw) > 0 {
			_ = json.Unmarshal(metricsRaw, &r.Metrics) // Ignore error, best effort
		}
		runs = append(runs, r)
	}

	if runs == nil {
		runs = []models.JobRun{}
	}

	c.JSON(http.StatusOK, gin.H{"runs": runs})
}

func DeleteJob(c *gin.Context) {
	id := c.Param("id")
	res, err := db.GetDB().Exec("DELETE FROM jobs WHERE id = $1", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Job deleted"})
}
