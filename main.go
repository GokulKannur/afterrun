package main

import (
	"cronmonitor/config"
	"cronmonitor/db"
	"cronmonitor/handlers"
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

	// Phase 1.x
	r.POST("/ping/:ping_key", handlers.PingHandler)

	// Phase 2.1: Read-Only UI
	r.GET("/", handlers.ShowJobs)
	r.GET("/jobs/:id", handlers.ShowJobDetail)

	// Phase 2: Job Management API
	api := r.Group("/api")
	{
		api.POST("/jobs", handlers.CreateJob)
		api.GET("/jobs", handlers.ListJobs)
		api.GET("/jobs/:id", handlers.GetJob)
		api.DELETE("/jobs/:id", handlers.DeleteJob)

		api.GET("/jobs/:id/runs", handlers.GetJobRuns)

		api.POST("/jobs/:id/rules", handlers.CreateRule)
		api.GET("/jobs/:id/rules", handlers.ListRules)
		api.DELETE("/rules/:id", handlers.DeleteRule)

		// Phase 3.5: Stats (Read-Only)
		api.GET("/stats/overview", handlers.GetStatsOverview)
		api.GET("/stats/job/:id", handlers.GetJobStats)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Println("Server starting on port " + port)
	r.Run(":" + port)
}
