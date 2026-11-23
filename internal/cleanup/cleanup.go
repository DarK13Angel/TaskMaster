package cleanup

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"taskmaster/internal/db"
)

// PurgeOldCompletedTasks deletes tasks marked Done or Killed whose completed_at is older than retentionDays
func PurgeOldCompletedTasks(database *db.DB, retentionDays int) (int64, error) {
	if retentionDays <= 0 {
		return 0, nil
	}

	// 1. Fetch task IDs so we can clean up any local attachment folders
	queryFind := fmt.Sprintf(`
		SELECT id FROM tasks 
		WHERE completed_at IS NOT NULL 
		  AND completed_at <= datetime('now', '-%d days')
		  AND status_id IN (SELECT id FROM status_tags WHERE name IN ('Done', 'Killed'))
	`, retentionDays)

	rows, err := database.Query(queryFind)
	if err != nil {
		return 0, fmt.Errorf("failed to query tasks for cleanup: %w", err)
	}
	defer rows.Close()

	var taskIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			taskIDs = append(taskIDs, id)
		}
	}

	if len(taskIDs) == 0 {
		return 0, nil
	}

	// 2. Increment historical_done_count on projects for all 'Done' tasks about to be purged
	queryIncrement := fmt.Sprintf(`
		UPDATE projects 
		SET historical_done_count = historical_done_count + (
			SELECT COUNT(*) FROM tasks 
			WHERE tasks.project_id = projects.id 
			  AND tasks.completed_at IS NOT NULL 
			  AND tasks.completed_at <= datetime('now', '-%d days')
			  AND tasks.status_id = (SELECT id FROM status_tags WHERE name = 'Done')
		)
		WHERE id IN (
			SELECT DISTINCT project_id FROM tasks 
			WHERE completed_at IS NOT NULL 
			  AND completed_at <= datetime('now', '-%d days')
			  AND status_id = (SELECT id FROM status_tags WHERE name = 'Done')
		)
	`, retentionDays, retentionDays)
	_, _ = database.Exec(queryIncrement)

	// 3. Delete tasks from SQLite (Cascade deletes subtasks and attachments automatically)
	queryDelete := fmt.Sprintf(`
		DELETE FROM tasks 
		WHERE completed_at IS NOT NULL 
		  AND completed_at <= datetime('now', '-%d days')
		  AND status_id IN (SELECT id FROM status_tags WHERE name IN ('Done', 'Killed'))
	`, retentionDays)

	res, err := database.Exec(queryDelete)
	if err != nil {
		return 0, fmt.Errorf("failed to delete old tasks: %w", err)
	}

	deletedCount, _ := res.RowsAffected()

	// 3. Remove local upload directories from disk
	homeDir, _ := os.UserHomeDir()
	if homeDir != "" {
		for _, id := range taskIDs {
			taskUploadDir := filepath.Join(homeDir, ".taskmaster", "uploads", id)
			_ = os.RemoveAll(taskUploadDir)
		}
	}

	if deletedCount > 0 {
		log.Printf("🧹 [Auto-Cleanup] Purged %d completed/killed tasks older than %d days", deletedCount, retentionDays)
	}

	return deletedCount, nil
}

// StartWorker runs the cleanup routine periodically in a lightweight background goroutine
func StartWorker(database *db.DB, retentionDays int, interval time.Duration) {
	go func() {
		// Run once on startup after 3 seconds
		time.Sleep(3 * time.Second)
		_, _ = PurgeOldCompletedTasks(database, retentionDays)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			_, _ = PurgeOldCompletedTasks(database, retentionDays)
		}
	}()
}
