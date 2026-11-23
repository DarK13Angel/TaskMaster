package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"taskmaster"
	"taskmaster/internal/api"
	"taskmaster/internal/cleanup"
	"taskmaster/internal/config"
	"taskmaster/internal/db"
)

func main() {
	fmt.Println("🚀 TaskMaster service starting...")

	cfg := config.Load()

	database, err := db.InitDB()
	if err != nil {
		log.Fatalf("Failed to initialise database: %v", err)
	}
	defer database.Close()

	fmt.Println(" Connected to SQLite database (~/.taskmaster/taskmaster.db)")

	// Start background cleanup worker from .env settings
	cleanupInterval := time.Duration(cfg.CleanupIntervalHours) * time.Hour
	cleanup.StartWorker(database, cfg.TaskTTLDays, cleanupInterval)
	fmt.Printf("🧹 Auto-cleanup active: Completed/Killed tasks purged after %d days (every %d hrs)\n", cfg.TaskTTLDays, cfg.CleanupIntervalHours)

	server := api.NewServer(database, taskmaster.DistFS())
	fmt.Printf(" TaskMaster API & Web UI listening on http://localhost:%s\n", cfg.Port)

	if err := http.ListenAndServe(":"+cfg.Port, server.Routes()); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
