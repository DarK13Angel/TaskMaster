package test

import (
	"testing"

	"taskmaster/internal/db"
)

func TestInitDBDoesNotMigrate(t *testing.T) {
	t.Setenv("TASKMASTER_DATA_DIR", t.TempDir())

	database, err := db.InitDB()
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer database.Close()

	exists, err := database.TablesExist()
	if err != nil {
		t.Fatalf("TablesExist failed: %v", err)
	}
	if exists {
		t.Fatal("expected TablesExist to be false after InitDB without Migrate")
	}
}

func TestMigrateAndIdempotency(t *testing.T) {
	t.Setenv("TASKMASTER_DATA_DIR", t.TempDir())

	database, err := db.InitDB()
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer database.Close()

	if err := database.Migrate(); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	exists, err := database.TablesExist()
	if err != nil {
		t.Fatalf("TablesExist failed: %v", err)
	}
	if !exists {
		t.Fatal("expected TablesExist to be true after Migrate")
	}

	// Running Migrate again (force migration) should succeed without duplicating default tags.
	if err := database.Migrate(); err != nil {
		t.Fatalf("second Migrate failed: %v", err)
	}

	var tagCount int
	if err := database.QueryRow("SELECT COUNT(*) FROM status_tags").Scan(&tagCount); err != nil {
		t.Fatalf("failed to count status_tags: %v", err)
	}
	if tagCount != 5 {
		t.Fatalf("expected 5 default status_tags, got %d", tagCount)
	}
}
