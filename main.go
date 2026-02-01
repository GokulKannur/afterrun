package main

import (
	"cronmonitor/config"
	"cronmonitor/db"
	"cronmonitor/handlers"
	"cronmonitor/middleware"
	"cronmonitor/services"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

func runMigrations() {
	sqlBytes, err := os.ReadFile("schema.sql")
	if err != nil {
		log.Fatal("Failed to read schema.sql:", err)
	}

	if _, err := db.GetDB().Exec(string(sqlBytes)); err != nil {
		log.Fatal("Failed to apply schema:", err)
	}
	log.Println("Database schema verified")
}

func main() {
	fmt.Println("BOOTING v12 - Phase 3.5 Ready...")
	if err := db.InitDB(); err != nil {
		log.Fatal("Failed to connect to database: ", err)
	}
	runMigrations()

	// Phase 3.5: Feature Flags
	features := config.LoadFeatures()
	log.Printf(
		"Features: auth=%v billing=%v baselines=%v write_ui=%v",
		features.AuthEnabled,
		features.BillingEnabled,
		features.BaselinesEnabled,
		features.WriteUIEnabled,
	)

	// Phase 1.3: Background Missed Run Check
	go func() {
		// 30 Seconds for testing (as per requirement)
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			func() {
				defer func() {
					if r := recover(); r != nil {
						fmt.Printf("MissedRun Ticker panic: %v\n", r)
					}
				}()
				services.CheckForMissedRuns()
			}()
		}
	}()

	r := gin.Default()
	r.LoadHTMLGlob("templates/*")
	r.Static("/static", "./static")

	// Public Webhook (MUST be public)
	// Support GET/HEAD for compatibility with wget/curl and browser testing
	r.POST("/ping/:ping_key", handlers.PingHandler)
	r.GET("/ping/:ping_key", handlers.PingHandler)
	r.HEAD("/ping/:ping_key", handlers.PingHandler)

	// Auth Routes (Public)
	api := r.Group("/api")
	api.POST("/auth/signup", handlers.Signup)
	api.POST("/auth/login", handlers.Login)
	api.GET("/auth/me", middleware.AuthRequired(), handlers.Me)

	// Billing Routes (Simulated)
	api.POST("/billing/upgrade", middleware.AuthRequired(), handlers.UpgradePlan)
	api.POST("/billing/downgrade", middleware.AuthRequired(), handlers.DowngradePlan)

	// UI Routes (SSR - Auth via Cookie inside Handlers is handled by middleware wrapper if we choose)
	// For Phase 4, we wrap UI in middleware too, as it supports Cookie auth fallback.
	ui := r.Group("/")
	ui.Use(middleware.AuthRequired())
	{
		ui.GET("/", handlers.ShowJobs)
		ui.GET("/jobs/:id", handlers.ShowJobDetail)
	}

	// Open UI Routes
	r.GET("/login", handlers.ShowLogin)
	r.GET("/signup", handlers.ShowSignup)

	// Protected API Routes
	protected := api.Group("/")
	protected.Use(middleware.AuthRequired())
	{
		protected.POST("/jobs", handlers.CreateJob)
		protected.GET("/jobs", handlers.ListJobs)
		protected.GET("/jobs/:id", handlers.GetJob)
		protected.DELETE("/jobs/:id", handlers.DeleteJob)

		protected.GET("/jobs/:id/runs", handlers.GetJobRuns)

		protected.POST("/jobs/:id/rules", handlers.CreateRule)
		protected.GET("/jobs/:id/rules", handlers.ListRules)
		protected.DELETE("/rules/:id", handlers.DeleteRule)

		// Phase 3.5: Stats (Read-Only)
		protected.GET("/stats/overview", handlers.GetStatsOverview)
		protected.GET("/stats/job/:id", handlers.GetJobStats)
	}

	// DEBUG: Explicitly check if we can read the file
	r.GET("/debug/css", func(c *gin.Context) {
		content, err := os.ReadFile("static/style.css")
		if err != nil {
			c.String(500, "Error reading file: %v", err)
			return
		}
		c.String(200, "File Read Success! Size: %d bytes\n\n%s", len(content), string(content))
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Println("Server starting on port " + port)
	r.Run(":" + port)
}
