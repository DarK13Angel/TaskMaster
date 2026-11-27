package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"taskmaster/internal/models"
	"taskmaster/internal/utils"
	"time"

	"github.com/google/uuid"
)

// ListProjects returns paginated projects
func (s *Server) ListProjects(w http.ResponseWriter, r *http.Request) {
	skip, limit, err := utils.ParsePagination(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	projects := make([]models.Project, 0)
	err = s.DB.NewSelect().
		Model(&projects).
		Offset(skip).
		Limit(limit).
		Order("sort_order ASC, created_at ASC").
		Scan(r.Context())

	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to query projects")
		return
	}

	writeJSON(w, http.StatusOK, projects)
}

// GetProject returns a project by id
func (s *Server) GetProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "project id is required")
		return
	}

	var p models.Project
	err := s.DB.NewSelect().Model(&p).Where("id = ?", id).Scan(r.Context())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "project not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to query project: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, p)
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
	name := strings.TrimSpace(req.Name)
	description := strings.TrimSpace(req.Description)

	var maxSort int
	if err := s.DB.NewSelect().
		Model((*models.Project)(nil)).
		ColumnExpr("COALESCE(MAX(sort_order), 0)").
		Scan(r.Context(), &maxSort); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create project")
		return
	}

	p := models.Project{
		ID:          id,
		Name:        name,
		Description: description,
		SortOrder:   maxSort + 1,
	}

	_, err := s.DB.NewInsert().Model(&p).Exec(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create project")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"id": id, "name": name})
}

// UpdateProject edits project details or archives/unarchives it
func (s *Server) UpdateProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "project id is required")
		return
	}
	var p models.Project
	// Check Project ID exists
	err := s.DB.NewSelect().Model(&p).Where("id = ?", id).Scan(r.Context())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "project not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to query project: "+err.Error())
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

	changed := false
	if req.Name != nil {
		trimmed := strings.TrimSpace(*req.Name)
		if trimmed != "" && p.Name != trimmed {
			p.Name = trimmed
			changed = true
		}
	}
	if req.Description != nil {
		trimmed := strings.TrimSpace(*req.Description)
		if p.Description != trimmed {
			p.Description = trimmed
			changed = true
		}
	}
	if req.IsArchived != nil && p.IsArchived != *req.IsArchived {
		p.IsArchived = *req.IsArchived
		changed = true
	}

	if !changed {
		writeJSON(w, http.StatusOK, map[string]string{"message": "no changes"})
		return
	}

	p.UpdatedAt = time.Now().UTC()

	res, err := s.DB.NewUpdate().Model(&p).Where("id = ?", id).Exec(r.Context())
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

	res, err := s.DB.NewDelete().Model((*models.Project)(nil)).Where("id = ?", id).Exec(r.Context())
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
