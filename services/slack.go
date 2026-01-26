package services

import (
	"bytes"
	"cronmonitor/models"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

func SendSlackAlert(job models.Job, run models.JobRun, message string) {
	// Safety: Recover from any panic to avoid crashing the worker
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Slack panic recovered: %v\n", r)
		}
	}()

	webhookURL := os.Getenv("SLACK_WEBHOOK_URL")
	if webhookURL == "" {
		fmt.Println("Slack skipped: SLACK_WEBHOOK_URL not set")
		return
	}

	payload := map[string]string{
		"text": fmt.Sprintf("🚨 Cron Alert\n\nJob: %s\nStatus: %s\nTime: %s\n\nIssue:\n%s\n\nRun ID: %s",
			job.Name,
			run.Status,
			run.CreatedAt,
			message,
			run.ID,
		),
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		fmt.Printf("Error marshaling Slack payload: %v\n", err)
		return
	}

	resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		fmt.Printf("Error sending Slack request: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		fmt.Printf("Slack API error: Status %d\n", resp.StatusCode)
	} else {
		fmt.Println("Slack alert sent successfully")
	}
}
