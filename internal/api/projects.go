package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"taskmaster/internal/models"

	"github.com/google/uuid"
)

// ListProjects returns all projects
func (s *Server) ListProjects(w http.ResponseWriter, r *http.Request) {
	rows, err := s.DB.Query(`
		SELECT id, name, description, sort_order, created_at, updated_at
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
		if err := rows.Scan(&p.ID, &p.Name, &desc, &p.SetOrder, &p.CreatedAt, &p.UpdatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to scan project")
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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		writeError(w, http.StatusBadRequest, "project name is required")
		return
	}

	id := uuid.New().String()
	_, err := s.DB.Exec(`
		INSERT INTO projects (id, name, description, sort_order)
		VALUES (?, ?, ?, (SELECT COALESCE(MAX(sort_order), 0) + 1 FROM projects))
	`, id, req.Name, req.Description)

	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create project")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"id": id, "name": req.Name})
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
