package main

import (
	"fmt"
	"log"
	"taskmaster/internal/db"
)

func main() {
	fmt.Println("TaskMaster service starting...")

	database, err := db.InitDB()
	if err != nil {
		log.Fatalf("Failed to initialise database: %v", err)
	}
	defer database.Close()

	fmt.Println(" Connected to SQLite database (~/.taskmaster/taskmaster.db)")
	fmt.Println(" Database schema and initial seeds verified successfully!")
}
