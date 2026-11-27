package api

import (
	"net/http"
	"strings"
	"time"

	"taskmaster/internal/models"
	"taskmaster/internal/utils"

	"github.com/google/uuid"
)

// ListProjects returns paginated projects
func (s *Server) ListProjects(w http.ResponseWriter, r *http.Request) {
	skip, limit, err := utils.ParsePagination(r)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	projects := make([]models.Project, 0)
	err = s.DB.NewSelect().
		Model(&projects).
		Offset(skip).
		Limit(limit).
		Order("sort_order ASC", "created_at ASC").
		Scan(r.Context())
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "failed to query projects")
		return
	}

	utils.WriteJSON(w, http.StatusOK, projects)
}

// GetProject returns a project by id
func (s *Server) GetProject(w http.ResponseWriter, r *http.Request) {
	id, err := utils.RequirePathID(r, "id", "project id")
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var p models.Project
	if err := s.DB.NewSelect().Model(&p).Where("id = ?", id).Scan(r.Context()); err != nil {
		if utils.IsNotFound(err) {
			utils.WriteError(w, http.StatusNotFound, "project not found")
			return
		}
		utils.WriteError(w, http.StatusInternalServerError, "failed to query project: "+err.Error())
		return
	}

	utils.WriteJSON(w, http.StatusOK, p)
}

// CreateProject adds a new project
func (s *Server) CreateProject(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := utils.DecodeJSON(r, &req); err != nil || strings.TrimSpace(req.Name) == "" {
		utils.WriteError(w, http.StatusBadRequest, "project name is required")
		return
	}

	sortOrder, err := utils.NextSortOrder(r.Context(), s.DB.DB, (*models.Project)(nil), "")
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "failed to create project")
		return
	}

	id := uuid.New().String()
	name := strings.TrimSpace(req.Name)
	p := models.Project{
		ID:          id,
		Name:        name,
		Description: strings.TrimSpace(req.Description),
		SortOrder:   sortOrder,
	}

	if _, err := s.DB.NewInsert().Model(&p).Exec(r.Context()); err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "failed to create project")
		return
	}

	utils.WriteJSON(w, http.StatusCreated, map[string]string{"id": id, "name": name})
}

// UpdateProject edits project details or archives/unarchives it
func (s *Server) UpdateProject(w http.ResponseWriter, r *http.Request) {
	id, err := utils.RequirePathID(r, "id", "project id")
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var p models.Project
	if err := s.DB.NewSelect().Model(&p).Where("id = ?", id).Scan(r.Context()); err != nil {
		if utils.IsNotFound(err) {
			utils.WriteError(w, http.StatusNotFound, "project not found")
			return
		}
		utils.WriteError(w, http.StatusInternalServerError, "failed to query project: "+err.Error())
		return
	}

	var req struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		IsArchived  *bool   `json:"is_archived"`
	}
	if err := utils.DecodeJSON(r, &req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	changed := false
	if req.Name != nil {
		if trimmed := strings.TrimSpace(*req.Name); trimmed != "" && p.Name != trimmed {
			p.Name = trimmed
			changed = true
		}
	}
	if req.Description != nil {
		if trimmed := strings.TrimSpace(*req.Description); p.Description != trimmed {
			p.Description = trimmed
			changed = true
		}
	}
	if req.IsArchived != nil && p.IsArchived != *req.IsArchived {
		p.IsArchived = *req.IsArchived
		changed = true
	}

	if !changed {
		utils.WriteJSON(w, http.StatusOK, map[string]string{"message": "no changes"})
		return
	}

	p.UpdatedAt = time.Now().UTC()
	res, err := s.DB.NewUpdate().Model(&p).Where("id = ?", id).Exec(r.Context())
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "failed to update project: "+err.Error())
		return
	}
	if !utils.HasRowsAffected(res) {
		utils.WriteError(w, http.StatusNotFound, "project not found")
		return
	}

	utils.WriteJSON(w, http.StatusOK, map[string]string{"message": "project updated successfully"})
}

// DeleteProject deletes a project and cascades deletion to its tasks
func (s *Server) DeleteProject(w http.ResponseWriter, r *http.Request) {
	id, err := utils.RequirePathID(r, "id", "project id")
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	deleted, err := utils.DeleteByID(r.Context(), s.DB.DB, (*models.Project)(nil), id)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "failed to delete project")
		return
	}
	if !deleted {
		utils.WriteError(w, http.StatusNotFound, "project not found")
		return
	}

	utils.WriteJSON(w, http.StatusOK, map[string]string{"message": "project deleted"})
}
