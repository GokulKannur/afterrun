package handlers

import (
	"cronmonitor/db"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Read-only overview stats
func GetStatsOverview(c *gin.Context) {
	var stats struct {
		TotalJobs     int     `json:"total_jobs"`
		TotalRuns     int     `json:"total_runs"`
		TotalAlerts   int     `json:"total_alerts"`
		SuccessRuns   int     `json:"success_runs"`
		SuccessRate   float64 `json:"success_rate"`
		AvgDurationMs float64 `json:"avg_duration_ms"`
	}

	dbConn := db.GetDB()
	userID, _ := c.Get("userID")

	// 1. Counts
	_ = dbConn.QueryRow("SELECT COUNT(*) FROM jobs WHERE user_id = $1", userID).Scan(&stats.TotalJobs)
	_ = dbConn.QueryRow("SELECT COUNT(*) FROM job_runs WHERE job_id IN (SELECT id FROM jobs WHERE user_id = $1)", userID).Scan(&stats.TotalRuns)
	_ = dbConn.QueryRow("SELECT COUNT(*) FROM alerts WHERE job_id IN (SELECT id FROM jobs WHERE user_id = $1)", userID).Scan(&stats.TotalAlerts)

	// 2. Success Runs
	_ = dbConn.QueryRow("SELECT COUNT(*) FROM job_runs WHERE status = 'ok' AND job_id IN (SELECT id FROM jobs WHERE user_id = $1)", userID).Scan(&stats.SuccessRuns)

	// 3. Avg Duration (handle NULL if no runs)
	var avgDuration *float64
	_ = dbConn.QueryRow("SELECT AVG(duration_ms) FROM job_runs WHERE duration_ms IS NOT NULL AND job_id IN (SELECT id FROM jobs WHERE user_id = $1)", userID).Scan(&avgDuration)
	if avgDuration != nil {
		stats.AvgDurationMs = *avgDuration
	}

	// 4. Calculate Success Rate (Divide by Zero check)
	if stats.TotalRuns > 0 {
		stats.SuccessRate = (float64(stats.SuccessRuns) / float64(stats.TotalRuns)) * 100
	} else {
		stats.SuccessRate = 0.0
	}

	c.JSON(http.StatusOK, stats)
}

// Read-only job stats
func GetJobStats(c *gin.Context) {
	jobID := c.Param("id")
	userID, _ := c.Get("userID")

	dbConn := db.GetDB()

	// Verify Ownership
	var dummyID string
	if err := dbConn.QueryRow("SELECT id FROM jobs WHERE id = $1 AND user_id = $2", jobID, userID).Scan(&dummyID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
		return
	}

	var stats struct {
		RunCount      int     `json:"run_count"`
		SuccessCount  int     `json:"success_count"`
		FailureCount  int     `json:"failure_count"`
		AvgDurationMs float64 `json:"avg_duration_ms"`
		P50DurationMs float64 `json:"p50_duration_ms"` // Ad-hoc percentile
		P95DurationMs float64 `json:"p95_duration_ms"` // Ad-hoc percentile
	}

	// dbConn := db.GetDB() // Already declared above

	// 1. Basic Counts
	_ = dbConn.QueryRow("SELECT COUNT(*) FROM job_runs WHERE job_id = $1", jobID).Scan(&stats.RunCount)
	if stats.RunCount == 0 {
		// Graceful exit for empty jobs
		c.JSON(http.StatusOK, stats)
		return
	}

	_ = dbConn.QueryRow("SELECT COUNT(*) FROM job_runs WHERE job_id = $1 AND status = 'ok'", jobID).Scan(&stats.SuccessCount)
	stats.FailureCount = stats.RunCount - stats.SuccessCount

	// 2. Avg Duration
	var avg *float64
	_ = dbConn.QueryRow("SELECT AVG(duration_ms) FROM job_runs WHERE job_id = $1", jobID).Scan(&avg)
	if avg != nil {
		stats.AvgDurationMs = *avg
	}

	// 3. Percentiles (Ad-hoc Query)
	// NOTE: Percentiles are computed from historical job_runs at query time.
	// These are NOT baselines yet. Baselines will be precomputed in Phase 8.

	// We use Postgres PERCENTILE_CONT if available (PG 9.4+), simplified here for standard SQL or simple averaging if needed.
	// Docker uses PG 15, so PERCENTILE_CONT is safe.
	_ = dbConn.QueryRow(`
		SELECT 
			PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY duration_ms),
			PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY duration_ms)
		FROM job_runs 
		WHERE job_id = $1 AND duration_ms IS NOT NULL
	`, jobID).Scan(&stats.P50DurationMs, &stats.P95DurationMs)

	c.JSON(http.StatusOK, stats)
}
