package db

import (
	"github.com/google/uuid"
)

// TablesExist checks whether all required application tables exist in the database.
func (db *DB) TablesExist() (bool, error) {
	var count int
	err := db.QueryRow(`
		SELECT COUNT(*) FROM sqlite_master
		WHERE type = 'table' AND name IN ('projects', 'status_tags', 'tasks', 'subtasks')
	`).Scan(&count)
	if err != nil {
		return false, err
	}
	return count == 4, nil
}

// Migrate creates tables and default records if they don't exist
func (db *DB) Migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS projects (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		description TEXT,
		sort_order INTEGER DEFAULT 0,
		is_archived BOOLEAN DEFAULT 0,
		historical_done_count INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS status_tags (
		id TEXT PRIMARY KEY,
		name TEXT UNIQUE NOT NULL,
		sort_order INTEGER NOT NULL,
		is_system BOOLEAN DEFAULT 0,
		is_initial BOOLEAN DEFAULT 0,
		is_terminal BOOLEAN DEFAULT 0
	);
	CREATE TABLE IF NOT EXISTS tasks (
		id TEXT PRIMARY KEY,
		project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
		parent_task TEXT REFERENCES tasks(id) ON DELETE SET NULL,
		title TEXT NOT NULL,
		notes TEXT,
		status_id TEXT NOT NULL REFERENCES status_tags(id),
		priority TEXT NOT NULL DEFAULT 'low',
		due_date DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		completed_at DATETIME
	);

	CREATE TABLE IF NOT EXISTS subtasks (
		id TEXT PRIMARY KEY,
		task_id TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
		title TEXT NOT NULL,
		is_done BOOLEAN DEFAULT 0,
		sort_order INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`
	if _, err := db.Exec(schema); err != nil {
		return err
	}

	// Graceful upgrades for existing databases
	_, _ = db.Exec("ALTER TABLE projects ADD COLUMN is_archived BOOLEAN DEFAULT 0;")
	_, _ = db.Exec("ALTER TABLE projects ADD COLUMN historical_done_count INTEGER DEFAULT 0;")
	_, _ = db.Exec("ALTER TABLE tasks ADD COLUMN parent_task TEXT REFERENCES tasks(id) ON DELETE SET NULL;")

	return db.seedDefaults()
}

// seedDefaults inserts initial Status Tags and a default Project if the DB is new
func (db *DB) seedDefaults() error {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM status_tags").Scan(&count)
	if err != nil {
		return err
	}
	if count == 0 {
		defaultTags := []struct {
			name       string
			order      int
			isSystem   bool
			isInitial  bool
			isTerminal bool
		}{
			{"Backlog", 100, true, true, false},
			{"ToDo", 200, true, true, false},
			{"In Progress", 300, true, false, false},
			{"Done", 400, true, false, true},
			{"Killed", 500, true, false, true},
		}
		stmt, err := db.Prepare(`
			INSERT INTO status_tags (id, name, sort_order, is_system, is_initial, is_terminal)
			VALUES (?, ?, ?, ?, ?, ?)
		`)
		if err != nil {
			return err
		}
		defer stmt.Close()
		for _, tag := range defaultTags {
			id := uuid.New().String()
			if _, err := stmt.Exec(id, tag.name, tag.order, tag.isSystem, tag.isInitial, tag.isTerminal); err != nil {
				return err
			}
		}
	}
	// // Seed a default project if none exists
	// var projectCount int
	// err = db.QueryRow("SELECT COUNT(*) FROM projects").Scan(&projectCount)
	// if err != nil {
	// 	return err
	// }
	// if projectCount == 0 {
	// 	projectID := uuid.New().String()
	// 	_, err := db.Exec(`
	// 		INSERT INTO projects (id, name, description, color, sort_order)
	// 		VALUES (?, 'General', 'Default project workspace', '#4F46E5', 0)
	// 	`, projectID)
	// 	if err != nil {
	// 		return err
	// 	}
	// }
	return nil
}
