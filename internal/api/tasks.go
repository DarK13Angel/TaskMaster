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

// ListTasks returns tasks with optional query filters and subtask counts
func (s *Server) ListTasks(w http.ResponseWriter, r *http.Request) {
	query := `
		SELECT 
			t.id, t.project_id, t.title, t.notes, t.status_id, t.priority, 
			t.due_date, t.created_at, t.updated_at, t.completed_at,
			p.name AS project_name, s.name AS status_name,
			(SELECT COUNT(*) FROM subtasks WHERE task_id = t.id) AS total_subtasks,
			(SELECT COUNT(*) FROM subtasks WHERE task_id = t.id AND is_done = 1) AS done_subtasks
		FROM tasks t
		JOIN projects p ON t.project_id = p.id
		JOIN status_tags s ON t.status_id = s.id
		WHERE 1=1
	`
	var args []any

	if r.URL.Query().Get("include_archived") != "true" {
		query += " AND COALESCE(p.is_archived, 0) = 0"
	}

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
			&t.TotalSubtasks, &t.DoneSubtasks,
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

// GetTask returns a single task with its full list of subtasks
func (s *Server) GetTask(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")
	if taskID == "" {
		writeError(w, http.StatusBadRequest, "task id is required")
		return
	}

	query := `
		SELECT 
			t.id, t.project_id, t.title, t.notes, t.status_id, t.priority, 
			t.due_date, t.created_at, t.updated_at, t.completed_at,
			p.name AS project_name, s.name AS status_name
		FROM tasks t
		JOIN projects p ON t.project_id = p.id
		JOIN status_tags s ON t.status_id = s.id
		WHERE t.id = ?
	`

	var t models.Task
	var notes sql.NullString
	var dueDate, completedAt sql.NullTime

	err := s.DB.QueryRow(query, taskID).Scan(
		&t.ID, &t.ProjectID, &t.Title, &notes, &t.StatusID, &t.Priority,
		&dueDate, &t.CreatedAt, &t.UpdatedAt, &completedAt,
		&t.ProjectName, &t.StatusName,
	)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "task not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch task: "+err.Error())
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

	// Fetch subtasks
	subRows, err := s.DB.Query(`
		SELECT id, task_id, title, is_done, sort_order
		FROM subtasks
		WHERE task_id = ?
		ORDER BY sort_order ASC, created_at ASC
	`, taskID)
	if err == nil {
		defer subRows.Close()
		subtasks := []models.Subtask{}
		for subRows.Next() {
			var st models.Subtask
			if err := subRows.Scan(&st.ID, &st.TaskID, &st.Title, &st.IsDone, &st.SortOrder); err == nil {
				subtasks = append(subtasks, st)
				t.TotalSubtasks++
				if st.IsDone {
					t.DoneSubtasks++
				}
			}
		}
		t.Subtasks = subtasks
	}

	writeJSON(w, http.StatusOK, t)
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

// UpdateTask edits the task content (title, notes, priority, due_date, project_id)
func (s *Server) UpdateTask(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")
	if taskID == "" {
		writeError(w, http.StatusBadRequest, "task id is required")
		return
	}

	var req struct {
		Title     *string          `json:"title"`
		Notes     *string          `json:"notes"`
		Priority  *models.Priority `json:"priority"`
		DueDate   *time.Time       `json:"due_date"`
		ProjectID *string          `json:"project_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Build dynamic UPDATE query based on fields provided
	var updates []string
	var args []any

	if req.Title != nil && strings.TrimSpace(*req.Title) != "" {
		updates = append(updates, "title = ?")
		args = append(args, strings.TrimSpace(*req.Title))
	}
	if req.Notes != nil {
		updates = append(updates, "notes = ?")
		args = append(args, strings.TrimSpace(*req.Notes))
	}
	if req.Priority != nil && req.Priority.IsValid() {
		updates = append(updates, "priority = ?")
		args = append(args, *req.Priority)
	}
	if req.DueDate != nil {
		updates = append(updates, "due_date = ?")
		args = append(args, *req.DueDate)
	}
	if req.ProjectID != nil && *req.ProjectID != "" {
		updates = append(updates, "project_id = ?")
		args = append(args, *req.ProjectID)
	}

	if len(updates) == 0 {
		writeJSON(w, http.StatusOK, map[string]string{"message": "no changes"})
		return
	}

	updates = append(updates, "updated_at = CURRENT_TIMESTAMP")
	query := "UPDATE tasks SET " + strings.Join(updates, ", ") + " WHERE id = ?"
	args = append(args, taskID)

	res, err := s.DB.Exec(query, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update task: "+err.Error())
		return
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "task updated successfully"})
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

	// 4. Update task status & timestamp (record completed_at if moving to Done or Killed)
	var completedAt *time.Time
	if targetStatus.Name == "Done" || targetStatus.Name == "Killed" {
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

// ==========================================
// SUBTASKS (CHECKLIST) HANDLERS
// ==========================================

// ListSubtasks returns all subtasks for a given task
func (s *Server) ListSubtasks(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")
	if taskID == "" {
		writeError(w, http.StatusBadRequest, "task id is required")
		return
	}

	rows, err := s.DB.Query(`
		SELECT id, task_id, title, is_done, sort_order
		FROM subtasks
		WHERE task_id = ?
		ORDER BY sort_order ASC, created_at ASC
	`, taskID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to query subtasks")
		return
	}
	defer rows.Close()

	subtasks := []models.Subtask{}
	for rows.Next() {
		var st models.Subtask
		if err := rows.Scan(&st.ID, &st.TaskID, &st.Title, &st.IsDone, &st.SortOrder); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to scan subtask")
			return
		}
		subtasks = append(subtasks, st)
	}

	writeJSON(w, http.StatusOK, subtasks)
}

// AddSubtask creates a new checklist item under a task
func (s *Server) AddSubtask(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")
	if taskID == "" {
		writeError(w, http.StatusBadRequest, "task id is required")
		return
	}

	var req struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Title) == "" {
		writeError(w, http.StatusBadRequest, "subtask title is required")
		return
	}

	id := uuid.New().String()
	_, err := s.DB.Exec(`
		INSERT INTO subtasks (id, task_id, title, is_done, sort_order)
		VALUES (?, ?, ?, 0, (SELECT COALESCE(MAX(sort_order), 0) + 1 FROM subtasks WHERE task_id = ?))
	`, id, taskID, strings.TrimSpace(req.Title), taskID)

	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to add subtask: "+err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, models.Subtask{
		ID:     id,
		TaskID: taskID,
		Title:  req.Title,
		IsDone: false,
	})
}

// ToggleSubtask toggles completion or updates title of a subtask
func (s *Server) ToggleSubtask(w http.ResponseWriter, r *http.Request) {
	subtaskID := r.PathValue("id")
	if subtaskID == "" {
		writeError(w, http.StatusBadRequest, "subtask id is required")
		return
	}

	var req struct {
		Title  *string `json:"title"`
		IsDone *bool   `json:"is_done"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var updates []string
	var args []any

	if req.Title != nil {
		updates = append(updates, "title = ?")
		args = append(args, strings.TrimSpace(*req.Title))
	}
	if req.IsDone != nil {
		updates = append(updates, "is_done = ?")
		args = append(args, *req.IsDone)
	}

	if len(updates) == 0 {
		writeJSON(w, http.StatusOK, map[string]string{"message": "no changes"})
		return
	}

	query := "UPDATE subtasks SET " + strings.Join(updates, ", ") + " WHERE id = ?"
	args = append(args, subtaskID)

	res, err := s.DB.Exec(query, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update subtask: "+err.Error())
		return
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		writeError(w, http.StatusNotFound, "subtask not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "subtask updated"})
}

// DeleteSubtask removes a checklist item
func (s *Server) DeleteSubtask(w http.ResponseWriter, r *http.Request) {
	subtaskID := r.PathValue("id")
	if subtaskID == "" {
		writeError(w, http.StatusBadRequest, "subtask id is required")
		return
	}

	res, err := s.DB.Exec("DELETE FROM subtasks WHERE id = ?", subtaskID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete subtask")
		return
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		writeError(w, http.StatusNotFound, "subtask not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "subtask deleted"})
}
