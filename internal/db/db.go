package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	_ "modernc.org/sqlite"
)

// DB wraps *bun.DB connection pool
type DB struct {
	*bun.DB
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

	bunDB := bun.NewDB(sqlDB, sqlitedialect.New())
	return &DB{bunDB}, nil
}
