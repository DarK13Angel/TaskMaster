package main

import (
	"flag"
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
	forceMigrate := flag.Bool("force-migrate", false, "Force run database migrations and exit")
	flag.Parse()

	fmt.Println("🚀 TaskMaster service starting...")

	database, err := db.InitDB()
	if err != nil {
		log.Fatalf("Failed to initialise database: %v", err)
	}
	defer database.Close()

	if *forceMigrate {
		fmt.Println("Running forced database migration...")
		if err := database.Migrate(); err != nil {
			log.Fatalf("Failed to migrate database: %v", err)
		}
		fmt.Println("Database migration completed successfully.")
		return
	}

	exists, err := database.TablesExist()
	if err != nil {
		log.Fatalf("Failed to check database tables: %v", err)
	}
	if !exists {
		fmt.Println("Database tables not found. Running initial migration...")
		if err := database.Migrate(); err != nil {
			log.Fatalf("Failed to migrate database: %v", err)
		}
		fmt.Println("Initial database migration completed. Exiting.")
		return
	}

	fmt.Println(" Connected to SQLite database (~/.taskmaster/taskmaster.db)")

	cfg := config.Load()

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
