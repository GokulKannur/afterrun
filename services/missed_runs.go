package services

import (
	"cronmonitor/db"
	"cronmonitor/models"
	"database/sql"
	"fmt"
	"os"
	"time"

	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
)

func CheckForMissedRuns() {
	// Safety: Never panic
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("CheckForMissedRuns panic: %v\n", r)
		}
	}()

	fmt.Println("Running Missed Run Check...")

	conn := db.GetDB()
	rows, err := conn.Query("SELECT id, name, created_at, ping_key FROM jobs")
	if err != nil {
		fmt.Printf("Error fetching jobs for check: %v\n", err)
		return
	}
	defer rows.Close()

	// Hardcoded threshold per requirements
	threshold := 2 * time.Minute

	for rows.Next() {
		var job models.Job
		if err := rows.Scan(&job.ID, &job.Name, &job.CreatedAt, &job.PingKey); err != nil {
			fmt.Printf("Error scanning job: %v\n", err)
			continue
		}

		var lastKnownRunStr string
		var missed bool

		// Check last run elapsed time (in seconds)
		// We use Postgres to calculate the difference to avoid Timezone mismatches between Go and PG.
		var secondsSinceLastRun float64
		err := conn.QueryRow(`
			SELECT EXTRACT(EPOCH FROM (NOW() - created_at)) 
			FROM job_runs 
			WHERE job_id = $1 
			ORDER BY created_at DESC LIMIT 1
		`, job.ID).Scan(&secondsSinceLastRun)

		fmt.Printf("Checking Job: %s (ID: %s)\n", job.Name, job.ID)

		if err == sql.ErrNoRows {
			// No runs ever. Check job creation time.
			// Also calc via DB
			var secondsSinceCreation float64
			conn.QueryRow("SELECT EXTRACT(EPOCH FROM (NOW() - created_at)) FROM jobs WHERE id = $1", job.ID).Scan(&secondsSinceCreation)

			fmt.Printf("  No previous runs. CreatedAt (seconds ago): %f, Threshold: %f\n", secondsSinceCreation, threshold.Seconds())
			if secondsSinceCreation > threshold.Seconds() {
				missed = true
				lastKnownRunStr = "Never ran"
			}
		} else if err != nil {
			fmt.Printf("Error fetching last run for job %s: %v\n", job.Name, err)
			continue
		} else {
			// Has run. Check time since last run.
			fmt.Printf("  Last Run (seconds ago): %f, Threshold: %f\n", secondsSinceLastRun, threshold.Seconds())
			if secondsSinceLastRun > threshold.Seconds() {
				missed = true
				// Fetch pretty timestamp for display only
				var ts time.Time
				conn.QueryRow("SELECT created_at FROM job_runs WHERE job_id = $1 ORDER BY created_at DESC LIMIT 1", job.ID).Scan(&ts)
				lastKnownRunStr = ts.Format(time.RFC3339)
			}
		}

		if missed {
			fmt.Println("  -> Status: MISSED. Triggering alert...")
			triggerMissedRunAlert(job, lastKnownRunStr, threshold)
		} else {
			fmt.Println("  -> Status: OK")
		}
	}
}

func triggerMissedRunAlert(job models.Job, lastKnownRunStr string, threshold time.Duration) {
	conn := db.GetDB()
	alertMsg := "Job did not run within expected window"

	// Deduplication
	// Query alerts for this job with same message sent recently
	var count int
	// PG syntax: $2 is timestamp
	dedupeTime := time.Now().Add(-threshold)
	err := conn.QueryRow(`
		SELECT count(*) FROM alerts 
		WHERE job_id = $1 
		AND message = $2 
		AND sent_at > $3
	`, job.ID, alertMsg, dedupeTime).Scan(&count)

	if err == nil && count > 0 {
		fmt.Printf("  -> Duplicate alert suppressed (Count: %d). \n", count)
		return // Already alerted recently
	}

	// Insert Alert (run_id is NULL)
	_, err = conn.Exec(`
		INSERT INTO alerts (job_id, run_id, message) 
		VALUES ($1, NULL, $2)
	`, job.ID, alertMsg)

	if err != nil {
		fmt.Printf("Error inserting missed run alert: %v\n", err)
	} else {
		fmt.Printf("Missed run detected for %s. Alert saved.\n", job.Name)
	}

	// Send Email
	sendMissedRunEmail(job, lastKnownRunStr)

	// Send Slack (if configured)
	// We pass empty JobRun since there is no specific run
	go SendSlackAlert(job, models.JobRun{}, alertMsg)
}

func sendMissedRunEmail(job models.Job, lastKnownRunStr string) {
	apiKey := os.Getenv("SENDGRID_API_KEY")
	alertEmail := os.Getenv("ALERT_EMAIL")

	if apiKey == "" || alertEmail == "" {
		// Silent skip
		return
	}

	subject := fmt.Sprintf("[CRITICAL] %s did not run", job.Name)
	plainTextContent := fmt.Sprintf(`Job: %s

This job did not report any runs within the expected time window.

Last known run:
%s

This usually means:
- Cron did not execute
- Server was down
- Script failed before startup

Please investigate immediately.`, job.Name, lastKnownRunStr)

	from := mail.NewEmail("CronMonitor", alertEmail)
	to := mail.NewEmail("Admin", alertEmail)
	message := mail.NewSingleEmail(from, subject, to, plainTextContent, plainTextContent)
	client := sendgrid.NewSendClient(apiKey)

	_, err := client.Send(message)
	if err != nil {
		fmt.Printf("Error sending missed run email: %v\n", err)
	}
}
