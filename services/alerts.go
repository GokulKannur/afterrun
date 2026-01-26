package services

import (
	"cronmonitor/db"
	"cronmonitor/models"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
)

func SendAlert(job models.Job, run models.JobRun, rule models.Rule, actualValue float64, stderr string) {
	// 1. Prepare Alert Message
	alertMessage := fmt.Sprintf("%s %s %f (actual: %f)",
		rule.MetricName, rule.Operator, rule.ThresholdValue, actualValue)

	if run.ID == "" {
		fmt.Println("Error: run.ID is empty, cannot save alert")
		return
	}

	// 2. Write alert to DB FIRST (Source of Truth)
	_, err := db.GetDB().Exec(`
		INSERT INTO alerts (job_id, run_id, message)
		VALUES ($1, $2, $3)
	`, job.ID, run.ID, alertMessage)
	if err != nil {
		fmt.Printf("Error saving alert: %v\n", err)
		// We continue to try sending email even if DB fails?
		// User said "DB = truth, Email = best effort".
		// If DB fails, we probably shouldn't crash, but user didn't specify return.
		// I will return to be safe/consistent with "DB is truth", but user didn't explicitly say "Stop if DB fails".
		// Actually user code snippet has `return` on error!
		return
	}
	fmt.Println("Alert validated & saved to DB.")

	// Fire-and-forget Slack alert (Non-blocking)
	go SendSlackAlert(job, run, alertMessage)

	// 3. Check Email Config
	apiKey := os.Getenv("SENDGRID_API_KEY")
	alertEmail := os.Getenv("ALERT_EMAIL")

	if apiKey == "" || alertEmail == "" {
		fmt.Println("Missing SendGrid config, skipping email")
		return
	}

	// 4. Gather Context (Last Successful Run)
	var lastStatusInfo string

	var lastRunTime time.Time
	var lastDuration int
	var lastMetricsRaw []byte

	err = db.GetDB().QueryRow(`
		SELECT created_at, duration_ms, metrics 
		FROM job_runs 
		WHERE job_id = $1 AND status = 'ok' AND created_at < $2 
		ORDER BY created_at DESC LIMIT 1
	`, job.ID, run.CreatedAt).Scan(&lastRunTime, &lastDuration, &lastMetricsRaw)

	if err != nil {
		lastStatusInfo = "None found (this job has never succeeded)"
	} else {
		// Pretty print last metrics
		// Create a temporary map to marshal/unmarshal for pretty printing if needed, or just print string
		// But JSONB comes out as bytes.
		lastStatusInfo = fmt.Sprintf("Time: %s\nDuration: %dms\nMetrics: %s",
			lastRunTime.Format(time.RFC3339),
			lastDuration,
			string(lastMetricsRaw))
	}

	// 5. Construct Email Content
	// Subject Logic
	var subject string
	if run.Status == "fail" {
		subject = fmt.Sprintf("[CRITICAL] %s failed", job.Name)
	} else {
		subject = fmt.Sprintf("[WARNING] %s ran but produced suspicious output", job.Name)
	}
	// Append metric context if short enough
	if len(subject)+len(rule.MetricName)+10 < 80 {
		subject += fmt.Sprintf(" (%s)", rule.MetricName)
	}

	// Human Explanation
	explanation := fmt.Sprintf("This job ran successfully, but the output indicates a problem.\nIt returned %s %f, while your rule expects %s %s %f.",
		rule.MetricName, actualValue, rule.MetricName, rule.Operator, rule.ThresholdValue)
	if run.Status == "fail" {
		explanation = fmt.Sprintf("The job failed to complete successfully (Status: %s).", run.Status)
	}

	metricsJSON, _ := json.MarshalIndent(run.Metrics, "", "  ")

	plainTextContent := fmt.Sprintf(`%s

%s

JOB SUMMARY:
Job: %s
Status: %s
Time: %s

WHAT WENT WRONG:
Rule violated: %s
Actual value: %f

LAST SUCCESSFUL RUN:
%s

CURRENT RUN METRICS:
Duration: %dms
Metrics: %s

STDERR:
%s

---
Run ID: %s`,
		subject, // 1. Subject header in body
		explanation,
		job.Name,
		run.Status,
		run.CreatedAt.Format(time.RFC3339),
		alertMessage,
		actualValue,
		lastStatusInfo,
		run.DurationMs,
		string(metricsJSON),
		stderr,
		run.ID,
	)

	from := mail.NewEmail("CronMonitor", alertEmail)
	to := mail.NewEmail("Admin", alertEmail)
	message := mail.NewSingleEmail(from, subject, to, plainTextContent, plainTextContent)
	client := sendgrid.NewSendClient(apiKey)

	response, err := client.Send(message)
	if err != nil {
		fmt.Printf("Error sending email: %v\n", err)
	} else {
		fmt.Printf("Email sent. Status Code: %d\n", response.StatusCode)
	}
}
