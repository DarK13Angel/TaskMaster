package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"taskmaster/internal/models"
	"taskmaster/internal/state"

	"github.com/google/uuid"
)

// ListTasks returns tasks with optional query filters and subtask counts
func (s *Server) ListTasks(w http.ResponseWriter, r *http.Request) {
	tasks := make([]models.Task, 0)
	q := s.DB.NewSelect().
		Model(&tasks).
		Column("t.*").
		ColumnExpr("p.name AS project_name").
		ColumnExpr("s.name AS status_name").
		ColumnExpr("(SELECT COUNT(*) FROM subtasks WHERE task_id = t.id) AS total_subtasks").
		ColumnExpr("(SELECT COUNT(*) FROM subtasks WHERE task_id = t.id AND is_done = 1) AS done_subtasks").
		Join("JOIN projects p ON t.project_id = p.id").
		Join("JOIN status_tags s ON t.status_id = s.id")

	if r.URL.Query().Get("include_archived") != "true" {
		q = q.Where("COALESCE(p.is_archived, 0) = 0")
	}
	if projectID := r.URL.Query().Get("project_id"); projectID != "" {
		q = q.Where("t.project_id = ?", projectID)
	}
	if statusID := r.URL.Query().Get("status_id"); statusID != "" {
		q = q.Where("t.status_id = ?", statusID)
	}
	if priority := r.URL.Query().Get("priority"); priority != "" {
		q = q.Where("t.priority = ?", priority)
	}
	if search := r.URL.Query().Get("search"); search != "" {
		pattern := "%" + search + "%"
		q = q.Where("(t.title LIKE ? OR t.notes LIKE ?)", pattern, pattern)
	}

	if err := q.Order("t.created_at DESC").Scan(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to query tasks")
		return
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

	var t models.Task
	err := s.DB.NewSelect().
		Model(&t).
		Column("t.*").
		ColumnExpr("p.name AS project_name").
		ColumnExpr("s.name AS status_name").
		Join("JOIN projects p ON t.project_id = p.id").
		Join("JOIN status_tags s ON t.status_id = s.id").
		Where("t.id = ?", taskID).
		Scan(r.Context())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "task not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to fetch task: "+err.Error())
		return
	}

	subtasks := make([]models.Subtask, 0)
	if err := s.DB.NewSelect().
		Model(&subtasks).
		Where("task_id = ?", taskID).
		Order("sort_order ASC, created_at ASC").
		Scan(r.Context()); err == nil {
		for _, st := range subtasks {
			t.TotalSubtasks++
			if st.IsDone {
				t.DoneSubtasks++
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
		err := s.DB.NewSelect().
			Model((*models.Project)(nil)).
			Column("id").
			Order("sort_order ASC").
			Limit(1).
			Scan(r.Context(), &req.ProjectID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "no project found; create a project first")
			return
		}
	}

	// Fetch or default the status_id to an initial status (ToDo or Backlog)
	var targetStatus models.StatusTag
	if req.StatusID != "" {
		err := s.DB.NewSelect().Model(&targetStatus).Where("id = ?", req.StatusID).Scan(r.Context())
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid status_id")
			return
		}
	} else {
		// Default to 'ToDo' if not specified
		err := s.DB.NewSelect().Model(&targetStatus).Where("name = ?", "ToDo").Scan(r.Context())
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
	task := models.Task{
		ID:        id,
		ProjectID: req.ProjectID,
		Title:     req.Title,
		Notes:     req.Notes,
		StatusID:  targetStatus.ID,
		Priority:  req.Priority,
		DueDate:   req.DueDate,
	}

	if _, err := s.DB.NewInsert().Model(&task).Exec(r.Context()); err != nil {
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
	q := s.DB.NewUpdate().Model((*models.Task)(nil))
	hasUpdates := false

	if req.Title != nil && strings.TrimSpace(*req.Title) != "" {
		q = q.Set("title = ?", strings.TrimSpace(*req.Title))
		hasUpdates = true
	}
	if req.Notes != nil {
		q = q.Set("notes = ?", strings.TrimSpace(*req.Notes))
		hasUpdates = true
	}
	if req.Priority != nil && req.Priority.IsValid() {
		q = q.Set("priority = ?", *req.Priority)
		hasUpdates = true
	}
	if req.DueDate != nil {
		q = q.Set("due_date = ?", *req.DueDate)
		hasUpdates = true
	}
	if req.ProjectID != nil && *req.ProjectID != "" {
		q = q.Set("project_id = ?", *req.ProjectID)
		hasUpdates = true
	}

	if !hasUpdates {
		writeJSON(w, http.StatusOK, map[string]string{"message": "no changes"})
		return
	}

	res, err := q.Set("updated_at = CURRENT_TIMESTAMP").Where("id = ?", taskID).Exec(r.Context())
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
	err := s.DB.NewSelect().
		Model(&currentStatus).
		Join("JOIN tasks t ON t.status_id = st.id").
		Where("t.id = ?", taskID).
		Scan(r.Context())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "task not found")
			return
		}
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
	err = s.DB.NewSelect().Model(&targetStatus).Where("id = ?", req.TargetStatusID).Scan(r.Context())
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
		now := time.Now().UTC()
		completedAt = &now
	}

	_, err = s.DB.NewUpdate().
		Model((*models.Task)(nil)).
		Set("status_id = ?", targetStatus.ID).
		Set("completed_at = ?", completedAt).
		Set("updated_at = CURRENT_TIMESTAMP").
		Where("id = ?", taskID).
		Exec(r.Context())

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

	res, err := s.DB.NewDelete().Model((*models.Task)(nil)).Where("id = ?", taskID).Exec(r.Context())
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

	subtasks := make([]models.Subtask, 0)
	err := s.DB.NewSelect().
		Model(&subtasks).
		Where("task_id = ?", taskID).
		Order("sort_order ASC, created_at ASC").
		Scan(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to query subtasks")
		return
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

	var maxSort int
	if err := s.DB.NewSelect().
		Model((*models.Subtask)(nil)).
		ColumnExpr("COALESCE(MAX(sort_order), 0)").
		Where("task_id = ?", taskID).
		Scan(r.Context(), &maxSort); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to add subtask: "+err.Error())
		return
	}

	id := uuid.New().String()
	st := models.Subtask{
		ID:        id,
		TaskID:    taskID,
		Title:     strings.TrimSpace(req.Title),
		IsDone:    false,
		SortOrder: maxSort + 1,
	}

	if _, err := s.DB.NewInsert().Model(&st).Exec(r.Context()); err != nil {
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

	q := s.DB.NewUpdate().Model((*models.Subtask)(nil))
	hasUpdates := false

	if req.Title != nil {
		q = q.Set("title = ?", strings.TrimSpace(*req.Title))
		hasUpdates = true
	}
	if req.IsDone != nil {
		q = q.Set("is_done = ?", *req.IsDone)
		hasUpdates = true
	}

	if !hasUpdates {
		writeJSON(w, http.StatusOK, map[string]string{"message": "no changes"})
		return
	}

	res, err := q.Where("id = ?", subtaskID).Exec(r.Context())
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

	res, err := s.DB.NewDelete().Model((*models.Subtask)(nil)).Where("id = ?", subtaskID).Exec(r.Context())
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
