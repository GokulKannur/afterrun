package handlers

import (
	"cronmonitor/config"
	"cronmonitor/db"
	"cronmonitor/models"
	"cronmonitor/services"
	"database/sql"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

func ShowLogin(c *gin.Context) {
	c.HTML(http.StatusOK, "login.html", gin.H{})
}

func ShowSignup(c *gin.Context) {
	c.HTML(http.StatusOK, "signup.html", gin.H{})
}

func ShowJobs(c *gin.Context) {
	userID, exists := c.Get("userID")
	userEmail, _ := c.Get("userEmail") // for header

	if !exists {
		// Should be handled by middleware redirection if protected,
		// but if middleware just passed (system user), it works.
		// If real auth failure, middleware would have aborted.
		// Wait, middleware sets "userID" even for system fallback.
	}

	rows, err := db.GetDB().Query(`
		SELECT id, name, ping_key, schedule, timezone, grace_minutes, created_at 
		FROM jobs 
		WHERE user_id = $1
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "layout.html", gin.H{"error": "Database error"})
		return
	}
	defer rows.Close()

	var jobs []models.Job
	for rows.Next() {
		var j models.Job
		// Fix Mismatch: Scan all 7 selected columns
		if err := rows.Scan(&j.ID, &j.Name, &j.PingKey, &j.Schedule, &j.Timezone, &j.GraceMinutes, &j.CreatedAt); err != nil {
			fmt.Println("Scan error:", err) // Debug log
			continue
		}

		// Calculate Ping URL since it's used in the modal now
		j.PingURL = fmt.Sprintf("http://%s/ping/%s", c.Request.Host, j.PingKey)

		// N+1 for Last Run (Acceptable for MVP UI)
		var lastRun models.JobRun
		err := db.GetDB().QueryRow(`
			SELECT status, duration_ms, created_at 
			FROM job_runs 
			WHERE job_id = $1 
			ORDER BY created_at DESC 
			LIMIT 1
		`, j.ID).Scan(&lastRun.Status, &lastRun.DurationMs, &lastRun.CreatedAt)

		if err == nil {
			j.LastRun = &models.JobRun{
				Status:     lastRun.Status,
				DurationMs: lastRun.DurationMs,
				CreatedAt:  lastRun.CreatedAt,
			}
		}

		jobs = append(jobs, j)
	}

	// Billing Info
	var tier string
	if err := db.GetDB().QueryRow("SELECT COALESCE(subscription_tier, 'free') FROM users WHERE id = $1", userID).Scan(&tier); err != nil {
		tier = "free"
	}

	count := len(jobs) // We just fetched them, so Count = len(jobs) is accurate for the view
	limit := services.GetJobLimit(tier)

	features := config.LoadFeatures()

	c.HTML(http.StatusOK, "jobs.html", gin.H{
		"Title":          "Jobs",
		"Jobs":           jobs,
		"UserEmail":      userEmail,
		"Tier":           tier,
		"JobCount":       count,
		"JobLimit":       limit,
		"WriteUIEnabled": features.WriteUIEnabled,
	})
}

func ShowJobDetail(c *gin.Context) {
	id := c.Param("id")
	var job models.Job

	userID, _ := c.Get("userID")
	userEmail, _ := c.Get("userEmail")

	err := db.GetDB().QueryRow("SELECT id, name, ping_key, COALESCE(schedule, ''), COALESCE(timezone, 'UTC'), COALESCE(grace_minutes, 30), created_at FROM jobs WHERE id = $1 AND user_id = $2", id, userID).Scan(&job.ID, &job.Name, &job.PingKey, &job.Schedule, &job.Timezone, &job.GraceMinutes, &job.CreatedAt)

	if err == sql.ErrNoRows {
		c.HTML(http.StatusNotFound, "error.html", gin.H{"error": "Job not found"})
		return
	} else if err != nil {
		c.String(http.StatusInternalServerError, "Database error")
		return
	}

	job.PingURL = fmt.Sprintf("http://%s/ping/%s", c.Request.Host, job.PingKey)

	// Fetch Runs (Limit 50)
	rows, err := db.GetDB().Query("SELECT id, status, duration_ms, created_at FROM job_runs WHERE job_id = $1 ORDER BY created_at DESC LIMIT 50", id)
	if err != nil {
		fmt.Printf("Error fetching runs: %v\n", err)
	} else {
		defer rows.Close()
		for rows.Next() {
			var r models.JobRun
			if err := rows.Scan(&r.ID, &r.Status, &r.DurationMs, &r.CreatedAt); err == nil {
				job.JobRuns = append(job.JobRuns, r)
			}
		}
	}

	features := config.LoadFeatures()

	c.HTML(http.StatusOK, "job_detail.html", gin.H{
		"Job":            job,
		"UserEmail":      userEmail,
		"Runs":           job.JobRuns,
		"WriteUIEnabled": features.WriteUIEnabled,
	})
}
