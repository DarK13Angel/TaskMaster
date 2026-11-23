package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// DB wraps standard *sql.DB connection pool
type DB struct {
	*sql.DB
}

// InitDB opens/creates database for taskmaster
func InitDB() (*DB, error) {
	appDir := os.Getenv("TASKMASTER_DATA_DIR")
	if appDir == "" {
		appDir = os.Getenv("DATA_DIR")
	}
	if appDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get user home directory: %w", err)
		}
		appDir = filepath.Join(homeDir, ".taskmaster")
	}

	if err := os.MkdirAll(appDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create app directory: %w", err)
	}
	// creates dbpath and dsn
	dbPath := filepath.Join(appDir, "taskmaster.db")
	dsn := fmt.Sprintf("%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)", dbPath)
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to Open database: %w", err)
	}
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("database ping failed: %w", err)
	}

	db := &DB{sqlDB}
	if err := db.migrate(); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return db, nil
}
