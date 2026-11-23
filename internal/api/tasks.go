package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"taskmaster/internal/models"
	"taskmaster/internal/state"

	"github.com/google/uuid"
)

// ListTasks returns tasks with optional query filters:
// /api/v1/tasks?project_id=...&status_id=...&priority=...&search=...
func (s *Server) ListTasks(w http.ResponseWriter, r *http.Request) {
	query := `
		SELECT 
			t.id, t.project_id, t.title, t.notes, t.status_id, t.priority, 
			t.due_date, t.created_at, t.updated_at, t.completed_at,
			p.name AS project_name, s.name AS status_name
		FROM tasks t
		JOIN projects p ON t.project_id = p.id
		JOIN status_tags s ON t.status_id = s.id
		WHERE 1=1
	`
	var args []any

	if projectID := r.URL.Query().Get("project_id"); projectID != "" {
		query += " AND t.project_id = ?"
		args = append(args, projectID)
	}
	if statusID := r.URL.Query().Get("status_id"); statusID != "" {
		query += " AND t.status_id = ?"
		args = append(args, statusID)
	}
	if priority := r.URL.Query().Get("priority"); priority != "" {
		query += " AND t.priority = ?"
		args = append(args, priority)
	}
	if search := r.URL.Query().Get("search"); search != "" {
		query += " AND (t.title LIKE ? OR t.notes LIKE ?)"
		pattern := "%" + search + "%"
		args = append(args, pattern, pattern)
	}

	query += " ORDER BY t.created_at DESC"

	rows, err := s.DB.Query(query, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to query tasks")
		return
	}
	defer rows.Close()

	tasks := []models.Task{}
	for rows.Next() {
		var t models.Task
		var notes sql.NullString
		var dueDate, completedAt sql.NullTime

		err := rows.Scan(
			&t.ID, &t.ProjectID, &t.Title, &notes, &t.StatusID, &t.Priority,
			&dueDate, &t.CreatedAt, &t.UpdatedAt, &completedAt,
			&t.ProjectName, &t.StatusName,
		)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to scan task: "+err.Error())
			return
		}

		if notes.Valid {
			t.Notes = notes.String
		}
		if dueDate.Valid {
			t.DueDate = &dueDate.Time
		}
		if completedAt.Valid {
			t.CompletedAt = &completedAt.Time
		}

		tasks = append(tasks, t)
	}

	writeJSON(w, http.StatusOK, tasks)
}

// CreateTask adds a new task to a project
func (s *Server) CreateTask(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProjectID string          `json:"project_id"`
		Title     string          `json:"title"`
		Notes     string          `json:"notes"`
		StatusID  string          `json:"status_id"`
		Priority  models.Priority `json:"priority"`
		DueDate   *time.Time      `json:"due_date"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if strings.TrimSpace(req.Title) == "" {
		writeError(w, http.StatusBadRequest, "task title is required")
		return
	}

	// Default to first project if project_id not provided
	if req.ProjectID == "" {
		err := s.DB.QueryRow("SELECT id FROM projects ORDER BY sort_order ASC LIMIT 1").Scan(&req.ProjectID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "no project found; create a project first")
			return
		}
	}

	// Fetch or default the status_id to an initial status (ToDo or Backlog)
	var targetStatus models.StatusTag
	if req.StatusID != "" {
		err := s.DB.QueryRow(`
			SELECT id, name, sort_order, is_system, is_initial, is_terminal
			FROM status_tags WHERE id = ?
		`, req.StatusID).Scan(&targetStatus.ID, &targetStatus.Name, &targetStatus.SortOrder, &targetStatus.IsSystem, &targetStatus.IsInitial, &targetStatus.IsTerminal)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid status_id")
			return
		}
	} else {
		// Default to 'ToDo' if not specified
		err := s.DB.QueryRow(`
			SELECT id, name, sort_order, is_system, is_initial, is_terminal
			FROM status_tags WHERE name = 'ToDo'
		`).Scan(&targetStatus.ID, &targetStatus.Name, &targetStatus.SortOrder, &targetStatus.IsSystem, &targetStatus.IsInitial, &targetStatus.IsTerminal)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "default ToDo status not found")
			return
		}
	}

	// Validate creation rule via state machine
	if err := state.ValidateCreation(targetStatus); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Validate priority, default to Medium if empty
	if !req.Priority.IsValid() {
		req.Priority = models.PriorityMedium
	}

	id := uuid.New().String()
	_, err := s.DB.Exec(`
		INSERT INTO tasks (id, project_id, title, notes, status_id, priority, due_date)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, id, req.ProjectID, req.Title, req.Notes, targetStatus.ID, req.Priority, req.DueDate)

	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to insert task: "+err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{
		"id":        id,
		"title":     req.Title,
		"status_id": targetStatus.ID,
		"priority":  string(req.Priority),
	})
}

// MoveTask transitions a task to a target status after running state machine validation
func (s *Server) MoveTask(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")
	if taskID == "" {
		writeError(w, http.StatusBadRequest, "task id is required")
		return
	}

	var req models.MoveTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.TargetStatusID == "" {
		writeError(w, http.StatusBadRequest, "target_status_id is required")
		return
	}

	// 1. Fetch current task status
	var currentStatus models.StatusTag
	err := s.DB.QueryRow(`
		SELECT s.id, s.name, s.sort_order, s.is_system, s.is_initial, s.is_terminal
		FROM tasks t
		JOIN status_tags s ON t.status_id = s.id
		WHERE t.id = ?
	`, taskID).Scan(&currentStatus.ID, &currentStatus.Name, &currentStatus.SortOrder, &currentStatus.IsSystem, &currentStatus.IsInitial, &currentStatus.IsTerminal)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "task not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch task: "+err.Error())
		return
	}

	// If moving to the exact same status, ignore and return success immediately
	if currentStatus.ID == req.TargetStatusID {
		writeJSON(w, http.StatusOK, map[string]string{"message": "status unchanged"})
		return
	}

	// 2. Fetch target status
	var targetStatus models.StatusTag
	err = s.DB.QueryRow(`
		SELECT id, name, sort_order, is_system, is_initial, is_terminal
		FROM status_tags WHERE id = ?
	`, req.TargetStatusID).Scan(&targetStatus.ID, &targetStatus.Name, &targetStatus.SortOrder, &targetStatus.IsSystem, &targetStatus.IsInitial, &targetStatus.IsTerminal)
	if err != nil {
		writeError(w, http.StatusBadRequest, "target status not found")
		return
	}

	// 3. Validate transition
	if err := state.ValidateTransition(currentStatus, targetStatus); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// 4. Update task status & timestamp (record completed_at if moving to Done)
	var completedAt *time.Time
	if targetStatus.Name == "Done" {
		now := time.Now()
		completedAt = &now
	}

	_, err = s.DB.Exec(`
		UPDATE tasks 
		SET status_id = ?, completed_at = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, targetStatus.ID, completedAt, taskID)

	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update task: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"id":              taskID,
		"new_status_id":   targetStatus.ID,
		"new_status_name": targetStatus.Name,
	})
}

// DeleteTask removes a task
func (s *Server) DeleteTask(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")
	if taskID == "" {
		writeError(w, http.StatusBadRequest, "task id is required")
		return
	}

	res, err := s.DB.Exec("DELETE FROM tasks WHERE id = ?", taskID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete task")
		return
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "task deleted"})
}
