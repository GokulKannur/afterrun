package handlers

import (
	"cronmonitor/db"
	"cronmonitor/models"
	"net/http"

	"github.com/gin-gonic/gin"
)

func CreateRule(c *gin.Context) {
	userID, _ := c.Get("userID")
	jobID := c.Param("id")

	// Verify Ownership
	var dummyID string
	if err := db.GetDB().QueryRow("SELECT id FROM jobs WHERE id = $1 AND user_id = $2", jobID, userID).Scan(&dummyID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
		return
	}

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

	var ruleID string
	var createdAt string
	err := db.GetDB().QueryRow(`
		INSERT INTO rules (job_id, metric_name, operator, threshold_value, severity)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at
	`, jobID, req.MetricName, req.Operator, req.ThresholdValue, req.Severity).Scan(&ruleID, &createdAt)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create rule (Job might not exist or DB error)"})
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
	userID, _ := c.Get("userID")
	jobID := c.Param("id")

	// Verify Ownership
	var dummyID string
	if err := db.GetDB().QueryRow("SELECT id FROM jobs WHERE id = $1 AND user_id = $2", jobID, userID).Scan(&dummyID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
		return
	}

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
		rules = append(rules, r)
	}

	if rules == nil {
		rules = []models.Rule{}
	}

	c.JSON(http.StatusOK, gin.H{"rules": rules})
}

func DeleteRule(c *gin.Context) {
	userID, _ := c.Get("userID")
	id := c.Param("id")

	// Verify Ownership via Join (Rule -> Job -> User)
	res, err := db.GetDB().Exec(`
		DELETE FROM rules 
		WHERE id = $1 
		AND job_id IN (SELECT id FROM jobs WHERE user_id = $2)
	`, id, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Rule not found or permission denied"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Rule deleted"})
}
