package handlers

import (
	"cronmonitor/db"
	"cronmonitor/models"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

func CreateJob(c *gin.Context) {
	userID, _ := c.Get("userID")
	var job models.Job
	if err := c.ShouldBindJSON(&job); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Generate a secure random ping key
	pingKeyBytes := make([]byte, 16)
	_, err := rand.Read(pingKeyBytes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate key"})
		return
	}
	job.PingKey = hex.EncodeToString(pingKeyBytes)

	// Insert
	err = db.GetDB().QueryRow(`
		INSERT INTO jobs (name, ping_key, schedule, timezone, grace_minutes, user_id)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at
	`, job.Name, job.PingKey, job.Schedule, job.Timezone, job.GraceMinutes, userID).Scan(&job.ID, &job.CreatedAt)

	if err != nil {
		fmt.Printf("Error creating job: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	job.PingURL = fmt.Sprintf("http://%s/ping/%s", c.Request.Host, job.PingKey)

	c.JSON(http.StatusCreated, job)
}

func ListJobs(c *gin.Context) {
	userID, _ := c.Get("userID")

	rows, err := db.GetDB().Query(`
		SELECT id, name, ping_key, schedule, timezone, grace_minutes, created_at 
		FROM jobs 
		WHERE user_id = $1
		ORDER BY created_at DESC
	`, userID)
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
	userID, _ := c.Get("userID")
	id := c.Param("id")
	var job models.Job
	err := db.GetDB().QueryRow("SELECT id, name, ping_key, schedule, timezone, grace_minutes, created_at FROM jobs WHERE id = $1 AND user_id = $2", id, userID).Scan(&job.ID, &job.Name, &job.PingKey, &job.Schedule, &job.Timezone, &job.GraceMinutes, &job.CreatedAt)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
		return
	} else if err != nil {
		fmt.Printf("GetJob DB error: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	job.PingURL = fmt.Sprintf("http://%s/ping/%s", c.Request.Host, job.PingKey)

	c.JSON(http.StatusOK, job)
}

func GetJobRuns(c *gin.Context) {
	userID, _ := c.Get("userID")
	jobID := c.Param("id")
	// Verify ownership first
	var dummyID string
	if err := db.GetDB().QueryRow("SELECT id FROM jobs WHERE id = $1 AND user_id = $2", jobID, userID).Scan(&dummyID); err != nil {
		c.Status(http.StatusNotFound)
		return
	}

	rows, err := db.GetDB().Query(`
		SELECT id, status, duration_ms, created_at 
		FROM job_runs 
		WHERE job_id = $1 
		ORDER BY created_at DESC 
		LIMIT 50
	`, jobID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	defer rows.Close()

	var runs []models.JobRun
	for rows.Next() {
		var r models.JobRun
		// metrics and stderr are not selected in the new query
		if err := rows.Scan(&r.ID, &r.Status, &r.DurationMs, &r.CreatedAt); err != nil {
			continue
		}
		runs = append(runs, r)
	}

	if runs == nil {
		runs = []models.JobRun{}
	}

	c.JSON(http.StatusOK, gin.H{"runs": runs})
}

func DeleteJob(c *gin.Context) {
	userID, _ := c.Get("userID")
	id := c.Param("id")
	result, err := db.GetDB().Exec("DELETE FROM jobs WHERE id = $1 AND user_id = $2", id, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Job deleted"})
}
