package handlers

import (
	"cronmonitor/config"
	"cronmonitor/db"
	"cronmonitor/services"
	"net/http"

	"github.com/gin-gonic/gin"
)

func UpgradePlan(c *gin.Context) {
	// Feature Flag Check
	features := config.LoadFeatures()
	if !features.BillingEnabled {
		c.JSON(http.StatusNotFound, gin.H{"error": "Billing not enabled"})
		return
	}

	userID, _ := c.Get("userID")
	var req struct {
		Plan string `json:"plan"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	if !services.IsValidPlan(req.Plan) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid plan. Must be 'indie' or 'team'."})
		return
	}

	_, err := db.GetDB().Exec(
		"UPDATE users SET subscription_tier = $1, subscription_status = 'active' WHERE id = $2",
		req.Plan, userID,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":             "Upgraded successfully",
		"subscription_tier":   req.Plan,
		"subscription_status": "active",
	})
}

func DowngradePlan(c *gin.Context) {
	features := config.LoadFeatures()
	if !features.BillingEnabled {
		c.JSON(http.StatusNotFound, gin.H{"error": "Billing not enabled"})
		return
	}

	userID, _ := c.Get("userID")

	// Set to Free, status Cancelled (simulating end of paid period instantly for now)
	_, err := db.GetDB().Exec(
		"UPDATE users SET subscription_tier = 'free', subscription_status = 'cancelled' WHERE id = $1",
		userID,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":             "Downgraded to free",
		"subscription_tier":   "free",
		"subscription_status": "cancelled",
	})
}
