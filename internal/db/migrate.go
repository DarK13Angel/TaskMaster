package db

import (
	"github.com/google/uuid"
)

// migrate creates tables and default records if they don't exist
func (db *DB) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS projects (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		description TEXT,
		sort_order INTEGER DEFAULT 0,
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
		title TEXT NOT NULL,
		notes TEXT,
		status_id TEXT NOT NULL REFERENCES status_tags(id),
		priority TEXT NOT NULL DEFAULT 'medium',
		due_date DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		completed_at DATETIME
	);
	`
	if _, err := db.Exec(schema); err != nil {
		return err
	}
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
