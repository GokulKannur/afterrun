package handlers

import (
	"cronmonitor/db"
	"cronmonitor/models"
	"net/http"

	"github.com/gin-gonic/gin"
)

func CreateRule(c *gin.Context) {
	jobID := c.Param("id")

	// Verify job exists (Foreign key constraint handles this usually, but good for validation msg)
	// We'll rely on DB constraint for MVP speed, or check?
	// Prompt says "Handle all errors gracefully". DB FK error is fine, but checking is nicer.

	var req struct {
		MetricName     string  `json:"metric_name"`
		Operator       string  `json:"operator"`
		ThresholdValue float64 `json:"threshold_value"`
		Severity       string  `json:"severity"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	if req.MetricName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "metric_name is required"})
		return
	}

	validOps := map[string]bool{"==": true, "!=": true, "<": true, ">": true}
	if !validOps[req.Operator] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid operator. Must be ==, !=, <, >"})
		return
	}

	validSev := map[string]bool{"warning": true, "critical": true}
	if !validSev[req.Severity] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid severity. Must be warning or critical"})
		return
	}

	var ruleID string
	var createdAt string
	err := db.GetDB().QueryRow(`
		INSERT INTO rules (job_id, metric_name, operator, threshold_value, severity)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at
	`, jobID, req.MetricName, req.Operator, req.ThresholdValue, req.Severity).Scan(&ruleID, &createdAt)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create rule (Job might not exist)"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":              ruleID,
		"job_id":          jobID,
		"metric_name":     req.MetricName,
		"operator":        req.Operator,
		"threshold_value": req.ThresholdValue,
		"severity":        req.Severity,
		"created_at":      createdAt,
	})
}

func ListRules(c *gin.Context) {
	jobID := c.Param("id")

	rows, err := db.GetDB().Query(`
		SELECT id, metric_name, operator, threshold_value, severity, created_at 
		FROM rules 
		WHERE job_id = $1
	`, jobID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	defer rows.Close()

	var rules []models.Rule
	for rows.Next() {
		var r models.Rule
		if err := rows.Scan(&r.ID, &r.MetricName, &r.Operator, &r.ThresholdValue, &r.Severity, &r.CreatedAt); err != nil {
			continue
		}
		// Since we didn't scan job_id, set it manually if needed, or omit. Model has JobID.
		r.JobID = jobID
		rules = append(rules, r)
	}

	if rules == nil {
		rules = []models.Rule{}
	}

	c.JSON(http.StatusOK, gin.H{"rules": rules})
}

func DeleteRule(c *gin.Context) {
	id := c.Param("id")
	res, err := db.GetDB().Exec("DELETE FROM rules WHERE id = $1", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Rule not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Rule deleted"})
}
