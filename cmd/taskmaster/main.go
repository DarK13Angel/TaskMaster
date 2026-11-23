package main

import (
	"fmt"
	"log"
	"net/http"

	"taskmaster/internal/api"
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

	server := api.NewServer(database)
	port := "8088"
	fmt.Printf("TaskMaster API listening on http://localhost:%s\n", port)

	if err := http.ListenAndServe(":"+port, server.Routes()); err != nil {
		log.Fatal("Failed to start server: %v", err)
	}
}
