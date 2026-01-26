package handlers

import (
	"cronmonitor/db"
	"cronmonitor/models"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

func ShowJobs(c *gin.Context) {
	rows, err := db.GetDB().Query(`
		SELECT id, name, ping_key, created_at 
		FROM jobs 
		ORDER BY created_at DESC
	`)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "layout.html", gin.H{"error": "Database error"})
		return
	}
	defer rows.Close()

	var jobs []models.Job
	for rows.Next() {
		var j models.Job
		if err := rows.Scan(&j.ID, &j.Name, &j.PingKey, &j.CreatedAt); err != nil {
			continue
		}

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

	c.HTML(http.StatusOK, "jobs.html", gin.H{
		"Title": "Jobs",
		"Jobs":  jobs,
	})
}

func ShowJobDetail(c *gin.Context) {
	id := c.Param("id")
	var job models.Job

	err := db.GetDB().QueryRow(`
		SELECT id, name, ping_key, COALESCE(schedule, ''), COALESCE(timezone, 'UTC'), COALESCE(grace_minutes, 30), created_at 
		FROM jobs WHERE id = $1
	`, id).Scan(&job.ID, &job.Name, &job.PingKey, &job.Schedule, &job.Timezone, &job.GraceMinutes, &job.CreatedAt)

	if err == sql.ErrNoRows {
		c.String(http.StatusNotFound, "Job not found")
		return
	} else if err != nil {
		c.String(http.StatusInternalServerError, "Database error")
		return
	}

	job.PingURL = fmt.Sprintf("http://%s/ping/%s", c.Request.Host, job.PingKey)

	// Fetch History
	rows, err := db.GetDB().Query(`
		SELECT status, duration_ms, metrics, stderr, created_at 
		FROM job_runs 
		WHERE job_id = $1 
		ORDER BY created_at DESC 
		LIMIT 10
	`, id)

	var runs []models.JobRun
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var r models.JobRun
			var metricsRaw []byte
			if err := rows.Scan(&r.Status, &r.DurationMs, &metricsRaw, &r.Stderr, &r.CreatedAt); err == nil {
				if len(metricsRaw) > 0 {
					_ = json.Unmarshal(metricsRaw, &r.Metrics)
				}
				runs = append(runs, r)
			}
		}
	}

	c.HTML(http.StatusOK, "job_detail.html", gin.H{
		"Title": job.Name,
		"Job":   job,
		"Runs":  runs,
	})
}
