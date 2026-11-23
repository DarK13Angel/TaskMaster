package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"taskmaster/internal/models"

	"github.com/google/uuid"
)

// ListProjects returns all projects
func (s *Server) ListProjects(w http.ResponseWriter, r *http.Request) {
	rows, err := s.DB.Query(`
		SELECT id, name, description, sort_order, COALESCE(is_archived, 0), COALESCE(historical_done_count, 0), created_at, updated_at
		FROM projects
		ORDER BY sort_order ASC, created_at ASC
	`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to query projects")
		return
	}
	defer rows.Close()

	projects := []models.Project{}
	for rows.Next() {
		var p models.Project
		var desc sql.NullString
		if err := rows.Scan(&p.ID, &p.Name, &desc, &p.SortOrder, &p.IsArchived, &p.HistoricalDoneCount, &p.CreatedAt, &p.UpdatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to scan project: "+err.Error())
			return
		}
		if desc.Valid {
			p.Description = desc.String
		}
		projects = append(projects, p)
	}

	writeJSON(w, http.StatusOK, projects)
}

// CreateProject adds a new project
func (s *Server) CreateProject(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Name) == "" {
		writeError(w, http.StatusBadRequest, "project name is required")
		return
	}

	id := uuid.New().String()
	_, err := s.DB.Exec(`
		INSERT INTO projects (id, name, description, sort_order, is_archived, historical_done_count)
		VALUES (?, ?, ?, (SELECT COALESCE(MAX(sort_order), 0) + 1 FROM projects), 0, 0)
	`, id, strings.TrimSpace(req.Name), strings.TrimSpace(req.Description))

	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create project")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"id": id, "name": req.Name})
}

// UpdateProject edits project details or archives/unarchives it
func (s *Server) UpdateProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "project id is required")
		return
	}

	var req struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		IsArchived  *bool   `json:"is_archived"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var updates []string
	var args []any

	if req.Name != nil && strings.TrimSpace(*req.Name) != "" {
		updates = append(updates, "name = ?")
		args = append(args, strings.TrimSpace(*req.Name))
	}
	if req.Description != nil {
		updates = append(updates, "description = ?")
		args = append(args, strings.TrimSpace(*req.Description))
	}
	if req.IsArchived != nil {
		updates = append(updates, "is_archived = ?")
		args = append(args, *req.IsArchived)
	}

	if len(updates) == 0 {
		writeJSON(w, http.StatusOK, map[string]string{"message": "no changes"})
		return
	}

	updates = append(updates, "updated_at = CURRENT_TIMESTAMP")
	query := "UPDATE projects SET " + strings.Join(updates, ", ") + " WHERE id = ?"
	args = append(args, id)

	res, err := s.DB.Exec(query, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update project: "+err.Error())
		return
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "project updated successfully"})
}

// DeleteProject deletes a project and cascades deletion to its tasks
func (s *Server) DeleteProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "project id is required")
		return
	}

	res, err := s.DB.Exec("DELETE FROM projects WHERE id = ?", id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete project")
		return
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "project deleted"})
}
