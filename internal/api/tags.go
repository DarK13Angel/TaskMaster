package api

import (
	"net/http"
	"taskmaster/internal/models"
)

// ListStatusTags returns all pipeline status columns ordered by sort_order
func (s *Server) ListStatusTags(w http.ResponseWriter, r *http.Request) {
	tags := make([]models.StatusTag, 0)
	err := s.DB.NewSelect().
		Model(&tags).
		Order("sort_order ASC").
		Scan(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to query status tags")
		return
	}

	writeJSON(w, http.StatusOK, tags)
}

// ListPriorities returns the available priority options
func (s *Server) ListPriorities(w http.ResponseWriter, r *http.Request) {
	priorities := []models.Priority{
		models.PriorityLow,
		models.PriorityMedium,
		models.PriorityHigh,
		models.PriorityUrgent,
	}
	writeJSON(w, http.StatusOK, priorities)
}
